//go:build windows

package win32

// Live Win32 glue: the lazy DLL + proc bindings the fleet was hand-rolling in
// more than one place, thin typed wrappers over the nine shared procedures, the
// standard message pump, a hidden message-only window helper, and the top-down
// 32-bpp BGRA StretchDIBits blit. Everything reaches the Windows API through
// golang.org/x/sys/windows (no cgo), so it links with CGO_ENABLED=0.

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Shared system DLLs, resolved lazily on first use. Exposed so a consumer can
// bind a proc win32 does not yet wrap (e.g. Shell_NotifyIconW, BeginPaint) off
// the same handle rather than re-declaring the DLL.
var (
	User32   = windows.NewLazySystemDLL(User32DLL)
	Gdi32    = windows.NewLazySystemDLL(Gdi32DLL)
	Kernel32 = windows.NewLazySystemDLL(Kernel32DLL)
	Advapi32 = windows.NewLazySystemDLL(Advapi32DLL)
	Shell32  = windows.NewLazySystemDLL(Shell32DLL)
	Combase  = windows.NewLazySystemDLL(CombaseDLL)
)

// The nine procedures both go-widgets/tray and go-widgets/window hand-rolled,
// plus PostMessageW and DestroyWindow that both also need.
var (
	procRegisterClassExW = User32.NewProc("RegisterClassExW")
	procCreateWindowExW  = User32.NewProc("CreateWindowExW")
	procDefWindowProcW   = User32.NewProc("DefWindowProcW")
	procGetMessageW      = User32.NewProc("GetMessageW")
	procTranslateMessage = User32.NewProc("TranslateMessage")
	procDispatchMessageW = User32.NewProc("DispatchMessageW")
	procPostQuitMessage  = User32.NewProc("PostQuitMessage")
	procPostMessageW     = User32.NewProc("PostMessageW")
	procLoadCursorW      = User32.NewProc("LoadCursorW")
	procDestroyWindow    = User32.NewProc("DestroyWindow")

	procGetModuleHandleW = Kernel32.NewProc("GetModuleHandleW")

	procStretchDIBits = Gdi32.NewProc("StretchDIBits")
)

// NewCallback wraps a window procedure so it can be stored in
// WndClassExW.LpfnWndProc. It is a thin passthrough to windows.NewCallback,
// which allocates a non-collectable trampoline — so create the callback ONCE
// per window class, never per message. fn must be
// func(hwnd HWND, msg uint32, wParam WPARAM, lParam LPARAM) LRESULT (or an
// all-uintptr-width equivalent windows.NewCallback accepts).
func NewCallback(fn any) uintptr { return windows.NewCallback(fn) }

// UTF16PtrFromString is a convenience passthrough so consumers reach one import
// for the whole backend. It returns a pointer to a NUL-terminated UTF-16
// encoding of s, or an error if s contains a NUL.
func UTF16PtrFromString(s string) (*uint16, error) { return windows.UTF16PtrFromString(s) }

// RegisterClassEx registers a window class and returns its atom, or a wrapped
// error when the atom is zero.
func RegisterClassEx(wc *WndClassExW) (ATOM, error) {
	r, _, e := procRegisterClassExW.Call(uintptr(unsafe.Pointer(wc)))
	if r == 0 {
		return 0, fmt.Errorf("win32: RegisterClassExW: %w", e)
	}
	return ATOM(r), nil
}

// CreateWindowEx creates a window and returns its handle, or a wrapped error
// when the handle is zero. Pass [HWNDMessage] as parent for a message-only
// window.
func CreateWindowEx(exStyle uint32, className, windowName *uint16, style uint32, x, y, w, h int32, parent HWND, menu HMENU, inst HINSTANCE, param unsafe.Pointer) (HWND, error) {
	r, _, e := procCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), uintptr(menu), uintptr(inst), uintptr(param),
	)
	if r == 0 {
		return 0, fmt.Errorf("win32: CreateWindowExW: %w", e)
	}
	return HWND(r), nil
}

// DefWindowProc invokes the default window procedure for messages a consumer's
// WNDPROC does not handle.
func DefWindowProc(hwnd HWND, msg uint32, wParam WPARAM, lParam LPARAM) LRESULT {
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), uintptr(wParam), uintptr(lParam))
	return LRESULT(r)
}

// GetMessage retrieves the next message for the calling thread, blocking until
// one arrives. It returns >0 for an ordinary message, 0 on WM_QUIT and -1 on
// error — the raw GetMessageW contract, so callers can distinguish them.
func GetMessage(m *Msg, hwnd HWND, filterMin, filterMax uint32) int32 {
	r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(m)), uintptr(hwnd), uintptr(filterMin), uintptr(filterMax))
	return int32(r)
}

// TranslateMessage translates virtual-key messages into character messages.
func TranslateMessage(m *Msg) bool {
	r, _, _ := procTranslateMessage.Call(uintptr(unsafe.Pointer(m)))
	return r != 0
}

// DispatchMessage dispatches a message to its window procedure.
func DispatchMessage(m *Msg) LRESULT {
	r, _, _ := procDispatchMessageW.Call(uintptr(unsafe.Pointer(m)))
	return LRESULT(r)
}

// PostQuitMessage posts WM_QUIT, which ends the [Pump] loop.
func PostQuitMessage(exitCode int32) { procPostQuitMessage.Call(uintptr(exitCode)) }

// PostMessage posts a message to a window's queue without waiting.
func PostMessage(hwnd HWND, msg uint32, wParam WPARAM, lParam LPARAM) bool {
	r, _, _ := procPostMessageW.Call(uintptr(hwnd), uintptr(msg), uintptr(wParam), uintptr(lParam))
	return r != 0
}

// GetModuleHandle returns the module handle for name, or the calling process'
// own handle when name is nil.
func GetModuleHandle(name *uint16) HINSTANCE {
	r, _, _ := procGetModuleHandleW.Call(uintptr(unsafe.Pointer(name)))
	return HINSTANCE(r)
}

// LoadCursor loads a cursor. For a standard cursor pass a nil instance and a
// resource id such as [IDCArrow].
func LoadCursor(inst HINSTANCE, name uintptr) HCURSOR {
	r, _, _ := procLoadCursorW.Call(uintptr(inst), name)
	return HCURSOR(r)
}

// DestroyWindow destroys a window created with [CreateWindowEx].
func DestroyWindow(hwnd HWND) bool {
	r, _, _ := procDestroyWindow.Call(uintptr(hwnd))
	return r != 0
}

// Pump runs the standard GetMessage/TranslateMessage/DispatchMessage loop on
// the calling thread until WM_QUIT (GetMessage returns 0), then returns nil. It
// returns a non-nil error only if GetMessageW itself fails (-1). Pin the
// goroutine with runtime.LockOSThread before calling, so the window procedure
// and the pump run on the thread that owns the window.
func Pump() error {
	var m Msg
	for {
		r := GetMessage(&m, 0, 0, 0)
		if r == -1 {
			return fmt.Errorf("win32: GetMessageW failed")
		}
		if r == 0 { // WM_QUIT
			return nil
		}
		TranslateMessage(&m)
		DispatchMessage(&m)
	}
}

// StretchDIBitsBGRA blits a pre-packed, top-down 32-bpp BGRA buffer (see
// [PackBGRA]) into the device context, stretching the srcW×srcH source over the
// dstW×dstH destination rectangle with SRCCOPY. The negative BiHeight requests a
// top-down DIB so row 0 is the top row. It returns StretchDIBits' scanline
// count. A zero-sized rectangle or an empty buffer is a no-op returning 0.
func StretchDIBitsBGRA(hdc HDC, dstX, dstY, dstW, dstH, srcW, srcH int32, bgra []byte) int32 {
	if len(bgra) == 0 || srcW <= 0 || srcH <= 0 || dstW <= 0 || dstH <= 0 {
		return 0
	}
	bmi := BitmapInfoHeader{
		BiSize:        uint32(unsafe.Sizeof(BitmapInfoHeader{})),
		BiWidth:       srcW,
		BiHeight:      -srcH, // negative → top-down (row 0 at top)
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: BIRGB,
	}
	r, _, _ := procStretchDIBits.Call(
		uintptr(hdc),
		uintptr(dstX), uintptr(dstY), uintptr(dstW), uintptr(dstH),
		0, 0, uintptr(srcW), uintptr(srcH),
		uintptr(unsafe.Pointer(&bgra[0])),
		uintptr(unsafe.Pointer(&bmi)),
		uintptr(DIBRGBColors), uintptr(SRCCOPY),
	)
	return int32(r)
}
