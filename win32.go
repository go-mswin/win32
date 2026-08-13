package win32

import "errors"

// ErrUnsupported is reported by the Windows-only entry points when a
// prerequisite is missing. It is stable and may be tested with errors.Is.
var ErrUnsupported = errors.New("win32: unsupported on this platform (windows only)")

// System DLL names bound by the //go:build windows glue. Exposed so consumers
// that need a proc win32 does not yet wrap can bind it off the same lazy DLL
// handle instead of declaring their own.
const (
	User32DLL   = "user32.dll"
	Gdi32DLL    = "gdi32.dll"
	Kernel32DLL = "kernel32.dll"
	Advapi32DLL = "advapi32.dll"
	Shell32DLL  = "shell32.dll"
	CombaseDLL  = "combase.dll"
)
