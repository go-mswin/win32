//go:build windows

package win32

// Live display enumeration: which monitors are attached, where they sit, how
// much of each is left for windows, what they are called and at what DPI they
// are running.
//
// This is the half of the Windows API that answers "which screens are there",
// and it was the one thing a consumer could not get from this package: both
// go-mswin/screencapture and the Win32 back-end of go-widgets/window had to
// bind EnumDisplayMonitors and GetMonitorInfoW off their own handles to find
// out. It is one API, so it is bound once, here.
//
// Everything reaches the OS through golang.org/x/sys/windows, so it links with
// CGO_ENABLED=0.

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Shcore carries the per-monitor DPI query. It is NOT one of the windowing
// DLLs — it is the shell scaling library, and it only exists from Windows 8.1
// — so [GetDpiForMonitor] checks that it resolves rather than assuming it.
var Shcore = windows.NewLazySystemDLL(ShcoreDLL)

var (
	procEnumDisplayMonitors   = User32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW       = User32.NewProc("GetMonitorInfoW")
	procEnumDisplayDevicesW   = User32.NewProc("EnumDisplayDevicesW")
	procMonitorFromWindow     = User32.NewProc("MonitorFromWindow")
	procSetProcessDPIAwareCtx = User32.NewProc("SetProcessDpiAwarenessContext")

	procGetDpiForMonitor = Shcore.NewProc("GetDpiForMonitor")

	procRtlMoveMemory = Kernel32.NewProc("RtlMoveMemory")
)

// enum serialises the monitor enumeration and holds the callback currently
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
	fn func(HMONITOR, HDC, Rect) bool
	// stopped records that fn asked to end the walk, which the OS reports the
	// same way as a genuine failure.
	stopped bool
}

// enumTrampoline is the single MONITORENUMPROC. sync.OnceValue defers the
// allocation to the first enumeration, so a process that never enumerates
// displays never pays for it.
var enumTrampoline = sync.OnceValue(func() uintptr {
	return windows.NewCallback(func(mon, dc, lprc, _ uintptr) uintptr {
		if !enum.fn(HMONITOR(mon), HDC(dc), readRect(lprc)) {
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
	enum.fn, enum.stopped = fn, false
	defer func() { enum.fn = nil }()

	r, _, _ := procEnumDisplayMonitors.Call(0, 0, enumTrampoline(), 0)
	if r == 0 && !enum.stopped {
		// EnumDisplayMonitors returns FALSE both when the callback stopped it
		// and when it genuinely failed, and does not distinguish them. A
		// callback that asked to stop got what it asked for, so only a
		// callback that did not is a failure worth reporting.
		return lastErr("EnumDisplayMonitors")
	}
	return nil
}

// GetMonitorInfo describes one monitor: its bounds, its work area, whether it
// is the primary one and what GDI calls it.
//
// The structure's cbSize is set here rather than by the caller. It is how the
// OS tells MONITORINFOEXW from MONITORINFO, and a wrong one is SILENTLY
// REJECTED — the call returns FALSE and sets no error that names the cause.
func GetMonitorInfo(mon HMONITOR) (MonitorInfoEx, error) {
	mi := NewMonitorInfoEx()
	r, _, _ := procGetMonitorInfoW.Call(uintptr(mon), uintptr(unsafe.Pointer(&mi)))
	if r == 0 {
		return MonitorInfoEx{}, lastErr("GetMonitorInfoW")
	}
	return mi, nil
}

// EnumDisplayDevices reads one entry of the display device tree: the adapter
// at index when device is nil, or that adapter's monitor at index when device
// is its name (`\\.\DISPLAY1`, from [UTF16PtrFromString]).
//
// ok is false when index is past the end, which is how the enumeration ends
// and is NOT an error. flags is [EDDGetDeviceInterfaceName] or 0.
//
// [DisplayAdapters] and [DisplayMonitors] wrap the two walks a caller
// actually wants; this is here for anything that needs a single entry.
func EnumDisplayDevices(device *uint16, index uint32, flags uint32) (DisplayDevice, bool) {
	dd := NewDisplayDevice()
	r, _, _ := procEnumDisplayDevicesW.Call(
		uintptr(unsafe.Pointer(device)),
		uintptr(index),
		uintptr(unsafe.Pointer(&dd)),
		uintptr(flags),
	)
	if r == 0 {
		return DisplayDevice{}, false
	}
	return dd, true
}

// DisplayAdapters lists the display ADAPTERS: `\\.\DISPLAY1`, `\\.\DISPLAY2`,
// … with the graphics device's name in Description.
//
// Every adapter is listed, attached to the desktop or not — a laptop's
// disabled internal panel and every mirroring pseudo-device included. Filter
// with [DisplayDevice.Active] and [DisplayDevice.Mirroring].
func DisplayAdapters(flags uint32) []DisplayDevice {
	var out []DisplayDevice
	for i := uint32(0); ; i++ {
		dd, ok := EnumDisplayDevices(nil, i, flags)
		if !ok {
			return out
		}
		out = append(out, dd)
	}
}

// DisplayMonitors lists the monitors attached to one adapter, named as
// `\\.\DISPLAY1\Monitor0`.
//
// In practice there is exactly one — an adapter with two panels on it is two
// adapters as far as GDI is concerned — but the API is a list and so is this.
// An adapter name that cannot be converted to UTF-16 (it contains a NUL) has
// no monitors, which is the same answer as a name that names nothing.
func DisplayMonitors(adapter string, flags uint32) []DisplayDevice {
	name, err := windows.UTF16PtrFromString(adapter)
	if err != nil {
		return nil
	}
	var out []DisplayDevice
	for i := uint32(0); ; i++ {
		dd, ok := EnumDisplayDevices(name, i, flags)
		if !ok {
			return out
		}
		out = append(out, dd)
	}
}

// GetDpiForMonitor reports a monitor's DPI. dpiType is [MDTEffectiveDPI] for
// the scale factor the user chose, which is what a layout must use.
//
// The horizontal and vertical values are separate because the API says so;
// every display Windows has ever reported gives the same number twice.
//
// It fails on a system with no shcore.dll (before Windows 8.1) and on a
// monitor handle that is no longer valid. A caller that wants a number
// regardless should fall back to [DefaultDPI], which is the unscaled 100%.
func GetDpiForMonitor(mon HMONITOR, dpiType uint32) (x, y uint32, err error) {
	if e := procGetDpiForMonitor.Find(); e != nil {
		return 0, 0, fmt.Errorf("win32: GetDpiForMonitor: %w", e)
	}
	r, _, _ := procGetDpiForMonitor.Call(uintptr(mon), uintptr(dpiType),
		uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)))
	// GetDpiForMonitor returns an HRESULT, where any negative value is a
	// failure and S_OK is zero; it does not set the thread's last error, so
	// there is nothing for lastErr to read.
	if int32(r) < 0 {
		return 0, 0, fmt.Errorf("win32: GetDpiForMonitor: HRESULT 0x%08X", uint32(r))
	}
	if x == 0 || y == 0 {
		return 0, 0, fmt.Errorf("win32: GetDpiForMonitor reported %dx%d dpi", x, y)
	}
	return x, y, nil
}

// MonitorFromWindow returns the monitor a window is on — the one it overlaps
// most, when it straddles two. fallback is one of [MonitorDefaultToNull],
// [MonitorDefaultToPrimary] or [MonitorDefaultToNearest], and decides what a
// window that is on no monitor at all reports.
func MonitorFromWindow(hwnd HWND, fallback uint32) HMONITOR {
	r, _, _ := procMonitorFromWindow.Call(uintptr(hwnd), uintptr(fallback))
	return HMONITOR(r)
}

// SetProcessDPIAwarenessContext declares how this process wants to see DPI,
// and reports whether the declaration took.
//
// Pass [DPIAwarenessPerMonitorV2] before reading any monitor geometry.
// Without it the OS hands back VIRTUALISED coordinates — a 3840×2160 panel at
// 200% reports itself as 1920×1080 — which is plausible, wrong, and impossible
// to detect from the numbers alone.
//
// It fails when awareness has ALREADY been set, either by an earlier call or
// by the application manifest, which is not something to treat as an error:
// the process is aware, it is simply not this call that made it so.
func SetProcessDPIAwarenessContext(ctx uintptr) bool {
	r, _, _ := procSetProcessDPIAwareCtx.Call(ctx)
	return r != 0
}
