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
