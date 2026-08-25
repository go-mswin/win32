package win32

// Window messages (winuser.h). Only the ones the fleet's backends handle are
// named here; consumers add their own as needed.
const (
	WMNull        = 0x0000
	WMCreate      = 0x0001
	WMDestroy     = 0x0002
	WMSize        = 0x0005
	WMClose       = 0x0010
	WMQuit        = 0x0012
	WMPaint       = 0x000F
	WMEraseBkgnd  = 0x0014
	WMGetObject   = 0x003D
	WMKeyDown     = 0x0100
	WMKeyUp       = 0x0101
	WMChar        = 0x0102
	WMCommand     = 0x0111
	WMMouseMove   = 0x0200
	WMLButtonDown = 0x0201
	WMLButtonUp   = 0x0202
	WMRButtonDown = 0x0204
	WMRButtonUp   = 0x0205
	WMContextMenu = 0x007B
	WMMove        = 0x0003
	WMMouseWheel  = 0x020A
	WMDpiChanged  = 0x02E0

	// WMApp is the base of the application-private message range; a tray icon's
	// callback message is typically WMApp+1.
	WMApp = 0x8000
)

// Window styles (winuser.h).
const (
	WSOverlappedWindow = 0x00CF0000 // titled, resizable, min/max
)

// ShowWindow commands.
const (
	SWHide = 0
	SWShow = 5
)

// Class styles.
const (
	CSVRedraw = 0x0001
	CSHRedraw = 0x0002
)

// CWUseDefault is CW_USEDEFAULT: let Windows pick the position/size.
const CWUseDefault = ^uintptr(0) &^ (^uintptr(0) >> 1) // 0x80000000

// Standard cursor ids for LoadCursor(0, …).
const IDCArrow = 32512

// GDI StretchDIBits parameters.
const (
	BIRGB        = 0          // BI_RGB (uncompressed)
	DIBRGBColors = 0          // DIB_RGB_COLORS
	SRCCOPY      = 0x00CC0020 // SRCCOPY raster op
)

// SetWindowPos flags.
const (
	SWPNoZOrder   = 0x0004
	SWPNoActivate = 0x0010
)

// Popup-menu flags (AppendMenuW / TrackPopupMenu).
const (
	MFString    = 0x0000
	MFGrayed    = 0x0001
	MFChecked   = 0x0008
	MFPopup     = 0x0010
	MFSeparator = 0x0800

	TPMReturnCmd   = 0x0100
	TPMRightButton = 0x0002
)

// Shell_NotifyIcon actions and flags (shellapi.h).
const (
	NIMAdd    = 0x0
	NIMModify = 0x1
	NIMDelete = 0x2

	NIFMessage = 0x1
	NIFIcon    = 0x2
	NIFTip     = 0x4
)

// Extended window styles (winuser.h).
const (
	// WSExTopmost keeps the window above every non-topmost window.
	WSExTopmost = 0x00000008
	// WSExToolWindow is a floating palette: no taskbar entry, and excluded
	// from the Alt-Tab list.
	WSExToolWindow = 0x00000080
	// WSExLayered enables per-pixel alpha and colour keying. A layered window
	// is only included in a screen capture when the blit asks for
	// [CaptureBLT].
	WSExLayered = 0x00080000
	// WSExNoActivate keeps the window from taking the foreground when clicked.
	WSExNoActivate = 0x08000000
)

// GetWindowLongPtr / SetWindowLongPtr indices (winuser.h). They are NEGATIVE,
// which is why the wrappers take an int32 and sign-extend rather than making
// every caller build a uintptr by hand.
const (
	GWLWndProc   = -4
	GWLHInstance = -6
	GWLID        = -12
	GWLStyle     = -16
	GWLExStyle   = -20
	GWLUserData  = -21
)

// SetWindowPos flags (winuser.h). SWPNoZOrder and SWPNoActivate are declared
// with the tray's constants above.
const (
	SWPNoSize        = 0x0001
	SWPNoMove        = 0x0002
	SWPNoRedraw      = 0x0008
	SWPFrameChanged  = 0x0020
	SWPShowWindow    = 0x0040
	SWPHideWindow    = 0x0080
	SWPNoOwnerZOrder = 0x0200
)

// SetWindowPos hWndInsertAfter values (winuser.h). They are handle-shaped
// NEGATIVE constants, hence the bit expressions rather than plain literals:
// written this way they are correct on both 64-bit targets. A wrong one does
// not fail to build — the window simply lands somewhere unexpected in the
// z-order, or the call silently does nothing.
const (
	HWNDTop       HWND = 0
	HWNDBottom    HWND = 1
	HWNDTopmost   HWND = ^HWND(0) // -1
	HWNDNoTopmost HWND = ^HWND(1) // -2
)

// Raster operations for BitBlt, StretchBlt and PatBlt (wingdi.h). SRCCOPY is
// declared with the DIB constants above.
const (
	// Blackness fills the destination with black using no source. It is the
	// cheapest way to clear a device context before a PrintWindow, which
	// COMPOSITES rather than overwriting.
	Blackness = 0x00000042
	// Whiteness fills the destination with white.
	Whiteness = 0x00FF0062
	// CaptureBLT includes LAYERED windows in the result. Without it, anything
	// with transparency — which on a modern desktop is most menu and
	// notification surfaces — is simply absent from a screen capture.
	CaptureBLT = 0x40000000
)

// StretchBlt modes (wingdi.h). Only [Halftone] AVERAGES pixels rather than
// dropping them, which on a downscaled desktop is the difference between
// readable text and noise. The mode is per-device-context state.
const (
	BlackOnWhite = 1
	WhiteOnBlack = 2
	ColorOnColor = 3
	Halftone     = 4
)
