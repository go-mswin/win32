// Package win32 is the owned, pure-Go (CGO_ENABLED=0) foundation for calling the
// Windows API from Go — the Windows peer of github.com/go-macos/objc.
//
// It is built on golang.org/x/sys/windows (the Go team's reference, pure-Go
// binding to the Windows syscalls): win32 does NOT reinvent the syscall layer,
// it packages the surface the fleet was hand-rolling in more than one place —
// the lazy DLL + proc bindings for user32/gdi32/kernel32/advapi32/shell32
// (+ combase for WinRT later), a hidden message window and the standard
// GetMessage/TranslateMessage/DispatchMessage pump, the common WNDCLASSEXW / MSG
// / RECT / POINT types, a top-down 32-bpp BGRA StretchDIBits blit helper, and
// the display enumeration (EnumDisplayMonitors / GetMonitorInfoW /
// EnumDisplayDevicesW / GetDpiForMonitor) that says which monitors are
// attached and where they are.
//
// Layout mirrors go-macos/objc: the OS-independent core — the types, the Win32
// constants, the LPARAM word macros and the BGRA packing — lives in UNTAGGED
// files that build and are exercised to 100% statement coverage on every GOOS;
// the live Win32 glue that reaches the DLLs lives in build-tagged
// (//go:build windows) files and is proven on a real Windows machine, with the
// captured artifact committed to the repository. Non-windows GOOS therefore
// build the core and stay green; only the tagged glue is Windows-only.
package win32
