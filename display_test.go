package win32

import (
	"testing"
	"unsafe"
)

// The C layout of the structures the display API writes into.
//
// A field of the wrong width or a missing pad byte does not fail to build and
// does not fail to run. MONITORINFOEXW is the worst case in this package: its
// cbSize is how the OS tells the EX form from the plain MONITORINFO, so a
// layout that has drifted makes GetMonitorInfoW return FALSE for every monitor
// on the machine, with no error naming the cause — a system that looks like it
// has no displays. DISPLAY_DEVICEW fails the other way: the call succeeds and
// the names come back shifted into the wrong fields.
//
// The expected values are the 64-bit ones, which is the only Windows this
// package targets. amd64 and arm64 agree on all of them, and so does every
// other GOOS this test runs on — which is the point of declaring these
// structures outside the //go:build windows glue.
func TestDisplayStructSizes(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"MONITORINFOEXW", unsafe.Sizeof(MonitorInfoEx{}), 104},
		{"DISPLAY_DEVICEW", unsafe.Sizeof(DisplayDevice{}), 840},
	} {
		if tc.got != tc.want {
			t.Errorf("sizeof(%s) = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

// The field offsets the OS writes into. A shift here produces a plausible
// wrong answer — a monitor's work area read as its bounds, an adapter name
// read as a device id — rather than a crash.
func TestDisplayStructOffsets(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"MONITORINFOEXW.rcMonitor", unsafe.Offsetof(MonitorInfoEx{}.RcMonitor), 4},
		{"MONITORINFOEXW.rcWork", unsafe.Offsetof(MonitorInfoEx{}.RcWork), 20},
		{"MONITORINFOEXW.dwFlags", unsafe.Offsetof(MonitorInfoEx{}.DwFlags), 36},
		{"MONITORINFOEXW.szDevice", unsafe.Offsetof(MonitorInfoEx{}.SzDevice), 40},
		{"DISPLAY_DEVICEW.DeviceName", unsafe.Offsetof(DisplayDevice{}.DeviceName), 4},
		{"DISPLAY_DEVICEW.DeviceString", unsafe.Offsetof(DisplayDevice{}.DeviceString), 68},
		{"DISPLAY_DEVICEW.StateFlags", unsafe.Offsetof(DisplayDevice{}.StateFlags), 324},
		{"DISPLAY_DEVICEW.DeviceID", unsafe.Offsetof(DisplayDevice{}.DeviceID), 328},
		{"DISPLAY_DEVICEW.DeviceKey", unsafe.Offsetof(DisplayDevice{}.DeviceKey), 584},
	} {
		if tc.got != tc.want {
			t.Errorf("offsetof(%s) = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

// The size-carrying fields must hold the size of the structure that carries
// them. Setting them is the whole job of the two constructors, and getting it
// wrong is silent: GetMonitorInfoW and EnumDisplayDevicesW both simply return
// FALSE.
func TestSizePrefixedConstructors(t *testing.T) {
	if mi := NewMonitorInfoEx(); uintptr(mi.CbSize) != unsafe.Sizeof(mi) {
		t.Errorf("NewMonitorInfoEx().CbSize = %d, want %d", mi.CbSize, unsafe.Sizeof(mi))
	}
	if dd := NewDisplayDevice(); uintptr(dd.Cb) != unsafe.Sizeof(dd) {
		t.Errorf("NewDisplayDevice().Cb = %d, want %d", dd.Cb, unsafe.Sizeof(dd))
	}
}

// The DPI awareness contexts are NEGATIVE handles written as bit expressions,
// which is exactly the kind of constant that is wrong without failing: a value
// off by one asks for system-aware where per-monitor-v2 was meant, and every
// rectangle the process then reads is virtualised and plausible.
func TestDPIAwarenessContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  uintptr
		want int64
	}{
		{"DPI_AWARENESS_CONTEXT_UNAWARE", DPIAwarenessUnaware, -1},
		{"DPI_AWARENESS_CONTEXT_SYSTEM_AWARE", DPIAwarenessSystemAware, -2},
		{"DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE", DPIAwarenessPerMonitorAware, -3},
		{"DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2", DPIAwarenessPerMonitorV2, -4},
		{"DPI_AWARENESS_CONTEXT_UNAWARE_GDISCALED", DPIAwarenessUnawareGDIScaled, -5},
	} {
		if int64(tc.got) != tc.want {
			t.Errorf("%s = %#x (%d), want %d", tc.name, tc.got, int64(tc.got), tc.want)
		}
	}
	if DefaultDPI != 96 {
		t.Errorf("DefaultDPI = %d, want 96 (USER_DEFAULT_SCREEN_DPI)", DefaultDPI)
	}
}

// field builds one of the API's fixed-length UTF-16 fields, NUL-terminated the
// way the OS leaves it when the value is shorter than the array.
func field(s string, size int) []uint16 {
	b := make([]uint16, size)
	i := 0
	for _, r := range s {
		if r > 0xFFFF {
			b[i] = uint16(0xD800 + ((r - 0x10000) >> 10))
			b[i+1] = uint16(0xDC00 + ((r - 0x10000) & 0x3FF))
			i += 2
			continue
		}
		b[i] = uint16(r)
		i++
	}
	return b
}

func TestMonitorInfoExAccessors(t *testing.T) {
	mi := NewMonitorInfoEx()
	copy(mi.SzDevice[:], field(`\\.\DISPLAY1`, cchDeviceName))
	mi.RcMonitor = Rect{Left: -1920, Top: 0, Right: 0, Bottom: 1080}
	mi.RcWork = Rect{Left: -1920, Top: 0, Right: 0, Bottom: 1040}

	if got := mi.Device(); got != `\\.\DISPLAY1` {
		t.Errorf("Device() = %q, want %q", got, `\\.\DISPLAY1`)
	}
	if mi.Primary() {
		t.Error("Primary() = true with dwFlags 0")
	}
	mi.DwFlags = MonitorInfoPrimary
	if !mi.Primary() {
		t.Error("Primary() = false with MONITORINFOF_PRIMARY set")
	}
	// A monitor left of the origin is at negative coordinates, and the
	// rectangle arithmetic has to survive it — this is the layout a second
	// display placed to the left produces, not an error to clamp away.
	if w, h := mi.RcMonitor.Width(), mi.RcMonitor.Height(); w != 1920 || h != 1080 {
		t.Errorf("RcMonitor = %dx%d, want 1920x1080", w, h)
	}
}

func TestDisplayDeviceAccessors(t *testing.T) {
	dd := NewDisplayDevice()
	copy(dd.DeviceName[:], field(`\\.\DISPLAY1\Monitor0`, cchDeviceName))
	copy(dd.DeviceString[:], field("Generic PnP Monitor", cchDeviceString))
	copy(dd.DeviceID[:], field(`\\?\DISPLAY#DELA0FF#5&2a5a2f3&0&UID4353#{e6f07b5f}`, cchDeviceString))
	copy(dd.DeviceKey[:], field(`\Registry\Machine\System\CurrentControlSet\Control\Class\{4d36e96e}\0002`, cchDeviceString))

	if got, want := dd.Name(), `\\.\DISPLAY1\Monitor0`; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
	if got, want := dd.Description(), "Generic PnP Monitor"; got != want {
		t.Errorf("Description() = %q, want %q", got, want)
	}
	if got := dd.ID(); got != `\\?\DISPLAY#DELA0FF#5&2a5a2f3&0&UID4353#{e6f07b5f}` {
		t.Errorf("ID() = %q", got)
	}
	if got := dd.Key(); got != `\Registry\Machine\System\CurrentControlSet\Control\Class\{4d36e96e}\0002` {
		t.Errorf("Key() = %q", got)
	}

	for _, tc := range []struct {
		name                    string
		flags                   uint32
		active, primary, mirror bool
	}{
		{"nothing set", 0, false, false, false},
		{"active", DisplayDeviceActive, true, false, false},
		{"attached to desktop is the same bit", DisplayDeviceAttachedToDesktop, true, false, false},
		{"primary", DisplayDeviceActive | DisplayDevicePrimaryDevice, true, true, false},
		{"mirroring pseudo-device", DisplayDeviceMirroringDriver, false, false, true},
		{"pruned modes say nothing about any of them", DisplayDeviceModesPruned, false, false, false},
	} {
		dd.StateFlags = tc.flags
		if got := dd.Active(); got != tc.active {
			t.Errorf("%s: Active() = %v, want %v", tc.name, got, tc.active)
		}
		if got := dd.Primary(); got != tc.primary {
			t.Errorf("%s: Primary() = %v, want %v", tc.name, got, tc.primary)
		}
		if got := dd.Mirroring(); got != tc.mirror {
			t.Errorf("%s: Mirroring() = %v, want %v", tc.name, got, tc.mirror)
		}
	}
}

// utf16Field is the one piece of decoding in this file, and both of its cases
// are real: the OS NUL-terminates a value that fits, and does NOT terminate
// one that exactly fills the array.
func TestUTF16Field(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []uint16
		want string
	}{
		{"empty field", make([]uint16, 8), ""},
		{"terminated", []uint16{'D', 'P', '-', '2', 0, 'x', 'x', 'x'}, "DP-2"},
		{"exactly fills the array, no terminator", []uint16{'A', 'B', 'C', 'D'}, "ABCD"},
		{"nothing at all", nil, ""},
		// A monitor whose EDID names it in a non-Latin script still has to
		// come back as itself, surrogate pair and all.
		{"astral plane", []uint16{0xD83D, 0xDCFA, 0}, "\U0001F4FA"},
	} {
		if got := utf16Field(tc.in); got != tc.want {
			t.Errorf("%s: utf16Field = %q, want %q", tc.name, got, tc.want)
		}
	}
}
