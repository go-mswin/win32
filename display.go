package win32

import (
	"unicode/utf16"
	"unsafe"
)

// The display-enumeration surface: what the OS says about the monitors
// attached to the machine.
//
// Everything here is OS-INDEPENDENT on purpose. These are the structures the
// Windows API writes into and the flags it writes into them, and they are
// plain Go declarations with no x/sys/windows import, so they compile — and
// are size-checked by the test suite — on every GOOS. That matters more here
// than anywhere else in this package: a field of the wrong width or a missing
// pad byte does not fail to build and does not fail to run. It silently shifts
// every field after it, so a monitor reports a plausible wrong rectangle, or
// GetMonitorInfoW rejects the structure outright because the cbSize it was
// handed no longer matches any form of MONITORINFO it knows. Only an assertion
// catches that, and display_test.go makes it.
//
// The live calls are in display_windows.go.

// HMONITOR is a display monitor handle, as EnumDisplayMonitors reports and
// GetMonitorInfoW consumes.
//
// It is a SNAPSHOT identity, not a durable one: a monitor that is unplugged
// and plugged back in gets a different HMONITOR, and a stale handle can be
// reused for a different monitor. Match a monitor again by its device name
// (`\\.\DISPLAY1`), not by this.
type HMONITOR HANDLE

// cchDeviceName is CCHDEVICENAME (wingdi.h): the fixed length of the GDI
// device-name fields, in UTF-16 code units. It is 32 whatever the API, and a
// device name that reaches it is NOT NUL-terminated — hence [utf16Field].
const cchDeviceName = 32

// cchDeviceString is the fixed length of DISPLAY_DEVICEW's three long fields
// (wingdi.h), in UTF-16 code units.
const cchDeviceString = 128

// MonitorInfoEx mirrors MONITORINFOEXW (winuser.h): what GetMonitorInfoW says
// about one monitor.
//
// CbSize MUST be the structure's own size before the call. That is how the OS
// tells the EX form (with SzDevice) from the plain MONITORINFO, and a wrong
// value is SILENTLY REJECTED — GetMonitorInfoW simply returns FALSE, with no
// error that names the cause. Use [NewMonitorInfoEx] rather than filling the
// field by hand.
type MonitorInfoEx struct {
	CbSize uint32
	// RcMonitor is the monitor's full rectangle on the virtual screen, in
	// PHYSICAL pixels for a process that has declared per-monitor DPI
	// awareness and in virtualised ones for a process that has not. The
	// primary monitor's top-left is the origin, so a monitor placed above or
	// to the left of it is at NEGATIVE coordinates.
	RcMonitor Rect
	// RcWork is the part of RcMonitor left for windows once the taskbar and
	// any app bars on this monitor have taken theirs. Windows states it PER
	// MONITOR, where X11's _NET_WORKAREA is one rectangle for the whole
	// desktop.
	RcWork Rect
	// DwFlags carries [MonitorInfoPrimary].
	DwFlags uint32
	// SzDevice is the GDI device name — `\\.\DISPLAY1` — which is the stable
	// way to name this monitor again later. Read it with [MonitorInfoEx.Device].
	SzDevice [cchDeviceName]uint16
}

// MonitorInfoPrimary is MONITORINFOF_PRIMARY: the monitor that owns the
// desktop's origin and carries the taskbar.
const MonitorInfoPrimary = 0x00000001

// NewMonitorInfoEx returns a MONITORINFOEXW with CbSize already set, which is
// the only correct way to hand one to GetMonitorInfoW.
func NewMonitorInfoEx() MonitorInfoEx {
	return MonitorInfoEx{CbSize: uint32(unsafe.Sizeof(MonitorInfoEx{}))}
}

// Device returns the GDI device name, e.g. `\\.\DISPLAY1`.
func (mi MonitorInfoEx) Device() string { return utf16Field(mi.SzDevice[:]) }

// Primary reports whether this is the monitor that owns the desktop's origin.
func (mi MonitorInfoEx) Primary() bool { return mi.DwFlags&MonitorInfoPrimary != 0 }

// DisplayDevice mirrors DISPLAY_DEVICEW (wingdi.h): one entry of the display
// device tree EnumDisplayDevicesW walks.
//
// The tree has two levels and the SAME structure describes both, which is the
// thing to know about it:
//
//   - An ADAPTER (enumerated with a nil device) has DeviceName `\\.\DISPLAY1`
//     and DeviceString set to the graphics adapter's name.
//   - A MONITOR (enumerated with that adapter's name as the device) has
//     DeviceName `\\.\DISPLAY1\Monitor0` and DeviceString set to the panel's
//     name AS ITS DRIVER GIVES IT — which for the generic monitor driver
//     Windows uses on most machines is the literal "Generic PnP Monitor" for
//     every panel attached. A name that cannot tell two monitors apart is not
//     a name, so a caller that needs the panel's own model has to go to its
//     EDID.
//
// Cb must be the structure's own size before the call; use [NewDisplayDevice].
type DisplayDevice struct {
	Cb           uint32
	DeviceName   [cchDeviceName]uint16
	DeviceString [cchDeviceString]uint16
	StateFlags   uint32
	DeviceID     [cchDeviceString]uint16
	DeviceKey    [cchDeviceString]uint16
}

// DISPLAY_DEVICE state flags (wingdi.h).
//
// Bit 0 has TWO documented names because the structure describes two kinds of
// device: it is DISPLAY_DEVICE_ATTACHED_TO_DESKTOP on an adapter and
// DISPLAY_DEVICE_ACTIVE on a monitor. Both mean "this one is part of the
// desktop right now", which is what [DisplayDevice.Active] reports.
const (
	DisplayDeviceActive            = 0x00000001
	DisplayDeviceAttachedToDesktop = 0x00000001
	DisplayDeviceMultiDriver       = 0x00000002
	DisplayDevicePrimaryDevice     = 0x00000004
	DisplayDeviceMirroringDriver   = 0x00000008
	DisplayDeviceVGACompatible     = 0x00000010
	DisplayDeviceRemovable         = 0x00000020
	DisplayDeviceModesPruned       = 0x08000000
)

// EDDGetDeviceInterfaceName is EDD_GET_DEVICE_INTERFACE_NAME. It changes what
// EnumDisplayDevicesW puts in DeviceID: the SetupAPI device INTERFACE path
//
//	\\?\DISPLAY#DELA0FF#5&2a5a2f3&0&UID4353#{e6f07b5f-…}
//
// rather than the driver's hardware id. The interface path is what carries the
// monitor's PnP id and instance, and therefore the only thing that leads back
// to the panel's EDID in the registry — so a caller after the real model name
// wants this flag.
const EDDGetDeviceInterfaceName = 0x00000001

// NewDisplayDevice returns a DISPLAY_DEVICEW with Cb already set, which is the
// only correct way to hand one to EnumDisplayDevicesW.
func NewDisplayDevice() DisplayDevice {
	return DisplayDevice{Cb: uint32(unsafe.Sizeof(DisplayDevice{}))}
}

// Name returns the GDI device name: `\\.\DISPLAY1` for an adapter,
// `\\.\DISPLAY1\Monitor0` for a monitor.
func (d DisplayDevice) Name() string { return utf16Field(d.DeviceName[:]) }

// Description returns the human-readable device string — the adapter's name,
// or the monitor's name as its driver gives it. It is deliberately NOT called
// String: this is a field the OS filled in, not a rendering of the value.
func (d DisplayDevice) Description() string { return utf16Field(d.DeviceString[:]) }

// ID returns the device id: the interface path when the enumeration asked for
// [EDDGetDeviceInterfaceName], the hardware id otherwise.
func (d DisplayDevice) ID() string { return utf16Field(d.DeviceID[:]) }

// Key returns the device's registry key, under HKLM.
func (d DisplayDevice) Key() string { return utf16Field(d.DeviceKey[:]) }

// Active reports whether the device is part of the desktop right now — the
// bit that is DISPLAY_DEVICE_ACTIVE on a monitor and
// DISPLAY_DEVICE_ATTACHED_TO_DESKTOP on an adapter.
func (d DisplayDevice) Active() bool { return d.StateFlags&DisplayDeviceActive != 0 }

// Primary reports whether the device is the primary display device.
func (d DisplayDevice) Primary() bool { return d.StateFlags&DisplayDevicePrimaryDevice != 0 }

// Mirroring reports a pseudo-device that mirrors another display rather than
// being one — a remote-desktop or capture driver. It has no panel and no EDID,
// and enumerating it as a screen would offer the user a display that is not
// there.
func (d DisplayDevice) Mirroring() bool { return d.StateFlags&DisplayDeviceMirroringDriver != 0 }

// GetDpiForMonitor DPI types (shellscalingapi.h). Only [MDTEffectiveDPI] is
// the scale factor the user chose and the one a layout must use;
// [MDTAngularDPI] and [MDTRawDPI] describe the panel itself.
const (
	MDTEffectiveDPI = 0
	MDTAngularDPI   = 1
	MDTRawDPI       = 2
)

// MonitorFrom* fallbacks (winuser.h): what to return when the point or window
// is on no monitor at all.
const (
	MonitorDefaultToNull    = 0x00000000
	MonitorDefaultToPrimary = 0x00000001
	MonitorDefaultToNearest = 0x00000002
)

// DPI awareness context handles (winuser.h). They are NEGATIVE handle-shaped
// constants, hence the bit expressions: written this way they are correct on
// both 64-bit targets.
//
// [DPIAwarenessPerMonitorV2] is the one anything reading monitor geometry
// wants. Without it the OS hands a process VIRTUALISED coordinates — a 3840×2160
// panel at 200% reports itself as 1920×1080 and every rectangle is scaled to
// match — so the numbers look entirely plausible and are not the panel's.
const (
	DPIAwarenessUnaware          uintptr = ^uintptr(0) // -1
	DPIAwarenessSystemAware      uintptr = ^uintptr(1) // -2
	DPIAwarenessPerMonitorAware  uintptr = ^uintptr(2) // -3
	DPIAwarenessPerMonitorV2     uintptr = ^uintptr(3) // -4
	DPIAwarenessUnawareGDIScaled uintptr = ^uintptr(4) // -5
)

// DefaultDPI is USER_DEFAULT_SCREEN_DPI (winuser.h): the reference at which a
// display is at 100% and one logical point is one device pixel. Every DPI the
// OS reports is relative to it, so a scale factor is dpi/DefaultDPI.
const DefaultDPI = 96

// utf16Field decodes one of the API's FIXED-LENGTH UTF-16 fields.
//
// It stops at the first NUL, and takes the whole field when there is none: a
// value that exactly fills its array is not terminated, and decoding the array
// wholesale would append the padding of the next call's leftovers. This is why
// the accessors above exist rather than every caller reaching for
// windows.UTF16ToString, which requires a terminator.
func utf16Field(b []uint16) string {
	for i, c := range b {
		if c == 0 {
			return string(utf16.Decode(b[:i]))
		}
	}
	return string(utf16.Decode(b))
}
