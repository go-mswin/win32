//go:build windows

package win32_test

// Live, on-a-real-Windows-machine proof of the win32 glue. These tests are
// gated behind environment variables so ordinary `go test` (headless CI) skips
// them; they are cross-compiled here (`go test -c`), copied to the Win11 ARM64
// QEMU VM and run there.
//
//   TestLiveMessagePump — a hidden message-only window (NewMessageWindow),
//   PostMessage of a sentinel, then Pump; the window procedure must receive the
//   sentinel and PostQuitMessage to end the pump. Needs no desktop, so it runs
//   over ssh (session 0) and prints a PASS the caller reads on stdout.
//
//   TestLiveBlit — a visible window whose WM_PAINT blits a framebuffer through
//   StretchDIBitsBGRA, held open long enough to screendump the ramfb. It must
//   run in the INTERACTIVE session (schtasks /IT) so the window reaches the
//   framebuffer the qemu monitor captures. WIN32_BLIT_WITNESS=1 paints a solid
//   magenta witness (control run: proves the window+blit path and that BGRA
//   bytes land as the intended colour); unset paints a four-quadrant image
//   (instrument run: proves channel order AND top-down orientation at once).

import (
	"os"
	"strconv"
	"testing"
	"time"
	"unsafe"

	win32 "github.com/go-mswin/win32"
)

// pumpGot records the sentinel the window procedure saw, for the assertion.
var pumpGot win32.WPARAM

const sentinel = win32.WPARAM(0xDEAD)

func pumpProc(hwnd win32.HWND, msg uint32, wParam win32.WPARAM, lParam win32.LPARAM) win32.LRESULT {
	if msg == win32.WMApp+1 {
		pumpGot = wParam
		win32.PostQuitMessage(0)
		return 0
	}
	return win32.DefWindowProc(hwnd, msg, wParam, lParam)
}

func TestLiveMessagePump(t *testing.T) {
	if os.Getenv("WIN32_LIVE") != "1" {
		t.Skip("set WIN32_LIVE=1 to run the live message-pump round-trip on Windows")
	}
	mw, err := win32.NewMessageWindow("GoMSWinPumpProbe", win32.NewCallback(pumpProc))
	if err != nil {
		t.Fatalf("NewMessageWindow: %v", err)
	}
	defer mw.Destroy()

	if !win32.PostMessage(mw.Hwnd, win32.WMApp+1, sentinel, 0) {
		t.Fatal("PostMessage returned false")
	}
	if err := win32.Pump(); err != nil {
		t.Fatalf("Pump: %v", err)
	}
	if pumpGot != sentinel {
		t.Fatalf("window procedure saw wParam=%#x, want %#x", uintptr(pumpGot), uintptr(sentinel))
	}
	t.Logf("PUMP_OK message-only window round-trip delivered wParam=%#x", uintptr(pumpGot))
}

// blit state, read by blitProc's WM_PAINT.
var (
	blitBGRA []byte
	blitW    int32
	blitH    int32
)

var (
	procBeginPaint   = win32.User32.NewProc("BeginPaint")
	procEndPaint     = win32.User32.NewProc("EndPaint")
	procShowWindow   = win32.User32.NewProc("ShowWindow")
	procUpdateWindow = win32.User32.NewProc("UpdateWindow")
	procPeekMessageW = win32.User32.NewProc("PeekMessageW")
)

const pmRemove = 0x0001

func blitProc(hwnd win32.HWND, msg uint32, wParam win32.WPARAM, lParam win32.LPARAM) win32.LRESULT {
	switch msg {
	case win32.WMPaint:
		var ps win32.PaintStruct
		hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			win32.StretchDIBitsBGRA(win32.HDC(hdc), 0, 0, blitW, blitH, blitW, blitH, blitBGRA)
			procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		}
		return 0
	case win32.WMEraseBkgnd:
		return 1
	case win32.WMDestroy:
		win32.PostQuitMessage(0)
		return 0
	}
	return win32.DefWindowProc(hwnd, msg, wParam, lParam)
}

// setPixel writes an RGBA pixel into a top-down RGBA buffer.
func setPixel(rgba []byte, w, x, y int, r, g, b byte) {
	i := (y*w + x) * 4
	rgba[i], rgba[i+1], rgba[i+2], rgba[i+3] = r, g, b, 255
}

func TestLiveBlit(t *testing.T) {
	if os.Getenv("WIN32_LIVE_BLIT") != "1" {
		t.Skip("set WIN32_LIVE_BLIT=1 (interactive session) to run the live StretchDIBits blit on Windows")
	}
	const w, h = 240, 160
	blitW, blitH = w, h
	rgba := make([]byte, 4*w*h)
	if os.Getenv("WIN32_BLIT_WITNESS") == "1" {
		// Control run: solid magenta (R=255,G=0,B=255). If the window shows
		// magenta, the window+blit path works and BGRA bytes map to the colour
		// intended — before any orientation/channel claim about the quadrants.
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				setPixel(rgba, w, x, y, 255, 0, 255)
			}
		}
		t.Log("BLIT_WITNESS solid magenta")
	} else {
		// Instrument run: TL red, TR green, BL blue, BR white. Each corner is a
		// distinct witness of both channel order and top-down orientation.
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				top := y < h/2
				left := x < w/2
				switch {
				case top && left:
					setPixel(rgba, w, x, y, 255, 0, 0) // red
				case top && !left:
					setPixel(rgba, w, x, y, 0, 255, 0) // green
				case !top && left:
					setPixel(rgba, w, x, y, 0, 0, 255) // blue
				default:
					setPixel(rgba, w, x, y, 255, 255, 255) // white
				}
			}
		}
		t.Log("BLIT_QUADRANT TL=red TR=green BL=blue BR=white")
	}
	blitBGRA = make([]byte, len(rgba))
	win32.PackBGRA(blitBGRA, rgba)

	className, _ := win32.UTF16PtrFromString("GoMSWinBlitProbe")
	title, _ := win32.UTF16PtrFromString("go-mswin/win32 blit probe")
	inst := win32.GetModuleHandle(nil)
	wc := win32.WndClassExW{
		CbSize:        uint32(unsafe.Sizeof(win32.WndClassExW{})),
		Style:         win32.CSHRedraw | win32.CSVRedraw,
		LpfnWndProc:   win32.NewCallback(blitProc),
		HInstance:     inst,
		HCursor:       win32.LoadCursor(0, win32.IDCArrow),
		LpszClassName: className,
	}
	if _, err := win32.RegisterClassEx(&wc); err != nil {
		t.Fatalf("RegisterClassEx: %v", err)
	}
	hwnd, err := win32.CreateWindowEx(0, className, title, win32.WSOverlappedWindow,
		100, 100, w+40, h+80, 0, 0, inst, nil)
	if err != nil {
		t.Fatalf("CreateWindowEx: %v", err)
	}
	procShowWindow.Call(uintptr(hwnd), win32.SWShow)
	procUpdateWindow.Call(uintptr(hwnd))

	// Hold the window open (draining messages non-blockingly so it repaints)
	// long enough to screendump the ramfb. WIN32_BLIT_HOLD overrides the seconds.
	holdSec := 30
	if v := os.Getenv("WIN32_BLIT_HOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			holdSec = n
		}
	}
	deadline := time.Now().Add(time.Duration(holdSec) * time.Second)
	var m win32.Msg
	for time.Now().Before(deadline) {
		r, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, pmRemove)
		if r != 0 {
			win32.TranslateMessage(&m)
			win32.DispatchMessage(&m)
		} else {
			time.Sleep(20 * time.Millisecond)
		}
	}
	win32.DestroyWindow(hwnd)
	t.Log("BLIT_DONE window held for screendump")
}
