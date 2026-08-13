package win32

// Handle and integer aliases mirroring the Windows API's uintptr-width types.
// They are plain uintptr-based types (not golang.org/x/sys/windows.Handle) so
// this OS-independent core compiles on every GOOS — the x/sys/windows import is
// confined to the //go:build windows glue. Their width and zero-is-nil
// semantics match the Win32 handles they stand in for.
type (
	// HANDLE is the generic Win32 kernel object handle.
	HANDLE uintptr
	// HWND is a window handle.
	HWND HANDLE
	// HINSTANCE is a module instance handle.
	HINSTANCE HANDLE
	// HICON is an icon handle.
	HICON HANDLE
	// HCURSOR is a cursor handle.
	HCURSOR HANDLE
	// HMENU is a menu handle.
	HMENU HANDLE
	// HDC is a device-context handle.
	HDC HANDLE
	// HBITMAP is a bitmap handle.
	HBITMAP HANDLE
	// HBRUSH is a brush handle.
	HBRUSH HANDLE
	// ATOM is a registered class atom.
	ATOM uint16
	// WPARAM is a message's wParam.
	WPARAM uintptr
	// LPARAM is a message's lParam.
	LPARAM uintptr
	// LRESULT is a window-procedure result.
	LRESULT uintptr
)

// HWNDMessage is HWND_MESSAGE ((HWND)-3): passed as the parent of CreateWindowEx
// it produces a message-only window — no taskbar entry, no z-order, no display —
// that still receives posted and sent messages. The tray's notification-icon
// owner uses it.
const HWNDMessage HWND = ^HWND(2)

// Point mirrors the Win32 POINT structure.
type Point struct{ X, Y int32 }

// Rect mirrors the Win32 RECT structure (left, top, right, bottom).
type Rect struct{ Left, Top, Right, Bottom int32 }

// Width returns the rectangle's width (Right-Left).
func (r Rect) Width() int32 { return r.Right - r.Left }

// Height returns the rectangle's height (Bottom-Top).
func (r Rect) Height() int32 { return r.Bottom - r.Top }

// Msg mirrors the Win32 MSG structure. GetMessage fills it; the trailing
// LPrivate field and the alignment padding Go inserts after Message reproduce
// the C layout exactly on 64-bit Windows.
type Msg struct {
	Hwnd     HWND
	Message  uint32
	WParam   WPARAM
	LParam   LPARAM
	Time     uint32
	Pt       Point
	LPrivate uint32
}

// WndClassExW mirrors the Win32 WNDCLASSEXW structure. LpfnWndProc holds the
// window-procedure callback (from [NewCallback]); the handle fields are
// uintptr-width to match the C ABI.
type WndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     HINSTANCE
	HIcon         HICON
	HCursor       HCURSOR
	HbrBackground HBRUSH
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       HICON
}

// BitmapInfoHeader mirrors the Win32 BITMAPINFOHEADER. A negative BiHeight
// requests a top-down DIB (row 0 at the top), matching a toolkit framebuffer.
type BitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

// PaintStruct mirrors the Win32 PAINTSTRUCT (BeginPaint/EndPaint). Only Hdc and
// RcPaint are usually read; the reserved tail preserves the C size.
type PaintStruct struct {
	Hdc         HDC
	FErase      int32
	RcPaint     Rect
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

// IconInfo mirrors the Win32 ICONINFO structure (CreateIconIndirect).
type IconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  HBITMAP
	HbmColor HBITMAP
}
