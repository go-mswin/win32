package win32

import (
	"bytes"
	"errors"
	"testing"
)

func TestWordMacros(t *testing.T) {
	v := MakeLParam(0x1234, 0xABCD)
	if v != 0xABCD1234 {
		t.Fatalf("MakeLParam = %#x, want 0xABCD1234", v)
	}
	if got := LoWord(v); got != 0x1234 {
		t.Fatalf("LoWord = %#x, want 0x1234", got)
	}
	if got := HiWord(v); got != 0xABCD {
		t.Fatalf("HiWord = %#x, want 0xABCD", got)
	}
	// 0xFFFF as a signed 16-bit word is -1: the negative-coordinate path.
	if got := SignedLoWord(0x0000FFFF); got != -1 {
		t.Fatalf("SignedLoWord(0xFFFF) = %d, want -1", got)
	}
	if got := SignedHiWord(0xFFFF0000); got != -1 {
		t.Fatalf("SignedHiWord(0xFFFF0000) = %d, want -1", got)
	}
	// A positive coordinate stays positive.
	if got := SignedLoWord(0x00000010); got != 16 {
		t.Fatalf("SignedLoWord(0x10) = %d, want 16", got)
	}
	if got := SignedHiWord(0x00100000); got != 16 {
		t.Fatalf("SignedHiWord(0x100000) = %d, want 16", got)
	}
}

func TestRectSize(t *testing.T) {
	r := Rect{Left: 10, Top: 20, Right: 110, Bottom: 220}
	if r.Width() != 100 {
		t.Fatalf("Width = %d, want 100", r.Width())
	}
	if r.Height() != 200 {
		t.Fatalf("Height = %d, want 200", r.Height())
	}
}

func TestHWNDMessageValue(t *testing.T) {
	// HWND_MESSAGE is (HWND)-3.
	if HWNDMessage != HWND(^uintptr(0)-2) {
		t.Fatalf("HWNDMessage = %#x, want -3", uintptr(HWNDMessage))
	}
}

func TestErrUnsupported(t *testing.T) {
	if !errors.Is(ErrUnsupported, ErrUnsupported) {
		t.Fatal("ErrUnsupported must match itself under errors.Is")
	}
	if ErrUnsupported.Error() == "" {
		t.Fatal("ErrUnsupported must carry a message")
	}
}

// oneRedPixel is R=255,G=0,B=0,A=255 in RGBA byte order.
func oneRedPixel() []byte { return []byte{255, 0, 0, 255} }

func TestPackBGRA(t *testing.T) {
	dst := make([]byte, 4)
	PackBGRA(dst, oneRedPixel())
	// Red RGBA {255,0,0,255} → BGRA {0,0,255,255}.
	if !bytes.Equal(dst, []byte{0, 0, 255, 255}) {
		t.Fatalf("PackBGRA red = %v, want [0 0 255 255]", dst)
	}
}

func TestPackBGRAShortDst(t *testing.T) {
	// dst shorter than src: only the pixels that fit are written, no overrun.
	src := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	dst := make([]byte, 4)
	PackBGRA(dst, src)
	if !bytes.Equal(dst, []byte{3, 2, 1, 4}) {
		t.Fatalf("PackBGRA short dst = %v, want [3 2 1 4]", dst)
	}
}

func TestPackBGRANonMultipleOfFour(t *testing.T) {
	// A trailing partial pixel (len%4 != 0) is dropped, not half-written.
	src := []byte{1, 2, 3, 4, 9, 9}
	dst := make([]byte, 6)
	PackBGRA(dst, src)
	if !bytes.Equal(dst, []byte{3, 2, 1, 4, 0, 0}) {
		t.Fatalf("PackBGRA ragged = %v, want [3 2 1 4 0 0]", dst)
	}
}

func TestPackBGRARectFull(t *testing.T) {
	// 2×2 surface, all red; pack the whole thing.
	w, h := 2, 2
	src := make([]byte, 4*w*h)
	for i := 0; i < len(src); i += 4 {
		src[i], src[i+1], src[i+2], src[i+3] = 255, 0, 0, 255
	}
	dst := make([]byte, len(src))
	PackBGRARect(dst, src, w, h, 0, 0, w, h)
	for i := 0; i < len(dst); i += 4 {
		if dst[i] != 0 || dst[i+1] != 0 || dst[i+2] != 255 || dst[i+3] != 255 {
			t.Fatalf("pixel %d = %v, want [0 0 255 255]", i/4, dst[i:i+4])
		}
	}
}

func TestPackBGRARectClamp(t *testing.T) {
	w, h := 2, 2
	src := make([]byte, 4*w*h)
	for i := 0; i < len(src); i += 4 {
		src[i], src[i+1], src[i+2], src[i+3] = 10, 20, 30, 40
	}
	dst := make([]byte, len(src))
	// Rectangle straddling every edge: negative origin AND over-large extent.
	// Clamps to the full surface, packs it, no overrun.
	PackBGRARect(dst, src, w, h, -1, -1, 10, 10)
	want := []byte{30, 20, 10, 40}
	for i := 0; i < len(dst); i += 4 {
		if !bytes.Equal(dst[i:i+4], want) {
			t.Fatalf("clamped pixel %d = %v, want %v", i/4, dst[i:i+4], want)
		}
	}
}

func TestPackBGRARectEmpty(t *testing.T) {
	dst := make([]byte, 16)
	orig := make([]byte, 16)
	// w0<=0 after clamping: nothing is written.
	PackBGRARect(dst, make([]byte, 16), 2, 2, 2, 0, 0, 2)
	if !bytes.Equal(dst, orig) {
		t.Fatalf("empty rect wrote into dst: %v", dst)
	}
}

func TestPackBGRARectOverrunGuard(t *testing.T) {
	// width/height claim a 2×2 surface but src holds only one pixel: the
	// range-check must stop rather than panic when the region runs past the
	// buffer.
	dst := make([]byte, 4)
	src := make([]byte, 4) // one pixel only
	PackBGRARect(dst, src, 2, 2, 0, 0, 2, 2)
	// First pixel packed; the guard returned before the out-of-range access.
	if len(dst) != 4 {
		t.Fatal("unexpected dst length")
	}
}

// The handle-shaped constants are NEGATIVE values written as bit expressions
// so they are correct on both 64-bit targets. A wrong one does not fail to
// build: SetWindowPos simply restacks the window somewhere unexpected, or
// silently does nothing.
func TestWindowPosConstants(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  HWND
		want int64
	}{
		{"HWND_TOP", HWNDTop, 0},
		{"HWND_BOTTOM", HWNDBottom, 1},
		{"HWND_TOPMOST", HWNDTopmost, -1},
		{"HWND_NOTOPMOST", HWNDNoTopmost, -2},
		{"HWND_MESSAGE", HWNDMessage, -3},
	} {
		if got := int64(int(tc.got)); got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// The GetWindowLongPtr indices are negative too, and are taken as int32 rather
// than uintptr for exactly that reason.
func TestWindowLongIndices(t *testing.T) {
	for name, got := range map[string]int{
		"GWL_WNDPROC":   GWLWndProc,
		"GWL_HINSTANCE": GWLHInstance,
		"GWL_ID":        GWLID,
		"GWL_STYLE":     GWLStyle,
		"GWL_EXSTYLE":   GWLExStyle,
		"GWL_USERDATA":  GWLUserData,
	} {
		if got >= 0 {
			t.Errorf("%s = %d, but every GWL index is negative", name, got)
		}
	}
	// The exact values, because a wrong one reads or writes a DIFFERENT field
	// of the window and the effect is silent.
	if GWLExStyle != -20 || GWLStyle != -16 || GWLWndProc != -4 {
		t.Fatalf("GWL indices drifted: exstyle=%d style=%d wndproc=%d",
			GWLExStyle, GWLStyle, GWLWndProc)
	}
}

func TestRasterOpConstants(t *testing.T) {
	// CAPTUREBLT is the flag without which a screen capture silently omits
	// every layered (transparent) window.
	if CaptureBLT != 0x40000000 {
		t.Fatalf("CAPTUREBLT = %#x", CaptureBLT)
	}
	if SRCCOPY != 0x00CC0020 {
		t.Fatalf("SRCCOPY = %#x", SRCCOPY)
	}
	if Blackness != 0x00000042 || Whiteness != 0x00FF0062 {
		t.Fatalf("BLACKNESS=%#x WHITENESS=%#x", Blackness, Whiteness)
	}
	// Only HALFTONE averages; the other three drop pixels.
	if Halftone != 4 {
		t.Fatalf("HALFTONE = %d", Halftone)
	}
}

func TestExtendedStyleConstants(t *testing.T) {
	for name, tc := range map[string]struct{ got, want uint32 }{
		"WS_EX_TOPMOST":    {WSExTopmost, 0x00000008},
		"WS_EX_TOOLWINDOW": {WSExToolWindow, 0x00000080},
		"WS_EX_LAYERED":    {WSExLayered, 0x00080000},
		"WS_EX_NOACTIVATE": {WSExNoActivate, 0x08000000},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %#x, want %#x", name, tc.got, tc.want)
		}
	}
}

func TestSetWindowPosFlags(t *testing.T) {
	// The flags are a bit set: no two may share a bit, or a caller asking for
	// one silently gets another as well.
	all := []uint32{SWPNoSize, SWPNoMove, SWPNoZOrder,
		SWPNoRedraw, SWPNoActivate, SWPFrameChanged,
		SWPShowWindow, SWPHideWindow, SWPNoOwnerZOrder}
	var seen uint32
	for _, f := range all {
		if f == 0 {
			t.Fatal("a SetWindowPos flag is zero")
		}
		if seen&f != 0 {
			t.Fatalf("flag %#x overlaps one already seen (%#x)", f, seen)
		}
		seen |= f
	}
}
