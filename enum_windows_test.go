//go:build windows

package win32_test

import (
	"testing"

	win32 "github.com/go-mswin/win32"
)

// The regression this whole file exists for.
//
// windows.NewCallback allocates out of a pool the Go runtime caps at 2000 for
// the entire process (runtime.cb_max), and going past it is not a recoverable
// panic — it is runtime.throw("too many callback functions"), which kills the
// process and cannot be recovered from. A wrapper that builds its callback
// PER CALL therefore does not leak slowly; it puts a hard ceiling of 2000 on
// how many times the application may ever ask, and then takes the process
// down. go-mswin/screencapture had exactly that, in both of its enumerations,
// so Shareable() burned two of the 2000 per call.
//
// 3000 walks is comfortably past the ceiling. With a trampoline per call this
// test does not fail — the whole test BINARY dies partway through, which is
// the shape of the defect: no error to inspect, no stack a caller can catch.
// Surviving is the proof.
//
// Each callback stops its walk immediately, so this measures the trampolines
// and not the desktop's window count. It needs no interactive session, which
// is why it is not gated: it runs on every Windows CI lane, on hardware nobody
// here controls.
func TestEnumerationsDoNotConsumeACallbackPerCall(t *testing.T) {
	const calls = 3000 // > runtime.cb_max (2000)
	for i := range calls {
		if err := win32.EnumDisplayMonitors(func(win32.HMONITOR, win32.HDC, win32.Rect) bool {
			return false
		}); err != nil {
			t.Fatalf("EnumDisplayMonitors call %d: %v", i, err)
		}
		if err := win32.EnumWindows(func(win32.HWND) bool {
			return false
		}); err != nil {
			t.Fatalf("EnumWindows call %d: %v", i, err)
		}
	}
	t.Logf("CALLBACK_CEILING_OK %d walks of each enumeration, %d past runtime.cb_max",
		calls, calls-2000)
}

// EnumWindows must find the desktop it is walking. A session with no
// interactive desktop still has top-level windows — the shell's, the service
// host's — so an empty walk means the enumeration did not run, not that the
// machine is quiet.
func TestEnumWindowsFindsWindows(t *testing.T) {
	n := 0
	if err := win32.EnumWindows(func(hwnd win32.HWND) bool {
		if hwnd != 0 && win32.IsWindow(hwnd) {
			n++
		}
		return true
	}); err != nil {
		t.Fatalf("EnumWindows: %v", err)
	}
	if n == 0 {
		t.Fatal("EnumWindows reported no window at all")
	}
	t.Logf("ENUMWINDOWS_OK %d top-level windows", n)
}

// A callback that stops the walk got what it asked for, and the OS reports
// that exactly as it reports a failure. Confusing the two would make every
// early exit an error.
func TestEnumWindowsStopsWithoutAnError(t *testing.T) {
	seen := 0
	if err := win32.EnumWindows(func(win32.HWND) bool {
		seen++
		return false
	}); err != nil {
		t.Fatalf("EnumWindows stopped by its callback: %v", err)
	}
	if seen != 1 {
		t.Errorf("callback returning false ran %d times, want 1", seen)
	}
}

// A nil callback is a programming error and must say so rather than handing
// the OS a zero trampoline to call.
func TestEnumerationsRefuseANilCallback(t *testing.T) {
	if err := win32.EnumWindows(nil); err == nil {
		t.Error("EnumWindows(nil) succeeded")
	}
	if err := win32.EnumDisplayMonitors(nil); err == nil {
		t.Error("EnumDisplayMonitors(nil) succeeded")
	}
}

// GetWindowThreadProcessID must name a real process for a real window, and
// nothing for a handle that names none.
func TestGetWindowThreadProcessID(t *testing.T) {
	var any win32.HWND
	_ = win32.EnumWindows(func(hwnd win32.HWND) bool {
		any = hwnd
		return false
	})
	if any == 0 {
		t.Skip("no window to ask about")
	}
	pid, tid := win32.GetWindowThreadProcessID(any)
	if pid == 0 || tid == 0 {
		t.Errorf("GetWindowThreadProcessID(%#x) = pid %d, tid %d; want both non-zero",
			uintptr(any), pid, tid)
	}
	if pid, tid := win32.GetWindowThreadProcessID(0); pid != 0 || tid != 0 {
		t.Errorf("GetWindowThreadProcessID(0) = pid %d, tid %d; want 0, 0", pid, tid)
	}
}
