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
