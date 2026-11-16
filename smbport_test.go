package win32

import (
	"strings"
	"testing"
)

func TestSMBMappingValidate(t *testing.T) {
	ok := SMBMapping{Remote: `\\127.0.0.1\theca`, Local: "Z:", User: `.\alice`, Password: "p", TCPPort: 4455}
	if err := ok.validate(); err != nil {
		t.Fatal(err)
	}
	for name, m := range map[string]SMBMapping{
		"not unc":   {Remote: "host/share"},
		"too short": {Remote: `\\a`},
		"bad local": {Remote: `\\h\s`, Local: "ZZ"},
		"nul":       {Remote: `\\h\s`, Password: "a\x00b"},
	} {
		if err := m.validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSMBPortError(t *testing.T) {
	full := (&SMBPortError{HRESULT: 0x80041001, Code: 53, Message: "The network path was not found."}).Error()
	for _, want := range []string{"0x80041001", "Win32 53", "network path"} {
		if !strings.Contains(full, want) {
			t.Errorf("%q lacks %q", full, want)
		}
	}
	bare := (&SMBPortError{HRESULT: 1}).Error()
	if strings.Contains(bare, "Win32") || strings.HasSuffix(bare, ": ") {
		t.Errorf("bare error %q prints empty fields", bare)
	}
}

func TestParseSystemErrorID(t *testing.T) {
	for in, want := range map[string]uint32{
		"Windows System Error 53":    53,
		" Windows System Error 1312": 1312,
	} {
		if n, ok := parseSystemErrorID(in); !ok || n != want {
			t.Errorf("parseSystemErrorID(%q) = %d, %v", in, n, ok)
		}
	}
	for _, in := range []string{"", "Windows System Error ", "Windows System Error x", "Error 53", "Windows System Error 99999999999"} {
		if _, ok := parseSystemErrorID(in); ok {
			t.Errorf("parseSystemErrorID(%q) accepted", in)
		}
	}
}
