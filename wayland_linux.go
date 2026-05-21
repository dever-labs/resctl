//go:build linux

package main

// wlrClient implements Wayland display output management using the
// zwlr-output-management-unstable-v1 protocol over the Wayland Unix socket.
//
// This requires a wlroots-based compositor (Sway, Hyprland, etc.).
// No external binary or Go module dependency is required.

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// --- internal data types ---

type wlrModeInfo struct {
	id        uint32
	width     int32
	height    int32
	refreshHz uint32 // converted from mHz
	preferred bool
	finished  bool
}

type wlrHeadInfo struct {
	id          uint32
	name        string
	enabled     bool
	modes       []*wlrModeInfo
	currentMode uint32 // ID of the active zwlr_output_mode_v1
	finished    bool
	posX        int32
	posY        int32
	transform   uint32
	scale       int32 // wl_fixed 24.8; 0 means not yet received (defaults to 256 = 1.0)
}

type wlrClient struct {
	conn   net.Conn
	nextID uint32 // client-side object IDs start at 3 (1=wl_display, 2=wl_registry)

	managerID     uint32
	managerSerial uint32

	heads     map[uint32]*wlrHeadInfo
	modes     map[uint32]*wlrModeInfo
	headOrder []uint32 // insertion order

	syncCallbackID   uint32
	syncCallbackDone bool

	configResultID uint32
	configResult   int // 0=pending, 1=succeeded, 2=failed/cancelled
}

// compositorHint returns a human-readable explanation and suggested alternative
// when zwlr_output_manager_v1 is unavailable, based on the desktop environment.
func compositorHint() string {
	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP"))
	if desktop == "" {
		desktop = strings.ToLower(os.Getenv("DESKTOP_SESSION"))
	}
	switch {
	case strings.Contains(desktop, "gnome"):
		return "GNOME does not support wlr-output-management; use 'gnome-randr' or Settings → Displays"
	case strings.Contains(desktop, "kde") || strings.Contains(desktop, "plasma"):
		return "KDE Plasma does not support wlr-output-management; use 'kscreen-doctor' or System Settings → Display"
	default:
		return "compositor does not support wlr-output-management (requires a wlroots-based compositor such as Sway or Hyprland)"
	}
}

// --- connection ---

func wlrDial() (*wlrClient, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return nil, fmt.Errorf("XDG_RUNTIME_DIR is not set")
	}
	disp := os.Getenv("WAYLAND_DISPLAY")
	if disp == "" {
		disp = "wayland-0"
	}
	conn, err := net.Dial("unix", filepath.Join(dir, disp))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Wayland socket: %w", err)
	}
	return &wlrClient{
		conn:   conn,
		nextID: 3,
		heads:  make(map[uint32]*wlrHeadInfo),
		modes:  make(map[uint32]*wlrModeInfo),
	}, nil
}

func (c *wlrClient) close() { _ = c.conn.Close() }

func (c *wlrClient) allocID() uint32 {
	id := c.nextID
	c.nextID++
	return id
}

// --- wire encoding helpers ---

// encodeString encodes a Wayland string: uint32 length (incl. null), content, 4-byte padding.
func encodeString(s string) []byte {
	s += "\x00"
	padded := (len(s) + 3) &^ 3
	buf := make([]byte, 4+padded)
	binary.LittleEndian.PutUint32(buf, uint32(len(s)))
	copy(buf[4:], s)
	return buf
}

// decodeString reads a Wayland string from data, returning the string and remaining bytes.
func decodeString(data []byte) (string, []byte) {
	if len(data) < 4 {
		return "", nil
	}
	length := int(binary.LittleEndian.Uint32(data[:4]))
	if length == 0 {
		return "", data[4:]
	}
	padded := (length + 3) &^ 3
	if len(data) < 4+padded {
		return "", nil
	}
	return string(data[4 : 4+length-1]), data[4+padded:] // strip null terminator
}

// --- message I/O ---

// send writes a Wayland message: [objectID][size<<16|opcode][args...].
func (c *wlrClient) send(objectID uint32, opcode uint16, args []byte) error {
	size := uint16(8 + len(args))
	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint32(hdr[0:4], objectID)
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(size)<<16|uint32(opcode))
	msg := append(hdr, args...)
	_, err := c.conn.Write(msg)
	return err
}

// readMsg reads one Wayland message from the socket.
func (c *wlrClient) readMsg() (objectID, opcode uint32, data []byte, err error) {
	hdr := make([]byte, 8)
	if _, err = io.ReadFull(c.conn, hdr); err != nil {
		return
	}
	objectID = binary.LittleEndian.Uint32(hdr[0:4])
	sizeOpcode := binary.LittleEndian.Uint32(hdr[4:8])
	opcode = sizeOpcode & 0xffff
	if size := sizeOpcode >> 16; size > 8 {
		data = make([]byte, size-8)
		_, err = io.ReadFull(c.conn, data)
	}
	return
}

// --- standard Wayland protocol ---

// sendGetRegistry sends wl_display.get_registry (opcode 1).
func (c *wlrClient) sendGetRegistry() error {
	args := make([]byte, 4)
	binary.LittleEndian.PutUint32(args, 2) // wl_registry is always id=2
	return c.send(1, 1, args)
}

// sendSync sends wl_display.sync (opcode 0) and sets up the callback.
func (c *wlrClient) sendSync() error {
	cbID := c.allocID()
	c.syncCallbackID = cbID
	c.syncCallbackDone = false
	args := make([]byte, 4)
	binary.LittleEndian.PutUint32(args, cbID)
	return c.send(1, 0, args)
}

// roundTrip dispatches events until the next wl_callback.done fires.
func (c *wlrClient) roundTrip() error {
	if err := c.sendSync(); err != nil {
		return err
	}
	return c.runUntil(func() bool { return c.syncCallbackDone })
}

// waylandTimeout is the maximum time to wait for a compositor response.
const waylandTimeout = 10 * time.Second

// runUntil dispatches messages from the socket until cond() returns true.
// A deadline is applied so the call never blocks indefinitely if the compositor
// stops responding.
func (c *wlrClient) runUntil(cond func() bool) error {
	if err := c.conn.SetDeadline(time.Now().Add(waylandTimeout)); err != nil {
		return fmt.Errorf("wayland: could not set socket deadline: %w", err)
	}
	defer func() { _ = c.conn.SetDeadline(time.Time{}) }()
	for !cond() {
		objectID, opcode, data, err := c.readMsg()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return fmt.Errorf("wayland: timed out waiting for compositor response")
			}
			return fmt.Errorf("wayland read: %w", err)
		}
		if err := c.dispatch(objectID, opcode, data); err != nil {
			return err
		}
	}
	return nil
}

// --- event dispatch ---

func (c *wlrClient) dispatch(objectID, opcode uint32, data []byte) error {
	switch objectID {
	case 1: // wl_display
		return c.onDisplayEvent(opcode, data)
	case 2: // wl_registry
		if opcode == 0 { // global
			return c.onRegistryGlobal(data)
		}
	case c.syncCallbackID:
		if opcode == 0 { // wl_callback.done
			c.syncCallbackDone = true
		}
	case c.managerID:
		c.onManagerEvent(opcode, data)
	case c.configResultID:
		c.onConfigEvent(opcode)
	default:
		if head, ok := c.heads[objectID]; ok {
			c.onHeadEvent(head, opcode, data)
		} else if mode, ok := c.modes[objectID]; ok {
			onModeEvent(mode, opcode, data)
		}
	}
	return nil
}

func (c *wlrClient) onDisplayEvent(opcode uint32, data []byte) error {
	if opcode == 0 && len(data) >= 12 { // error
		code := binary.LittleEndian.Uint32(data[4:8])
		msg, _ := decodeString(data[8:])
		return fmt.Errorf("wayland compositor error (code %d): %s", code, msg)
	}
	return nil
}

// onRegistryGlobal handles wl_registry.global events.
// When zwlr_output_manager_v1 is announced, it sends a bind request.
func (c *wlrClient) onRegistryGlobal(data []byte) error {
	if len(data) < 8 {
		return nil
	}
	name := binary.LittleEndian.Uint32(data[0:4])
	iface, rest := decodeString(data[4:])
	if len(rest) < 4 {
		return nil
	}
	version := binary.LittleEndian.Uint32(rest[0:4])

	if iface != "zwlr_output_manager_v1" || c.managerID != 0 {
		return nil
	}

	mgrID := c.allocID()
	c.managerID = mgrID

	if version > 3 {
		version = 3 // use at most v3; v4 adds adaptive sync which we don't need
	}

	// wl_registry.bind (opcode 0): name(uint32) + interface(string) + version(uint32) + id(uint32)
	ifaceEnc := encodeString(iface)
	args := make([]byte, 4+len(ifaceEnc)+4+4)
	l := 0
	binary.LittleEndian.PutUint32(args[l:], name)
	l += 4
	copy(args[l:], ifaceEnc)
	l += len(ifaceEnc)
	binary.LittleEndian.PutUint32(args[l:], version)
	l += 4
	binary.LittleEndian.PutUint32(args[l:], mgrID)

	return c.send(2, 0, args)
}

// onManagerEvent handles zwlr_output_manager_v1 events.
func (c *wlrClient) onManagerEvent(opcode uint32, data []byte) {
	switch opcode {
	case 0: // head(new_id) — server allocates a zwlr_output_head_v1 ID
		if len(data) < 4 {
			return
		}
		id := binary.LittleEndian.Uint32(data[0:4])
		head := &wlrHeadInfo{id: id}
		c.heads[id] = head
		c.headOrder = append(c.headOrder, id)
	case 1: // done(serial)
		if len(data) >= 4 {
			c.managerSerial = binary.LittleEndian.Uint32(data[0:4])
		}
	}
}

// onHeadEvent handles zwlr_output_head_v1 events.
func (c *wlrClient) onHeadEvent(head *wlrHeadInfo, opcode uint32, data []byte) {
	switch opcode {
	case 0: // name(string)
		head.name, _ = decodeString(data)
	case 3: // mode(new_id) — server allocates a zwlr_output_mode_v1 ID
		if len(data) >= 4 {
			id := binary.LittleEndian.Uint32(data[0:4])
			mode := &wlrModeInfo{id: id}
			c.modes[id] = mode
			head.modes = append(head.modes, mode)
		}
	case 4: // enabled(int)
		if len(data) >= 4 {
			head.enabled = binary.LittleEndian.Uint32(data[0:4]) != 0
		}
	case 5: // current_mode(object)
		if len(data) >= 4 {
			head.currentMode = binary.LittleEndian.Uint32(data[0:4])
		}
	case 6: // position(x int32, y int32)
		if len(data) >= 8 {
			head.posX = int32(binary.LittleEndian.Uint32(data[0:4]))
			head.posY = int32(binary.LittleEndian.Uint32(data[4:8]))
		}
	case 7: // transform(uint32)
		if len(data) >= 4 {
			head.transform = binary.LittleEndian.Uint32(data[0:4])
		}
	case 8: // scale(wl_fixed 24.8)
		if len(data) >= 4 {
			head.scale = int32(binary.LittleEndian.Uint32(data[0:4]))
		}
	case 9: // finished
		head.finished = true
	}
}

// onModeEvent handles zwlr_output_mode_v1 events.
func onModeEvent(mode *wlrModeInfo, opcode uint32, data []byte) {
	switch opcode {
	case 0: // size(int width, int height)
		if len(data) >= 8 {
			mode.width = int32(binary.LittleEndian.Uint32(data[0:4]))
			mode.height = int32(binary.LittleEndian.Uint32(data[4:8]))
		}
	case 1: // refresh(int mHz)
		if len(data) >= 4 {
			mHz := int32(binary.LittleEndian.Uint32(data[0:4]))
			mode.refreshHz = uint32((int64(mHz) + 500) / 1000) // round to nearest Hz
		}
	case 2: // preferred
		mode.preferred = true
	case 3: // finished
		mode.finished = true
	}
}

// onConfigEvent handles zwlr_output_configuration_v1 events.
func (c *wlrClient) onConfigEvent(opcode uint32) {
	switch opcode {
	case 0: // succeeded
		c.configResult = 1
	case 1, 2: // failed or cancelled
		c.configResult = 2
	}
}

// --- zwlr requests ---

// sendCreateConfiguration sends zwlr_output_manager_v1.create_configuration (opcode 0).
// args: new_id(uint32) + serial(uint32)
func (c *wlrClient) sendCreateConfiguration(configID uint32) error {
	args := make([]byte, 8)
	binary.LittleEndian.PutUint32(args[0:4], configID)
	binary.LittleEndian.PutUint32(args[4:8], c.managerSerial)
	return c.send(c.managerID, 0, args)
}

// sendEnableHead sends zwlr_output_configuration_v1.enable_head (opcode 0).
// args: new_id<config_head>(uint32) + head_object(uint32)
func (c *wlrClient) sendEnableHead(configID, configHeadID, headID uint32) error {
	args := make([]byte, 8)
	binary.LittleEndian.PutUint32(args[0:4], configHeadID)
	binary.LittleEndian.PutUint32(args[4:8], headID)
	return c.send(configID, 0, args)
}

// sendSetMode sends zwlr_output_configuration_head_v1.set_mode (opcode 0).
// args: mode_object(uint32)
func (c *wlrClient) sendSetMode(configHeadID, modeID uint32) error {
	args := make([]byte, 4)
	binary.LittleEndian.PutUint32(args[0:4], modeID)
	return c.send(configHeadID, 0, args)
}

// sendSetPosition sends zwlr_output_configuration_head_v1.set_position (opcode 2).
// args: x(int32) + y(int32)
func (c *wlrClient) sendSetPosition(configHeadID uint32, x, y int32) error {
	args := make([]byte, 8)
	binary.LittleEndian.PutUint32(args[0:4], uint32(x))
	binary.LittleEndian.PutUint32(args[4:8], uint32(y))
	return c.send(configHeadID, 2, args)
}

// sendSetTransform sends zwlr_output_configuration_head_v1.set_transform (opcode 3).
// args: transform(uint32)
func (c *wlrClient) sendSetTransform(configHeadID, transform uint32) error {
	args := make([]byte, 4)
	binary.LittleEndian.PutUint32(args[0:4], transform)
	return c.send(configHeadID, 3, args)
}

// sendSetScale sends zwlr_output_configuration_head_v1.set_scale (opcode 4).
// scale is a wl_fixed 24.8 value (e.g. 256 = 1.0, 512 = 2.0).
func (c *wlrClient) sendSetScale(configHeadID uint32, scale int32) error {
	args := make([]byte, 4)
	binary.LittleEndian.PutUint32(args[0:4], uint32(scale))
	return c.send(configHeadID, 4, args)
}

// sendApply sends zwlr_output_configuration_v1.apply (opcode 2).
func (c *wlrClient) sendApply(configID uint32) error {
	return c.send(configID, 2, nil)
}

// --- helpers ---

func (c *wlrClient) firstEnabledHead() *wlrHeadInfo {
	for _, id := range c.headOrder {
		h := c.heads[id]
		if h.enabled && !h.finished {
			return h
		}
	}
	return nil
}

// pickFreqFromHead selects the best refresh rate for width×height from the
// head's already-enumerated modes. Prefers the current rate; falls back to
// the highest available. Returns 0 if no matching mode is found.
func (c *wlrClient) pickFreqFromHead(head *wlrHeadInfo, width, height uint32) uint32 {
	var currentFreq uint32
	if cm, ok := c.modes[head.currentMode]; ok && !cm.finished {
		currentFreq = cm.refreshHz
	}

	var best uint32
	for _, m := range head.modes {
		if m.finished || uint32(m.width) != width || uint32(m.height) != height {
			continue
		}
		if m.refreshHz == currentFreq {
			return currentFreq
		}
		if m.refreshHz > best {
			best = m.refreshHz
		}
	}
	return best
}

func (c *wlrClient) findMode(head *wlrHeadInfo, width, height, freq uint32) *wlrModeInfo {
	var best *wlrModeInfo
	for _, m := range head.modes {
		if m.finished {
			continue
		}
		if uint32(m.width) != width || uint32(m.height) != height {
			continue
		}
		if freq != 0 && m.refreshHz != freq {
			continue
		}
		if best == nil || m.refreshHz > best.refreshHz {
			best = m
		}
	}
	return best
}

// --- public API ---

// wlrNativeQuery connects to the Wayland socket, enumerates outputs and modes,
// and returns the sorted mode list, the current mode, and the output name.
func wlrNativeQuery() (modes []Resolution, current Resolution, outputName string, err error) {
	c, err := wlrDial()
	if err != nil {
		return nil, Resolution{}, "", err
	}
	defer c.close()

	if err = c.sendGetRegistry(); err != nil {
		return nil, Resolution{}, "", err
	}
	// First roundtrip: receive registry globals and bind to the output manager.
	if err = c.roundTrip(); err != nil {
		return nil, Resolution{}, "", err
	}
	if c.managerID == 0 {
		return nil, Resolution{}, "",
			fmt.Errorf("zwlr_output_manager_v1 not available — %s", compositorHint())
	}
	// Second roundtrip: receive all head/mode/done events from the manager.
	if err = c.roundTrip(); err != nil {
		return nil, Resolution{}, "", err
	}

	head := c.firstEnabledHead()
	if head == nil {
		return nil, Resolution{}, "", fmt.Errorf("no enabled display found")
	}

	outputName = head.name
	seen := make(map[string]bool)
	for _, m := range head.modes {
		if m.finished {
			continue
		}
		key := fmt.Sprintf("%dx%d@%d", m.width, m.height, m.refreshHz)
		if seen[key] {
			continue
		}
		seen[key] = true
		res := Resolution{Width: uint32(m.width), Height: uint32(m.height), Freq: m.refreshHz}
		modes = append(modes, res)
		if m.id == head.currentMode {
			current = res
		}
	}
	sort.Slice(modes, func(i, j int) bool {
		a, b := modes[i], modes[j]
		if a.Width != b.Width {
			return a.Width < b.Width
		}
		if a.Height != b.Height {
			return a.Height < b.Height
		}
		return a.Freq < b.Freq
	})
	return modes, current, outputName, nil
}

// wlrNativeSet applies a new display mode via the zwlr-output-management protocol.
func wlrNativeSet(width, height, freq uint32) (Resolution, error) {
	c, err := wlrDial()
	if err != nil {
		return Resolution{}, err
	}
	defer c.close()

	if err = c.sendGetRegistry(); err != nil {
		return Resolution{}, err
	}
	if err = c.roundTrip(); err != nil {
		return Resolution{}, err
	}
	if c.managerID == 0 {
		return Resolution{}, fmt.Errorf("zwlr_output_manager_v1 not available — %s", compositorHint())
	}
	if err = c.roundTrip(); err != nil {
		return Resolution{}, err
	}

	head := c.firstEnabledHead()
	if head == nil {
		return Resolution{}, fmt.Errorf("no enabled display found")
	}

	// When no frequency is specified, pick the best available rate for the
	// requested dimensions using the modes already enumerated over this
	// connection — avoiding a second wlrNativeQuery round-trip.
	if freq == 0 {
		freq = c.pickFreqFromHead(head, width, height)
	}

	target := c.findMode(head, width, height, freq)
	if target == nil {
		if freq != 0 {
			return Resolution{}, fmt.Errorf("resolution %dx%d@%dHz is not supported by this display", width, height, freq)
		}
		return Resolution{}, fmt.Errorf("resolution %dx%d is not supported by this display", width, height)
	}

	configID := c.allocID()
	configHeadID := c.allocID()
	c.configResultID = configID
	c.configResult = 0

	// Default scale to 1.0 (wl_fixed 24.8 = 256) if not received from compositor.
	scale := head.scale
	if scale == 0 {
		scale = 256
	}

	if err = c.sendCreateConfiguration(configID); err != nil {
		return Resolution{}, err
	}
	if err = c.sendEnableHead(configID, configHeadID, head.id); err != nil {
		return Resolution{}, err
	}
	// Preserve all existing head properties; only the mode changes.
	if err = c.sendSetMode(configHeadID, target.id); err != nil {
		return Resolution{}, err
	}
	if err = c.sendSetPosition(configHeadID, head.posX, head.posY); err != nil {
		return Resolution{}, err
	}
	if err = c.sendSetTransform(configHeadID, head.transform); err != nil {
		return Resolution{}, err
	}
	if err = c.sendSetScale(configHeadID, scale); err != nil {
		return Resolution{}, err
	}
	if err = c.sendApply(configID); err != nil {
		return Resolution{}, err
	}

	if err = c.runUntil(func() bool { return c.configResult != 0 }); err != nil {
		return Resolution{}, err
	}
	if c.configResult == 2 {
		return Resolution{}, fmt.Errorf("compositor rejected the display configuration")
	}
	return Resolution{Width: uint32(target.width), Height: uint32(target.height), Freq: target.refreshHz}, nil
}
