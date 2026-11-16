package win32

// The OS-independent half of the Windows 11 24H2 alternative-SMB-port mapping.
//
// Documented reach (learn.microsoft.com/en-us/windows-server/storage/file-server/smb-ports):
// New-SmbMapping -TcpPort and `net use /TCPPORT`. Neither is a WNet flag and
// NETRESOURCEW / USE_INFO_2 have no port field, so the only API an application
// can call is the CIM method MSFT_SmbMapping.Create in
// root\Microsoft\Windows\SMB, which is what New-SmbMapping wraps. It is
// reached from smbport_windows.go through the WMI COM interfaces.

import (
	"fmt"
	"strconv"
	"strings"
)

// SMBMapping describes one mapping through MSFT_SmbMapping.Create.
type SMBMapping struct {
	Remote   string // \\host\share
	Local    string // "X:" or "" for no drive letter
	User     string // "" for the default user; use ".\name" for a local account
	Password string
	TCPPort  uint16 // 0 means the default, 445
}

// validate applies the rules the CIM method would otherwise report as an
// opaque failure.
func (m SMBMapping) validate() error {
	if !strings.HasPrefix(m.Remote, `\\`) || len(m.Remote) < 5 {
		return fmt.Errorf("win32: MapSMB: remote %q is not a UNC path", m.Remote)
	}
	if !ValidLocalName(m.Local) {
		return fmt.Errorf("win32: MapSMB: local %q is not empty or a drive letter", m.Local)
	}
	if strings.IndexByte(m.Remote+m.User+m.Password, 0) >= 0 {
		return fmt.Errorf("win32: MapSMB: string contains NUL")
	}
	return nil
}

// SMBPortError is a failure of MSFT_SmbMapping.Create. HRESULT is the WMI
// result (WBEM_E_FAILED = 0x80041001 when the provider reports a Win32 error),
// Code is the Win32 error the provider reported when it could be read (0 if
// not), and Message is the provider's text.
type SMBPortError struct {
	HRESULT uint32
	Code    uint32
	Message string
}

func (e *SMBPortError) Error() string {
	s := fmt.Sprintf("win32: MSFT_SmbMapping.Create: HRESULT 0x%08X", e.HRESULT)
	if e.Code != 0 {
		s += " Win32 " + strconv.FormatUint(uint64(e.Code), 10)
	}
	if e.Message != "" {
		s += ": " + e.Message
	}
	return s
}

// parseSystemErrorID extracts N from the provider's "Windows System Error N".
func parseSystemErrorID(s string) (uint32, bool) {
	const prefix = "Windows System Error "
	rest, ok := strings.CutPrefix(strings.TrimSpace(s), prefix)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(strings.TrimSpace(rest), 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}
