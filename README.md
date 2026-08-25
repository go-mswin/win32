# win32 — pure-Go bindings to the Windows API

`github.com/go-mswin/win32` is the owned, pure-Go **CGO=0** foundation for
calling the Windows API from Go — the peer of [`go-macos/objc`](https://github.com/go-macos).
It targets **64-bit Windows** (amd64 + arm64); "win32" is the Windows API name,
not a bitness (there is no separate "Win64 API").

Built on `golang.org/x/sys/windows` (no cgo). Provides the shared surface the
fleet currently hand-rolls in more than one place:

- lazy DLL + proc binding for `user32` / `gdi32` / `kernel32` / `advapi32` / `combase`
- a hidden message window + `GetMessage`/`TranslateMessage`/`DispatchMessage` pump
- `WNDCLASSEXW` / `MSG` / rect / point types and the common `CreateWindowExW`,
  `RegisterClassExW`, `DefWindowProcW`, `LoadCursorW`, `PostQuitMessage`, …
- top-down 32-bpp BGRA `StretchDIBits` blit helper
- **device contexts and GDI objects**: `GetDC` / `GetWindowDC` / `ReleaseDC`,
  `CreateCompatibleDC` / `DeleteDC`, `SelectObject` / `DeleteObject`,
  `BitBlt` / `StretchBlt` / `PatBlt`, `SetStretchBltMode`, `GetDeviceCaps`
- **window state and geometry**: `ShowWindow`, `UpdateWindow`,
  `InvalidateRect`, `SetWindowPos`, `SetForegroundWindow`,
  `GetForegroundWindow`, `GetWindowRect`, `GetClientRect`, `IsWindow`,
  `IsWindowVisible`, `IsIconic`, `GetWindowText`, `GetClassName`,
  `GetWindowLongPtr` / `SetWindowLongPtr` (which `x/sys/windows` does not wrap)

## Consumers
`go-widgets/tray`, `go-widgets/window` (win32 backend), `go-mswin/screencapture`,
and the WinRT plumbing in the weft Windows apps — one owned binding instead of
duplicated hand-rolls.

## License
BSD-3-Clause — copyright the go-mswin authors.
