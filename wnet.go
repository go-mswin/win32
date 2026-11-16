package win32

// The OS-independent half of the Windows Networking (WNet) connection API:
// the NETRESOURCEW layout, the connection flags, the error type, and the
// argument preparation that turns Go strings into the UTF-16 buffers
// mpr.dll wants. The calls themselves are in wnet_windows.go.
//
// Sources, as read on learn.microsoft.com:
//   - WNetAddConnection2W (winnetwk.h, Mpr.dll):
//     https://learn.microsoft.com/en-us/windows/win32/api/winnetwk/nf-winnetwk-wnetaddconnection2w
//   - NETRESOURCEW:
//     https://learn.microsoft.com/en-us/windows/win32/api/winnetwk/ns-winnetwk-netresourcew
//   - alternative SMB ports (Windows 11 24H2 / Server 2025), which are exposed
//     by New-SmbMapping -TcpPort and `net use /TCPPORT`, NOT by a WNet flag:
//     https://learn.microsoft.com/en-us/windows-server/storage/file-server/smb-ports
//   - WebClient BasicAuthLevel (0 off, 1 SSL only = default, 2 SSL and plain):
//     https://learn.microsoft.com/en-us/archive/blogs/robert_mcmurray/webdav-redirector-registry-settings

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"
)

// ConnectFlags is the dwFlags argument of WNetAddConnection2W.
type ConnectFlags uint32

// Connection options from winnetwk.h, with the values the documentation gives.
const (
	ConnectUpdateProfile ConnectFlags = 0x00000001 // remember the connection across logons
	ConnectUpdateRecent  ConnectFlags = 0x00000002
	ConnectTemporary     ConnectFlags = 0x00000004 // do not remember it
	ConnectInteractive   ConnectFlags = 0x00000008 // the OS may prompt
	ConnectPrompt        ConnectFlags = 0x00000010
	ConnectRedirect      ConnectFlags = 0x00000080
	ConnectCurrentMedia  ConnectFlags = 0x00000200
	ConnectCommandLine   ConnectFlags = 0x00000800
	ConnectCmdSaveCred   ConnectFlags = 0x00001000
	ConnectCredReset     ConnectFlags = 0x00002000
)

// NETRESOURCEW dwType values.
const (
	ResourceTypeAny   uint32 = 0
	ResourceTypeDisk  uint32 = 1
	ResourceTypePrint uint32 = 2
)

// Return codes of the WNet functions that callers commonly test for.
const (
	ErrorBadNetName      syscall.Errno = 67   // ERROR_BAD_NET_NAME
	ErrorAlreadyAssigned syscall.Errno = 85   // ERROR_ALREADY_ASSIGNED
	ErrorBadDevice       syscall.Errno = 1200 // ERROR_BAD_DEVICE
	ErrorNoNetOrBadPath  syscall.Errno = 1203 // ERROR_NO_NET_OR_BAD_PATH
	ErrorNoNetwork       syscall.Errno = 1222 // ERROR_NO_NETWORK
	ErrorNotConnected    syscall.Errno = 2250 // ERROR_NOT_CONNECTED
	ErrorLogonFailure    syscall.Errno = 1326 // ERROR_LOGON_FAILURE
)

// NetResource is NETRESOURCEW: eight 32-bit or pointer-sized fields, 48 bytes
// on a 64-bit Windows (amd64, arm64).
type NetResource struct {
	Scope       uint32
	Type        uint32
	DisplayType uint32
	Usage       uint32
	LocalName   *uint16
	RemoteName  *uint16
	Comment     *uint16
	Provider    *uint16
}

// WNetError is a WNet failure: the function and the Win32 code it returned.
// It unwraps to the syscall.Errno, so errors.Is(err, win32.ErrorLogonFailure)
// works.
type WNetError struct {
	Op   string
	Code uint32
}

func (e *WNetError) Error() string {
	return fmt.Sprintf("win32: %s: %s (code %d)", e.Op, syscall.Errno(e.Code).Error(), e.Code)
}

// Unwrap returns the Win32 code as a syscall.Errno.
func (e *WNetError) Unwrap() error { return syscall.Errno(e.Code) }

// connArgs holds the UTF-16 buffers of one WNetAddConnection2W call. The
// buffers are kept as slices so that the password can be wiped once the call
// returns.
type connArgs struct {
	res      NetResource
	user     []uint16 // nil means "default user"
	password []uint16 // nil means "default password"; empty string means none
	local    []uint16
	remote   []uint16
}

// utf16z encodes s as NUL-terminated UTF-16, refusing an embedded NUL, which
// would silently truncate the string on the C side.
func utf16z(s string) ([]uint16, error) {
	if strings.IndexByte(s, 0) >= 0 {
		return nil, fmt.Errorf("win32: string contains NUL")
	}
	return append(utf16.Encode([]rune(s)), 0), nil
}

// ValidLocalName reports whether name is empty (a deviceless connection) or a
// drive letter such as "X:".
func ValidLocalName(name string) bool {
	if name == "" {
		return true
	}
	if len(name) != 2 || name[1] != ':' {
		return false
	}
	c := name[0] | 0x20
	return c >= 'a' && c <= 'z'
}

// prepareConnection validates the arguments and builds the NETRESOURCEW and
// the UTF-16 buffers. user == "" selects the default user and password ==
// "" with a user set sends an empty password.
func prepareConnection(remote, local, user, password string) (*connArgs, error) {
	if remote == "" {
		return nil, fmt.Errorf("win32: AddConnection: empty remote name")
	}
	if !strings.HasPrefix(remote, `\\`) {
		return nil, fmt.Errorf("win32: AddConnection: remote name %q is not a UNC path", remote)
	}
	if !ValidLocalName(local) {
		return nil, fmt.Errorf("win32: AddConnection: local name %q is not empty or a drive letter", local)
	}
	a := &connArgs{}
	var err error
	if a.remote, err = utf16z(remote); err != nil {
		return nil, err
	}
	a.res = NetResource{Type: ResourceTypeDisk, RemoteName: &a.remote[0]}
	if local != "" {
		// ValidLocalName has already limited local to "X:", so it cannot
		// contain a NUL and needs no second check.
		a.local = append(utf16.Encode([]rune(local)), 0)
		a.res.LocalName = &a.local[0]
	}
	if user != "" {
		if a.user, err = utf16z(user); err != nil {
			return nil, err
		}
		if a.password, err = utf16z(password); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// userPtr and passwordPtr return nil for "use the default".
func (a *connArgs) userPtr() *uint16 {
	if a.user == nil {
		return nil
	}
	return &a.user[0]
}

func (a *connArgs) passwordPtr() *uint16 {
	if a.password == nil {
		return nil
	}
	return &a.password[0]
}

// wipe zeroes the password buffer.
func (a *connArgs) wipe() {
	clear(a.password)
}

// SMBRemote builds \\host\share for a Microsoft network provider. host may be
// an IPv4 literal or a name. An IPv6 literal is rewritten to the
// .ipv6-literal.net form the documentation requires.
func SMBRemote(host, share string) (string, error) {
	if host == "" || share == "" {
		return "", fmt.Errorf("win32: SMBRemote: empty host or share")
	}
	if strings.ContainsAny(host, `\/@`) || strings.ContainsAny(share, `\/`) {
		return "", fmt.Errorf("win32: SMBRemote: separator in host %q or share %q", host, share)
	}
	if strings.Contains(host, ":") {
		host = strings.ReplaceAll(host, ":", "-") + ".ipv6-literal.net"
	}
	return `\\` + host + `\` + share, nil
}

// DAVRemote builds the UNC form the WebClient redirector understands:
// \\host@port\DavWWWRoot[\path] for HTTP and \\host@SSL@port\DavWWWRoot[\path]
// for HTTPS. path uses forward slashes or backslashes and may be empty.
func DAVRemote(host string, port int, useTLS bool, path string) (string, error) {
	if host == "" {
		return "", fmt.Errorf("win32: DAVRemote: empty host")
	}
	if strings.ContainsAny(host, `\/@`) || strings.Contains(host, ":") {
		return "", fmt.Errorf("win32: DAVRemote: bad host %q", host)
	}
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("win32: DAVRemote: port %d out of range", port)
	}
	s := `\\` + host
	if useTLS {
		s += "@SSL"
	}
	s += "@" + strconv.Itoa(port) + `\DavWWWRoot`
	p := strings.Trim(strings.ReplaceAll(path, "/", `\`), `\`)
	if p != "" {
		s += `\` + p
	}
	return s, nil
}
