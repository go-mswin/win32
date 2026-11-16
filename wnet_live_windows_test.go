//go:build windows

package win32_test

import (
	"errors"
	"testing"

	win32 "github.com/go-mswin/win32"
)

// These run on every Windows lane: they need no server, only the router's
// answers to calls that cannot succeed.

func TestAddConnectionRefusesBeforeCalling(t *testing.T) {
	if err := win32.AddConnection("", "", "", "", 0); err == nil {
		t.Fatal("empty remote accepted")
	}
	if err := win32.AddConnection(`\\h\s`, "Q", "", "", 0); err == nil {
		t.Fatal("bad local accepted")
	}
}

func TestAddConnectionReportsTheRouterCode(t *testing.T) {
	// A name no provider can resolve. 53 (path not found), 67 (name not found),
	// 1203 and 1222 are all router answers; what matters is a *WNetError.
	err := win32.AddConnection(`\\no-such-host.invalid\share`, "", "u", "p", 0)
	var e *win32.WNetError
	if !errors.As(err, &e) {
		t.Fatalf("err = %v, want *WNetError", err)
	}
	if e.Op != "WNetAddConnection2W" || e.Code == 0 {
		t.Fatalf("unexpected error %+v", e)
	}
}

func TestCancelConnectionNotConnected(t *testing.T) {
	err := win32.CancelConnection("Q:", false)
	if !win32.IsNotConnected(err) {
		t.Fatalf("CancelConnection(Q:) = %v, want ERROR_NOT_CONNECTED", err)
	}
	if err := win32.CancelConnection("a\x00b", false); err == nil || win32.IsNotConnected(err) {
		t.Fatalf("NUL in name: %v", err)
	}
	if win32.IsNotConnected(nil) {
		t.Fatal("IsNotConnected(nil)")
	}
}

func TestMapSMBRefusesBadInput(t *testing.T) {
	if err := win32.MapSMB(win32.SMBMapping{Remote: "x"}); err == nil {
		t.Fatal("bad remote accepted")
	}
}
