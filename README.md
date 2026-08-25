# win32 — pure-Go bindings to the Windows API

`github.com/go-mswin/win32` is the owned, pure-Go **CGO=0** foundation for
calling the Windows API from Go — the peer of [`go-macos/objc`](https://github.com/go-macos).
It targets **64-bit Windows** (amd64 + arm64); "win32" is the Windows API name,
not a bitness (there is no separate "Win64 API").

Built on `golang.org/x/sys/windows` (no cgo). Provides the shared surface the
fleet currently hand-rolls in more than one place:

- lazy DLL + proc binding for `user32` / `gdi32` / `kernel32` / `advapi32` /
  `shell32` / `combase` / `shcore`
- a hidden message window + `GetMessage`/`TranslateMessage`/`DispatchMessage` pump
- `WNDCLASSEXW` / `MSG` / rect / point types and the common `CreateWindowExW`,
  `RegisterClassExW`, `DefWindowProcW`, `LoadCursorW`, `PostQuitMessage`, …
- top-down 32-bpp BGRA `StretchDIBits` blit helper
- **device contexts and GDI objects**: `GetDC` / `GetWindowDC` / `ReleaseDC`,
  `CreateCompatibleDC` / `DeleteDC`, `SelectObject` / `DeleteObject`,
  `BitBlt` / `StretchBlt` / `PatBlt`, `SetStretchBltMode`, `GetDeviceCaps`
- **callback-driven enumeration**: `EnumDisplayMonitors` and `EnumWindows`,
  each with ONE process-wide trampoline. `windows.NewCallback` allocates out of
  a pool the runtime caps at 2000 and going past it is `runtime.throw`, not a
  recoverable panic — so a wrapper that builds its callback per call puts a
  hard ceiling on how many times an application may ask, and then kills it
- **display enumeration**: `GetMonitorInfo` (`MONITORINFOEXW`, `cbSize` set for
  you — a wrong one is silently rejected), `EnumDisplayDevices` /
  `DisplayAdapters` / `DisplayMonitors` for the adapter and panel names,
  `GetDpiForMonitor` off `shcore`, `MonitorFromWindow`, and
  `SetProcessDPIAwarenessContext` with the Per-Monitor-V2 context — without
  which every rectangle the process reads is virtualised and plausible
- **window state and geometry**: `ShowWindow`, `UpdateWindow`,
  `InvalidateRect`, `SetWindowPos`, `SetForegroundWindow`,
  `GetForegroundWindow`, `GetWindowRect`, `GetClientRect`, `IsWindow`,
  `IsWindowVisible`, `IsIconic`, `GetWindowText`, `GetClassName`,
  `GetWindowLongPtr` / `SetWindowLongPtr` (which `x/sys/windows` does not wrap),
  `GetWindowThreadProcessID`

## Consumers
`go-widgets/tray`, `go-widgets/window` (win32 backend), `go-mswin/screencapture`,
and the WinRT plumbing in the weft Windows apps — one owned binding instead of
duplicated hand-rolls.

## Verification

The live glue is proven on a real Windows machine and the record is committed:
`win32-vm-proof-2026-08-13.txt` (message pump, BGRA blit) and
`win32-display-vm-proof-2026-08-25.txt` (display enumeration, compared field by
field against `System.Windows.Forms.Screen` and `dxdiag`).

The callback ceiling is guarded by an **ungated** test —
`TestEnumerationsDoNotConsumeACallbackPerCall` runs 3000 walks of each
enumeration on every Windows CI lane. It needs no desktop. With a trampoline
per call it does not fail; the test binary dies partway through, which is the
shape of the defect.

## License
BSD-3-Clause — copyright the go-mswin authors.
