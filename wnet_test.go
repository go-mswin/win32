package win32

import (
	"errors"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func TestNetResourceLayout(t *testing.T) {
	// NETRESOURCEW on 64-bit Windows: 4 DWORDs + 4 pointers.
	if got := unsafe.Sizeof(NetResource{}); got != 48 {
		t.Fatalf("sizeof(NetResource) = %d, want 48", got)
	}
	if got := unsafe.Offsetof(NetResource{}.RemoteName); got != 24 {
		t.Fatalf("offsetof(RemoteName) = %d, want 24", got)
	}
}

func TestValidLocalName(t *testing.T) {
	for _, ok := range []string{"", "X:", "z:", "A:"} {
		if !ValidLocalName(ok) {
			t.Errorf("ValidLocalName(%q) = false", ok)
		}
	}
	for _, bad := range []string{"X", "XY:", "1:", "X;", `\\h\s`, "@:"} {
		if ValidLocalName(bad) {
			t.Errorf("ValidLocalName(%q) = true", bad)
		}
	}
}

func TestPrepareConnection(t *testing.T) {
	a, err := prepareConnection(`\\127.0.0.1\share`, "X:", "alice", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	if a.res.Type != ResourceTypeDisk || a.res.RemoteName == nil || a.res.LocalName == nil {
		t.Fatalf("resource not filled: %+v", a.res)
	}
	if a.userPtr() == nil || a.passwordPtr() == nil {
		t.Fatal("credentials missing")
	}
	if len(a.password) != len("s3cret")+1 || a.password[len(a.password)-1] != 0 {
		t.Fatalf("password not NUL terminated: %v", a.password)
	}
	a.wipe()
	for _, c := range a.password {
		if c != 0 {
			t.Fatalf("password not wiped: %v", a.password)
		}
	}

	// Default credentials and a deviceless connection pass NULL pointers.
	d, err := prepareConnection(`\\h\s`, "", "", "ignored")
	if err != nil {
		t.Fatal(err)
	}
	if d.userPtr() != nil || d.passwordPtr() != nil || d.res.LocalName != nil {
		t.Fatalf("expected NULL user/password/local, got %+v", d)
	}
	// A user with an empty password sends an empty string, not NULL.
	e, err := prepareConnection(`\\h\s`, "", "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if e.passwordPtr() == nil || len(e.password) != 1 {
		t.Fatalf("empty password should be a lone NUL, got %v", e.password)
	}
}

func TestPrepareConnectionRefusals(t *testing.T) {
	cases := []struct {
		name                  string
		remote, local, u, pwd string
		want                  string
	}{
		{"empty remote", "", "", "", "", "empty remote"},
		{"not unc", "host/share", "", "", "", "not a UNC"},
		{"bad local", `\\h\s`, "Q", "", "", "local name"},
		{"nul remote", "\\\\h\\s\x00", "", "", "", "NUL"},
		{"nul user", `\\h\s`, "", "a\x00", "p", "NUL"},
		{"nul password", `\\h\s`, "", "a", "p\x00", "NUL"},
	}
	for _, c := range cases {
		_, err := prepareConnection(c.remote, c.local, c.u, c.pwd)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}
}

func TestSMBRemote(t *testing.T) {
	good := map[[2]string]string{
		{"127.0.0.1", "theca"}:   `\\127.0.0.1\theca`,
		{"nas", "x"}:             `\\nas\x`,
		{"fe80::1", "s"}:         `\\fe80--1.ipv6-literal.net\s`,
		{"2001:4898::2449", "s"}: `\\2001-4898--2449.ipv6-literal.net\s`,
	}
	for in, want := range good {
		got, err := SMBRemote(in[0], in[1])
		if err != nil || got != want {
			t.Errorf("SMBRemote%v = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range [][2]string{{"", "s"}, {"h", ""}, {`a\b`, "s"}, {"h@1", "s"}, {"h", `a\b`}, {"h", "a/b"}} {
		if _, err := SMBRemote(in[0], in[1]); err == nil {
			t.Errorf("SMBRemote%v accepted", in)
		}
	}
}

func TestDAVRemote(t *testing.T) {
	type c struct {
		host string
		port int
		tls  bool
		path string
		want string
	}
	for _, x := range []c{
		{"127.0.0.1", 8080, false, "", `\\127.0.0.1@8080\DavWWWRoot`},
		{"localhost", 8443, true, "", `\\localhost@SSL@8443\DavWWWRoot`},
		{"h", 1, false, "/a/b/", `\\h@1\DavWWWRoot\a\b`},
		{"h", 65535, true, `\x`, `\\h@SSL@65535\DavWWWRoot\x`},
	} {
		got, err := DAVRemote(x.host, x.port, x.tls, x.path)
		if err != nil || got != x.want {
			t.Errorf("DAVRemote(%+v) = %q, %v", x, got, err)
		}
	}
	for _, x := range []c{
		{"", 80, false, "", ""},
		{`a\b`, 80, false, "", ""},
		{"a/b", 80, false, "", ""},
		{"a@b", 80, false, "", ""},
		{"::1", 80, false, "", ""},
		{"h", 0, false, "", ""},
		{"h", 65536, false, "", ""},
	} {
		if _, err := DAVRemote(x.host, x.port, x.tls, x.path); err == nil {
			t.Errorf("DAVRemote(%+v) accepted", x)
		}
	}
}

func TestWNetError(t *testing.T) {
	var err error = &WNetError{Op: "WNetAddConnection2W", Code: uint32(ErrorLogonFailure)}
	if !errors.Is(err, ErrorLogonFailure) {
		t.Fatal("errors.Is(logon failure) = false")
	}
	if errors.Is(err, ErrorBadNetName) {
		t.Fatal("matched the wrong code")
	}
	msg := err.Error()
	if !strings.Contains(msg, "WNetAddConnection2W") || !strings.Contains(msg, "1326") {
		t.Fatalf("message %q lacks op or code", msg)
	}
	if errors.Unwrap(err) != syscall.Errno(1326) {
		t.Fatal("Unwrap is not the Errno")
	}
}
