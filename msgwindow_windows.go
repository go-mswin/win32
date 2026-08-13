//go:build windows

package win32

import "unsafe"

// MessageWindow is a hidden, message-only window: CreateWindowEx with
// [HWNDMessage] as parent gives it no taskbar entry, no z-order and no display,
// yet it still receives posted and sent messages. It is the owner a
// Shell_NotifyIcon tray icon or a background message pump needs. NewMessageWindow
// registers a class named className bound to wndProc, then creates the window.
type MessageWindow struct {
	Hwnd      HWND
	HInstance HINSTANCE
	class     *uint16
}

// NewMessageWindow registers a window class named className whose procedure is
// wndProc — a callback already produced by [NewCallback], so the caller owns the
// once-per-class allocation — and creates a message-only window of that class.
// It returns a wrapped error if the class registration or window creation fails.
func NewMessageWindow(className string, wndProc uintptr) (*MessageWindow, error) {
	class, err := UTF16PtrFromString(className)
	if err != nil {
		return nil, err
	}
	inst := GetModuleHandle(nil)
	cursor := LoadCursor(0, IDCArrow)

	wc := WndClassExW{
		CbSize:        uint32(unsafe.Sizeof(WndClassExW{})),
		LpfnWndProc:   wndProc,
		HInstance:     inst,
		HCursor:       cursor,
		LpszClassName: class,
	}
	if _, err := RegisterClassEx(&wc); err != nil {
		return nil, err
	}
	hwnd, err := CreateWindowEx(0, class, class, 0, 0, 0, 0, 0, HWNDMessage, 0, inst, nil)
	if err != nil {
		return nil, err
	}
	return &MessageWindow{Hwnd: hwnd, HInstance: inst, class: class}, nil
}

// Destroy destroys the underlying window. Safe to call once; the handle is
// cleared afterwards.
func (m *MessageWindow) Destroy() {
	if m.Hwnd != 0 {
		DestroyWindow(m.Hwnd)
		m.Hwnd = 0
	}
}
