//go:build windows

package win32

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Mpr is mpr.dll, the multiple-provider router that owns the WNet functions.
var Mpr = windows.NewLazySystemDLL("mpr.dll")

var (
	procWNetAddConnection2W    = Mpr.NewProc("WNetAddConnection2W")
	procWNetCancelConnection2W = Mpr.NewProc("WNetCancelConnection2W")
)

// AddConnection makes a connection to the network resource remote, optionally
// redirecting the drive letter localName ("X:"; "" for a deviceless
// connection), with the given credentials, through WNetAddConnection2W.
//
// The credentials travel as arguments of the API call, never on a command
// line, and the password buffer is zeroed when the call returns. user == ""
// uses the process's default credentials. flags == 0 is a temporary
// connection that is not restored at the next logon; ConnectUpdateProfile
// asks for the opposite.
//
// A failure is a *WNetError carrying the code mpr.dll returned.
func AddConnection(remote, localName, user, password string, flags ConnectFlags) error {
	a, err := prepareConnection(remote, localName, user, password)
	if err != nil {
		return err
	}
	defer a.wipe()
	r, _, _ := procWNetAddConnection2W.Call(
		uintptr(unsafe.Pointer(&a.res)),
		uintptr(unsafe.Pointer(a.passwordPtr())),
		uintptr(unsafe.Pointer(a.userPtr())),
		uintptr(flags),
	)
	if r != 0 {
		return &WNetError{Op: "WNetAddConnection2W", Code: uint32(r)}
	}
	return nil
}

// CancelConnection removes the connection to name, which is a drive letter
// ("X:") or a remote name. With force, open files on the connection are closed.
// The connection is not removed from the persistent profile unless the OS does
// that itself; pass flags through CancelConnectionFlags to control it.
func CancelConnection(name string, force bool) error {
	return CancelConnectionFlags(name, 0, force)
}

// CancelConnectionFlags is CancelConnection with the dwFlags argument:
// ConnectUpdateProfile also forgets a remembered connection.
func CancelConnectionFlags(name string, flags ConnectFlags, force bool) error {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	var f uintptr
	if force {
		f = 1
	}
	r, _, _ := procWNetCancelConnection2W.Call(uintptr(unsafe.Pointer(p)), uintptr(flags), f)
	if r != 0 {
		return &WNetError{Op: "WNetCancelConnection2W", Code: uint32(r)}
	}
	return nil
}

// IsNotConnected reports whether err is the "no such connection" failure of
// CancelConnection.
func IsNotConnected(err error) bool {
	var e *WNetError
	return errors.As(err, &e) && e.Code == uint32(ErrorNotConnected)
}
