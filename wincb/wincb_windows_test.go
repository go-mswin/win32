// Copyright (c) the go-mswin authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build windows

package wincb

import (
	"syscall"
	"testing"
)

// On every architecture a New callback is callable through the C convention
// and returns what the Go function returns.
func TestNewCallbackRoundTrips(t *testing.T) {
	cb := New(func(a, b uintptr) uintptr { return a*10 + b })
	if cb == 0 {
		t.Fatal("New returned 0")
	}
	if r, _, _ := syscall.SyscallN(cb, 4, 2); r != 42 {
		t.Fatalf("callback returned %d, want 42", r)
	}
}
