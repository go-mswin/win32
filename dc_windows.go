//go:build windows

package win32

// Device contexts, GDI objects and the window queries that go with them.
//
// win32 already shipped [StretchDIBitsBGRA], which takes an HDC, and no way
// whatsoever to OBTAIN one — every consumer had to bind GetDC off its own
// LazyDLL to use the blit this package provides. These are the procedures that
// close that gap, plus the window-state queries that anyone walking the desktop
// needs: they were hand-rolled in go-mswin/screencapture and in
// go-widgets/window before landing here.
//
// Everything reaches the OS through golang.org/x/sys/windows, so it links with
// CGO_ENABLED=0.

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procGetDC                = User32.NewProc("GetDC")
	procGetWindowDC          = User32.NewProc("GetWindowDC")
	procReleaseDC            = User32.NewProc("ReleaseDC")
	procShowWindow           = User32.NewProc("ShowWindow")
	procUpdateWindow         = User32.NewProc("UpdateWindow")
	procSetWindowPos         = User32.NewProc("SetWindowPos")
	procSetForegroundWindow  = User32.NewProc("SetForegroundWindow")
	procGetForegroundWindow  = User32.NewProc("GetForegroundWindow")
	procGetWindowRect        = User32.NewProc("GetWindowRect")
	procGetClientRect        = User32.NewProc("GetClientRect")
	procIsWindow             = User32.NewProc("IsWindow")
	procIsWindowVisible      = User32.NewProc("IsWindowVisible")
	procIsIconic             = User32.NewProc("IsIconic")
	procGetWindowTextW       = User32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = User32.NewProc("GetWindowTextLengthW")
	procGetClassNameW        = User32.NewProc("GetClassNameW")
	procGetWindowLongPtrW    = User32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW    = User32.NewProc("SetWindowLongPtrW")
	procInvalidateRect       = User32.NewProc("InvalidateRect")

	procCreateCompatibleDC = Gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = Gdi32.NewProc("DeleteDC")
	procSelectObject       = Gdi32.NewProc("SelectObject")
	procDeleteObject       = Gdi32.NewProc("DeleteObject")
	procBitBlt             = Gdi32.NewProc("BitBlt")
	procStretchBlt         = Gdi32.NewProc("StretchBlt")
	procSetStretchBltMode  = Gdi32.NewProc("SetStretchBltMode")
	procPatBlt             = Gdi32.NewProc("PatBlt")
	procGetDeviceCaps      = Gdi32.NewProc("GetDeviceCaps")
)

// lastErr wraps the calling thread's last Win32 error for op. It is only
// meaningful immediately after a call that documented a failure, so every call
// site below tests the return value FIRST.
func lastErr(op string) error {
	if e := windows.GetLastError(); e != nil {
		return fmt.Errorf("win32: %s: %w", op, e)
	}
	return fmt.Errorf("win32: %s failed with no error code", op)
}

// GetDC returns a device context for a window's CLIENT area, or for the whole
// virtual screen when hwnd is 0.
//
// A screen device context's coordinate origin is the top-left of the PRIMARY
// monitor, so a monitor placed above or to the left of it is at NEGATIVE
// coordinates. That is correct and must not be clamped.
//
// The result must go back through [ReleaseDC], never [DeleteDC]: they come
// from a small per-session pool, and leaking them eventually fails every GDI
// call in the session.
func GetDC(hwnd HWND) (HDC, error) {
	r, _, _ := procGetDC.Call(uintptr(hwnd))
	if r == 0 {
		return 0, lastErr("GetDC")
	}
	return HDC(r), nil
}

// GetWindowDC returns a device context for a window INCLUDING its frame and
// title bar, where [GetDC] gives only the client area.
func GetWindowDC(hwnd HWND) (HDC, error) {
	r, _, _ := procGetWindowDC.Call(uintptr(hwnd))
	if r == 0 {
		return 0, lastErr("GetWindowDC")
	}
	return HDC(r), nil
}

// ReleaseDC gives back a device context obtained from [GetDC] or
// [GetWindowDC]. hwnd must be the same one it was obtained for.
func ReleaseDC(hwnd HWND, dc HDC) bool {
	r, _, _ := procReleaseDC.Call(uintptr(hwnd), uintptr(dc))
	return r != 0
}

// CreateCompatibleDC creates a memory device context with the same pixel
// format as ref, which is what a bitmap is selected into before anything is
// drawn on it. A zero ref means the current screen.
//
// The result is destroyed with [DeleteDC], NOT [ReleaseDC].
func CreateCompatibleDC(ref HDC) (HDC, error) {
	r, _, _ := procCreateCompatibleDC.Call(uintptr(ref))
	if r == 0 {
		return 0, lastErr("CreateCompatibleDC")
	}
	return HDC(r), nil
}

// DeleteDC destroys a memory device context created by
// [CreateCompatibleDC].
func DeleteDC(dc HDC) bool {
	r, _, _ := procDeleteDC.Call(uintptr(dc))
	return r != 0
}

// SelectObject selects a GDI object — a bitmap, pen, brush or font — into a
// device context and returns the object it replaced.
//
// The previous object MUST be selected back before either it or the device
// context is destroyed: deleting a bitmap that is still selected leaks it, and
// the leak is invisible until the process runs out of GDI handles.
func SelectObject(dc HDC, obj HANDLE) HANDLE {
	r, _, _ := procSelectObject.Call(uintptr(dc), uintptr(obj))
	return HANDLE(r)
}

// DeleteObject destroys a GDI object.
func DeleteObject(obj HANDLE) bool {
	r, _, _ := procDeleteObject.Call(uintptr(obj))
	return r != 0
}

// BitBlt copies a rectangle of pixels from one device context to another.
//
// rop is a raster operation: [SRCCOPY] for a plain copy, and
// [SRCCOPY]|[CaptureBLT] when the source is the screen and layered windows
// must be included.
func BitBlt(dst HDC, dstX, dstY, w, h int32, src HDC, srcX, srcY int32, rop uint32) error {
	r, _, _ := procBitBlt.Call(
		uintptr(dst), uintptr(dstX), uintptr(dstY), uintptr(w), uintptr(h),
		uintptr(src), uintptr(srcX), uintptr(srcY), uintptr(rop))
	if r == 0 {
		return lastErr("BitBlt")
	}
	return nil
}

// StretchBlt copies a rectangle between device contexts, resampling it to a
// different size. Set [Halftone] with [SetStretchBltMode] first for anything
// being made smaller.
func StretchBlt(dst HDC, dstX, dstY, dstW, dstH int32,
	src HDC, srcX, srcY, srcW, srcH int32, rop uint32) error {
	r, _, _ := procStretchBlt.Call(
		uintptr(dst), uintptr(dstX), uintptr(dstY), uintptr(dstW), uintptr(dstH),
		uintptr(src), uintptr(srcX), uintptr(srcY), uintptr(srcW), uintptr(srcH),
		uintptr(rop))
	if r == 0 {
		return lastErr("StretchBlt")
	}
	return nil
}

// SetStretchBltMode sets a device context's resampling mode and returns the
// previous one. Only [Halftone] averages pixels rather than dropping them.
func SetStretchBltMode(dc HDC, mode int32) int32 {
	r, _, _ := procSetStretchBltMode.Call(uintptr(dc), uintptr(mode))
	return int32(r)
}

// PatBlt fills a rectangle of a device context using a raster operation with
// no source — [Blackness] and [Whiteness] being the useful ones. It is the
// cheapest way to clear a device context that is about to be composited into.
func PatBlt(dc HDC, x, y, w, h int32, rop uint32) error {
	r, _, _ := procPatBlt.Call(uintptr(dc), uintptr(x), uintptr(y),
		uintptr(w), uintptr(h), uintptr(rop))
	if r == 0 {
		return lastErr("PatBlt")
	}
	return nil
}

// GetDeviceCaps reports one of a device context's capability indices, e.g.
// HORZRES or BITSPIXEL.
func GetDeviceCaps(dc HDC, index int32) int32 {
	r, _, _ := procGetDeviceCaps.Call(uintptr(dc), uintptr(index))
	return int32(r)
}

// ShowWindow sets a window's show state, e.g. [SWShow] or [SWHide], and
// reports whether it was previously visible.
func ShowWindow(hwnd HWND, cmd int32) bool {
	r, _, _ := procShowWindow.Call(uintptr(hwnd), uintptr(cmd))
	return r != 0
}

// UpdateWindow sends WM_PAINT immediately if the window has an invalid
// region, rather than waiting for the message queue to drain.
func UpdateWindow(hwnd HWND) bool {
	r, _, _ := procUpdateWindow.Call(uintptr(hwnd))
	return r != 0
}

// InvalidateRect marks the whole window (or, with a rectangle, part of it) as
// needing repainting. A nil rect invalidates everything.
func InvalidateRect(hwnd HWND, r *Rect, erase bool) bool {
	var p uintptr
	if r != nil {
		p = uintptr(unsafe.Pointer(r))
	}
	var e uintptr
	if erase {
		e = 1
	}
	res, _, _ := procInvalidateRect.Call(uintptr(hwnd), p, e)
	return res != 0
}

// SetWindowPos moves, resizes and restacks a window. insertAfter is one of
// [HWNDTop], [HWNDBottom], [HWNDTopmost], [HWNDNoTopmost] or another window's
// handle; flags is a combination of the SWP constants.
func SetWindowPos(hwnd, insertAfter HWND, x, y, w, h int32, flags uint32) error {
	r, _, _ := procSetWindowPos.Call(uintptr(hwnd), uintptr(insertAfter),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(flags))
	if r == 0 {
		return lastErr("SetWindowPos")
	}
	return nil
}

// SetForegroundWindow brings a window to the foreground and gives it the
// keyboard focus. The OS refuses it in several documented situations — a
// process that has not had recent user input, most of all — so the boolean
// matters.
func SetForegroundWindow(hwnd HWND) bool {
	r, _, _ := procSetForegroundWindow.Call(uintptr(hwnd))
	return r != 0
}

// GetForegroundWindow returns the window the user is currently working with,
// or 0 when the foreground belongs to another desktop.
func GetForegroundWindow() HWND {
	r, _, _ := procGetForegroundWindow.Call()
	return HWND(r)
}

// GetWindowRect returns a window's bounding rectangle in SCREEN coordinates,
// frame included.
//
// On a composited desktop this rectangle is LARGER than what the user sees:
// it includes the invisible resize border DWM draws outside the frame, up to
// eight pixels a side on a default theme. DWMWA_EXTENDED_FRAME_BOUNDS is the
// corrected rectangle, and dwmapi is not part of this package.
func GetWindowRect(hwnd HWND) (Rect, error) {
	var r Rect
	res, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	if res == 0 {
		return Rect{}, lastErr("GetWindowRect")
	}
	return r, nil
}

// GetClientRect returns a window's client area, in CLIENT coordinates: Left
// and Top are always 0, so it is really a size.
func GetClientRect(hwnd HWND) (Rect, error) {
	var r Rect
	res, _, _ := procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	if res == 0 {
		return Rect{}, lastErr("GetClientRect")
	}
	return r, nil
}

// IsWindow reports whether a handle still names a window. A handle can be
// reused after the window is destroyed, so a true here is not a guarantee that
// it names the SAME window.
func IsWindow(hwnd HWND) bool {
	r, _, _ := procIsWindow.Call(uintptr(hwnd))
	return r != 0
}

// IsWindowVisible reports whether a window has WS_VISIBLE. It says nothing
// about whether the window is on screen, unoccluded, or cloaked by DWM.
func IsWindowVisible(hwnd HWND) bool {
	r, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
	return r != 0
}

// IsIconic reports whether a window is minimised. A minimised window has no
// pixels on screen.
func IsIconic(hwnd HWND) bool {
	r, _, _ := procIsIconic.Call(uintptr(hwnd))
	return r != 0
}

// GetWindowText returns a window's title. It is empty both for a window with
// no title and for one owned by another process that does not answer
// WM_GETTEXT.
func GetWindowText(hwnd HWND) string {
	n, _, _ := procGetWindowTextLengthW.Call(uintptr(hwnd))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, int(n)+1)
	got, _, _ := procGetWindowTextW.Call(uintptr(hwnd),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if got == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:got])
}

// GetClassName returns a window's class name, which survives a title change
// and is often the better way to find a window again. 256 characters is the
// documented maximum a class name can be.
func GetClassName(hwnd HWND) string {
	var buf [256]uint16
	got, _, _ := procGetClassNameW.Call(uintptr(hwnd),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if got == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:got])
}

// GetWindowLongPtr reads one of a window's LONG_PTR fields, e.g.
// [GWLExStyle]. golang.org/x/sys/windows does not wrap it.
//
// The index is NEGATIVE, so it is taken as an int32 and sign-extended rather
// than being passed as a uintptr the caller has to build by hand.
func GetWindowLongPtr(hwnd HWND, index int32) uintptr {
	r, _, _ := procGetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index))
	return r
}

// SetWindowLongPtr writes one of a window's LONG_PTR fields and returns the
// previous value.
func SetWindowLongPtr(hwnd HWND, index int32, value uintptr) uintptr {
	r, _, _ := procSetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index), value)
	return r
}
