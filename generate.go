//go:build windows

package main

// This file's go:generate directive produces resource.syso,
// which the Go linker automatically embeds into the Windows binary.
//
//go:generate goversioninfo -64 -o resource.syso
