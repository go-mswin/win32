package win32

// OS-independent helpers: the LPARAM/WPARAM word macros every message handler
// needs, and the RGBA→BGRA packing that turns a toolkit framebuffer into the
// byte order a Win32 32-bpp DIB expects. All pure, all exercised to 100% on
// every GOOS — the live StretchDIBits blit that consumes the packed bytes is in
// the //go:build windows glue.

// LoWord returns the low 16 bits of a 32-bit packed value (LOWORD).
func LoWord(v uint32) uint32 { return v & 0xFFFF }

// HiWord returns the high 16 bits of a 32-bit packed value (HIWORD).
func HiWord(v uint32) uint32 { return (v >> 16) & 0xFFFF }

// SignedLoWord returns the low word interpreted as a signed 16-bit value
// (GET_X_LPARAM): mouse coordinates can be negative on multi-monitor desktops.
func SignedLoWord(v uint32) int { return int(int16(v & 0xFFFF)) }

// SignedHiWord returns the high word interpreted as a signed 16-bit value
// (GET_Y_LPARAM).
func SignedHiWord(v uint32) int { return int(int16((v >> 16) & 0xFFFF)) }

// MakeLParam packs a low and high 16-bit word into a 32-bit value (MAKELPARAM).
func MakeLParam(lo, hi uint16) uint32 { return uint32(lo) | uint32(hi)<<16 }

// PackBGRA repacks a top-down RGBA pixel buffer (R,G,B,A) into the B,G,R,A byte
// order of a Win32 32-bpp DIB (a little-endian 0xAARRGGBB pixel). It writes as
// many whole pixels as fit in both slices, so a short dst is truncated rather
// than overrun.
func PackBGRA(dst, src []byte) {
	n := len(src)
	if len(dst) < n {
		n = len(dst)
	}
	n -= n % 4
	for i := 0; i < n; i += 4 {
		dst[i+0] = src[i+2] // B ← R
		dst[i+1] = src[i+1] // G
		dst[i+2] = src[i+0] // R ← B
		dst[i+3] = src[i+3] // A
	}
}

// PackBGRARect repacks only the rectangle (x,y,w0,h0) of a width×height RGBA
// surface into the matching region of the BGRA dst, for incremental
// (damage-region) present. The rectangle is clamped to the surface bounds, and
// each pixel access is range-checked so a rectangle that overruns either buffer
// stops rather than panics.
func PackBGRARect(dst, src []byte, width, height, x, y, w0, h0 int) {
	if x < 0 {
		w0 += x
		x = 0
	}
	if y < 0 {
		h0 += y
		y = 0
	}
	if x+w0 > width {
		w0 = width - x
	}
	if y+h0 > height {
		h0 = height - y
	}
	if w0 <= 0 || h0 <= 0 {
		return
	}
	stride := width * 4
	for row := y; row < y+h0; row++ {
		base := row*stride + x*4
		for i := base; i < base+w0*4; i += 4 {
			if i+3 >= len(src) || i+3 >= len(dst) {
				return
			}
			dst[i+0] = src[i+2] // B ← R
			dst[i+1] = src[i+1] // G
			dst[i+2] = src[i+0] // R ← B
			dst[i+3] = src[i+3] // A
		}
	}
}
