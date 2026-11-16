//go:build windows

package win32

// MSFT_SmbMapping.Create through the WMI COM interfaces, with no cgo and no
// child process: the credentials go into an in-memory CIM object, never onto a
// command line. The vtable slots are those of wbemcli.h (IWbemLocator,
// IWbemServices, IWbemClassObject).

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	// Ole32 and Oleaut32 are bound here for the COM calls; exposed like the
	// other shared DLL handles.
	Ole32    = windows.NewLazySystemDLL("ole32.dll")
	Oleaut32 = windows.NewLazySystemDLL("oleaut32.dll")

	procCoCreateInstance  = Ole32.NewProc("CoCreateInstance")
	procCoSetProxyBlanket = Ole32.NewProc("CoSetProxyBlanket")
	procSysAllocStringLen = Oleaut32.NewProc("SysAllocStringLen")
	procSysFreeString     = Oleaut32.NewProc("SysFreeString")
	procSysStringLen      = Oleaut32.NewProc("SysStringLen")
	procGetErrorInfo      = Oleaut32.NewProc("GetErrorInfo")
)

var (
	clsidWbemLocator   = windows.GUID{Data1: 0x4590f811, Data2: 0x1d3a, Data3: 0x11d0, Data4: [8]byte{0x89, 0x1f, 0x00, 0xaa, 0x00, 0x4b, 0x2e, 0x24}}
	iidWbemLocator     = windows.GUID{Data1: 0xdc12a687, Data2: 0x737f, Data3: 0x11cf, Data4: [8]byte{0x88, 0x4d, 0x00, 0xaa, 0x00, 0x4b, 0x2e, 0x24}}
	iidWbemClassObject = windows.GUID{Data1: 0xdc12a681, Data2: 0x737f, Data3: 0x11cf, Data4: [8]byte{0x88, 0x4d, 0x00, 0xaa, 0x00, 0x4b, 0x2e, 0x24}}
)

const (
	vtI4   = 3
	vtBSTR = 8
	vtBool = 11

	hrChangedMode = 0x80010106 // RPC_E_CHANGED_MODE
	hrSFalse      = 1          // S_FALSE: COM was already initialised on this thread
)

// variant is a 64-bit VARIANT: a type tag and an 8-byte payload, padded to the
// 24 bytes the union reaches because of VT_RECORD.
type variant struct {
	vt  uint16
	_   [3]uint16
	val uintptr
	_   uintptr
}

// com is a COM interface pointer.
type com struct{ p unsafe.Pointer }

// call invokes vtable slot idx with the interface pointer as first argument
// and returns the raw HRESULT.
func (o com) call(idx int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(o.p)
	fn := *(*uintptr)(unsafe.Add(vtbl, idx*int(unsafe.Sizeof(uintptr(0)))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(o.p)}, args...)...)
	return r
}

func (o com) release() {
	if o.p != nil {
		o.call(2)
	}
}

func failed(hr uintptr) bool { return int32(hr) < 0 }

// allocBSTR returns a BSTR for s.
func allocBSTR(s string) uintptr {
	u := utf16.Encode([]rune(s))
	var first *uint16
	if len(u) > 0 {
		first = &u[0]
	}
	r, _, _ := procSysAllocStringLen.Call(uintptr(unsafe.Pointer(first)), uintptr(len(u)))
	return r
}

// freeBSTR frees b, overwriting its characters first when wipe is set.
func freeBSTR(b uintptr, wipe bool) {
	if b == 0 {
		return
	}
	if wipe {
		n, _, _ := procSysStringLen.Call(b)
		if n > 0 {
			zeros := make([]uint16, n)
			procRtlMoveMemory.Call(b, uintptr(unsafe.Pointer(&zeros[0])), n*2)
		}
	}
	procSysFreeString.Call(b)
}

// bstrString copies a BSTR out of OS memory (RtlMoveMemory, as readRect does,
// so no uintptr ever becomes a Go pointer).
func bstrString(b uintptr) string {
	if b == 0 {
		return ""
	}
	n, _, _ := procSysStringLen.Call(b)
	if n == 0 {
		return ""
	}
	u := make([]uint16, n)
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&u[0])), b, n*2)
	return string(utf16.Decode(u))
}

// put sets a property of a CIM object.
func (o com) put(name string, v *variant) error {
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	if hr := o.call(5, uintptr(unsafe.Pointer(n)), 0, uintptr(unsafe.Pointer(v)), 0); failed(hr) {
		return fmt.Errorf("win32: IWbemClassObject::Put(%s): HRESULT 0x%08X", name, uint32(hr))
	}
	return nil
}

func (o com) putString(name, s string, wipe bool) error {
	b := allocBSTR(s)
	defer freeBSTR(b, wipe)
	return o.put(name, &variant{vt: vtBSTR, val: b})
}

func (o com) putBool(name string, b bool) error {
	v := variant{vt: vtBool}
	if b {
		v.val = 0xFFFF
	}
	return o.put(name, &v)
}

// get reads a property into v.
func (o com) get(name string, v *variant) bool {
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return false
	}
	return !failed(o.call(4, uintptr(unsafe.Pointer(n)), 0, uintptr(unsafe.Pointer(v)), 0, 0))
}

// wbemError builds the error for a failed ExecMethod from the thread's
// IErrorInfo, which for MSFT_SmbMapping is an MSFT_WmiError carrying
// "Windows System Error N" in MessageId and the text in Message.
func wbemError(hr uintptr) error {
	e := &SMBPortError{HRESULT: uint32(hr)}
	var ei unsafe.Pointer
	procGetErrorInfo.Call(0, uintptr(unsafe.Pointer(&ei)))
	if ei == nil {
		return e
	}
	info := com{ei}
	defer info.release()
	var obj unsafe.Pointer
	if failed(info.call(0, uintptr(unsafe.Pointer(&iidWbemClassObject)), uintptr(unsafe.Pointer(&obj)))) {
		return e
	}
	wo := com{obj}
	defer wo.release()
	var v variant
	if wo.get("Message", &v) && v.vt == vtBSTR {
		e.Message = bstrString(v.val)
		freeBSTR(v.val, false)
	}
	v = variant{}
	if wo.get("MessageId", &v) && v.vt == vtBSTR {
		e.Code, _ = parseSystemErrorID(bstrString(v.val))
		freeBSTR(v.val, false)
	}
	return e
}

// MapSMB maps m.Remote through MSFT_SmbMapping.Create, the method behind
// New-SmbMapping, so that m.TCPPort selects an alternative TCP port
// (Windows 11 24H2 / Server 2025 and later, administrator rights as the
// documentation states). The user must carry a domain part (".\name") for a
// local account: a bare name makes the client's own NTLM step fail with
// STATUS_NO_SUCH_LOGON_SESSION (0xC000005F) before anything is sent.
//
// On success the connection is a normal SMB connection; a port-less
// \\host\share UNC then reuses it, and CancelConnection removes the drive.
func MapSMB(m SMBMapping) (err error) {
	if err = m.validate(); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if e := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); e == nil || errors.Is(e, syscall.Errno(hrSFalse)) {
		defer windows.CoUninitialize()
	} else if !errors.Is(e, syscall.Errno(hrChangedMode)) {
		return fmt.Errorf("win32: CoInitializeEx: %w", e)
	}

	var locp unsafe.Pointer
	if hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidWbemLocator)), 0, 1, // CLSCTX_INPROC_SERVER
		uintptr(unsafe.Pointer(&iidWbemLocator)), uintptr(unsafe.Pointer(&locp))); failed(hr) {
		return fmt.Errorf("win32: CoCreateInstance(WbemLocator): HRESULT 0x%08X", uint32(hr))
	}
	loc := com{locp}
	defer loc.release()

	ns := allocBSTR(`ROOT\Microsoft\Windows\SMB`)
	var svcp unsafe.Pointer
	hr := loc.call(3, ns, 0, 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&svcp)))
	freeBSTR(ns, false)
	if failed(hr) {
		return fmt.Errorf("win32: IWbemLocator::ConnectServer: HRESULT 0x%08X", uint32(hr))
	}
	svc := com{svcp}
	defer svc.release()

	if hr, _, _ := procCoSetProxyBlanket.Call(uintptr(svcp), 10, 0, ^uintptr(0), 3, 3, 0, 0); failed(hr) {
		return fmt.Errorf("win32: CoSetProxyBlanket: HRESULT 0x%08X", uint32(hr))
	}

	class := allocBSTR("MSFT_SmbMapping")
	method := allocBSTR("Create")
	defer freeBSTR(class, false)
	defer freeBSTR(method, false)

	var clsp unsafe.Pointer
	if hr := svc.call(6, class, 0, 0, uintptr(unsafe.Pointer(&clsp)), 0); failed(hr) {
		return fmt.Errorf("win32: IWbemServices::GetObject(MSFT_SmbMapping): HRESULT 0x%08X", uint32(hr))
	}
	cls := com{clsp}
	defer cls.release()

	var sigp unsafe.Pointer
	mn, _ := windows.UTF16PtrFromString("Create")
	if hr := cls.call(19, uintptr(unsafe.Pointer(mn)), 0, uintptr(unsafe.Pointer(&sigp)), 0); failed(hr) {
		return fmt.Errorf("win32: IWbemClassObject::GetMethod(Create): HRESULT 0x%08X", uint32(hr))
	}
	sig := com{sigp}
	defer sig.release()

	var inp unsafe.Pointer
	if hr := sig.call(15, 0, uintptr(unsafe.Pointer(&inp))); failed(hr) {
		return fmt.Errorf("win32: IWbemClassObject::SpawnInstance: HRESULT 0x%08X", uint32(hr))
	}
	in := com{inp}
	defer in.release()

	if err = in.putString("RemotePath", m.Remote, false); err != nil {
		return err
	}
	if m.Local != "" {
		if err = in.putString("LocalPath", m.Local, false); err != nil {
			return err
		}
	}
	if m.User != "" {
		if err = in.putString("UserName", m.User, false); err != nil {
			return err
		}
		if err = in.putString("Password", m.Password, true); err != nil {
			return err
		}
	}
	if err = in.putBool("Persistent", false); err != nil {
		return err
	}
	if m.TCPPort != 0 {
		if err = in.put("TcpPort", &variant{vt: vtI4, val: uintptr(m.TCPPort)}); err != nil {
			return err
		}
	}

	var outp unsafe.Pointer
	if hr := svc.call(24, class, method, 0, 0, uintptr(inp), uintptr(unsafe.Pointer(&outp)), 0); failed(hr) {
		return wbemError(hr)
	}
	out := com{outp}
	defer out.release()
	var rv variant
	if out.get("ReturnValue", &rv) && uint32(rv.val) != 0 {
		return &SMBPortError{Code: uint32(rv.val), Message: "ReturnValue"}
	}
	return nil
}
