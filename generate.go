//go:build windows

package main

// go:generate is placed here so `go generate ./...` produces resource.syso,
// which the Go linker automatically embeds into the Windows binary.
//
//go:generate goversioninfo -64 -o resource.syso
