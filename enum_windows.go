//go:build windows

package win32

// The two enumerations the OS drives with a CALLBACK: the monitors on the
// desktop, and the top-level windows on it.
//
// They share this file because they share the thing that makes them awkward.
// windows.NewCallback allocates a trampoline that is NEVER collected, out of a
// pool the Go runtime caps at 2000 for the whole process (runtime.cb_max).
// Exceeding it is not a recoverable panic: it is runtime.throw("too many
// callback functions"), which kills the process outright and cannot be
// recovered from. So a callback built PER CALL is not a slow leak, it is a
// hard ceiling on how many times an application may ask — and a display
// chooser that re-enumerates whenever a monitor is plugged in reaches it.
//
// There is therefore ONE trampoline per enumeration for the whole process,
// created on first use, dispatching to whatever closure the lock currently
// holds. The lock is not an extra precaution either: a process-wide trampoline
// that reads a package variable REQUIRES that two walks cannot be in flight at
// once, or their results interleave into each other.

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procEnumDisplayMonitors = User32.NewProc("EnumDisplayMonitors")
	procEnumWindows         = User32.NewProc("EnumWindows")

	procRtlMoveMemory = Kernel32.NewProc("RtlMoveMemory")
)

// enum serialises the OS-driven enumerations and holds the callback currently
// running.
//
// Two things force this. The Go side of a Win32 callback is a PROCESS-WIDE
// trampoline, so two concurrent enumerations sharing one trampoline would
// interleave into each other's results. And windows.NewCallback allocates a
// trampoline that is NEVER collected, so making one per call leaks a page of
// executable memory per enumeration — a screen chooser that polls would grow
// without bound. One trampoline, created on first use, dispatching to whatever
// closure the lock currently protects, has neither problem.
var enum struct {
	sync.Mutex
	monitor func(HMONITOR, HDC, Rect) bool
	window  func(HWND) bool
	// stopped records that a callback asked to end the walk, which the OS
	// reports the same way as a genuine failure.
	stopped bool
}

// monitorTrampoline is the single MONITORENUMPROC. sync.OnceValue defers the
// allocation to the first enumeration, so a process that never enumerates
// displays never pays for it.
var monitorTrampoline = sync.OnceValue(func() uintptr {
	return windows.NewCallback(func(mon, dc, lprc, _ uintptr) uintptr {
		if !enum.monitor(HMONITOR(mon), HDC(dc), readRect(lprc)) {
			enum.stopped = true
			return 0 // the callback asked to stop
		}
		return 1
	})
})

// readRect copies a RECT out of OS memory at p.
//
// The address arrives as a plain uintptr — it is an argument of a C callback,
// not a Go pointer — and turning it back into a *Rect is exactly the
// conversion go vet's unsafeptr check exists to catch: nothing keeps that
// memory alive across a garbage collection, and the OS may reuse it the moment
// the callback returns. RtlMoveMemory copies it instead, so no uintptr ever
// becomes a pointer on the Go side. A RECT is 16 bytes; this is not a hot
// path.
func readRect(p uintptr) Rect {
	var r Rect
	if p == 0 {
		return r
	}
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&r)), p, unsafe.Sizeof(r))
	return r
}

// EnumDisplayMonitors calls fn once for each monitor on the desktop, in the
// order the OS reports them — which is NOT primary-first and carries no
// meaning; sort by whatever the caller needs.
//
// rect is the monitor's full rectangle on the virtual screen, the same one
// [GetMonitorInfo] returns as RcMonitor. dc is 0, because the enumeration is
// started without a device context; it is passed through so the signature
// matches MONITORENUMPROC and can grow a clipping variant later.
//
// Returning false from fn stops the enumeration early and is not an error.
//
// Calls are serialised against each other: the OS callback is a process-wide
// trampoline, so fn must not itself call EnumDisplayMonitors.
func EnumDisplayMonitors(fn func(mon HMONITOR, dc HDC, rect Rect) bool) error {
	if fn == nil {
		return fmt.Errorf("win32: EnumDisplayMonitors: nil callback")
	}
	enum.Lock()
	defer enum.Unlock()
	enum.monitor, enum.stopped = fn, false
	defer func() { enum.monitor = nil }()

	r, _, _ := procEnumDisplayMonitors.Call(0, 0, monitorTrampoline(), 0)
	if r == 0 && !enum.stopped {
		// EnumDisplayMonitors returns FALSE both when the callback stopped it
		// and when it genuinely failed, and does not distinguish them. A
		// callback that asked to stop got what it asked for, so only a
		// callback that did not is a failure worth reporting.
		return lastErr("EnumDisplayMonitors")
	}
	return nil
}

// windowTrampoline is the single WNDENUMPROC, allocated on first use for the
// same reason monitorTrampoline is.
var windowTrampoline = sync.OnceValue(func() uintptr {
	return windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		if !enum.window(HWND(hwnd)) {
			enum.stopped = true
			return 0 // the callback asked to stop
		}
		return 1
	})
})

// EnumWindows calls fn once for each TOP-LEVEL window on the desktop, in
// z-order: the foreground window first, then downwards.
//
// It is every top-level window, which on an ordinary session is thousands —
// the invisible message-only windows, the shell's own, one per tray icon.
// Anything user-facing is a small filtered subset of them ([IsWindowVisible],
// a non-empty [GetWindowText], no [WSExToolWindow]), and the filtering is the
// caller's because no two callers want the same one.
//
// Returning false from fn stops the walk early and is not an error.
//
// Calls are serialised against each other and against [EnumDisplayMonitors],
// so fn must not itself start either walk.
func EnumWindows(fn func(hwnd HWND) bool) error {
	if fn == nil {
		return fmt.Errorf("win32: EnumWindows: nil callback")
	}
	enum.Lock()
	defer enum.Unlock()
	enum.window, enum.stopped = fn, false
	defer func() { enum.window = nil }()

	r, _, _ := procEnumWindows.Call(windowTrampoline(), 0)
	if r == 0 && !enum.stopped {
		// EnumWindows reports a callback that stopped it and a genuine
		// failure the same way, so only a walk no callback stopped can be
		// called a failure.
		return lastErr("EnumWindows")
	}
	return nil
}
