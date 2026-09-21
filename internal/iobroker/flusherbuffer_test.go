package iobroker

/*
 * handle_input_test.go
 * Tests for handle_input.go
 * By J. Stuart McMurray
 * Created 20260802
 * Last Modified 20260814
 */

import (
	"bytes"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/tlog"
)

// type testFlusherBuffer is a wrapper around a bytes.Buffer with a flush
// method.
type testFlusherBuffer struct {
	*bytes.Buffer
	flushed bool
}

// Flush sets fb.flushed to true and returns nil.
func (fb *testFlusherBuffer) Flush() error { fb.flushed = true; return nil }

// Does a testFlusherBuffer do what we expect?
func TestTestFlusherBuffer(t *testing.T) {
	var (
		fb  = &testFlusherBuffer{Buffer: new(bytes.Buffer)}
		msg = tlog.S("msg")
	)
	fb.WriteString(msg)
	if err := fb.Flush(); nil != err {
		t.Errorf("Flush returned error: %v", err)
	}

	if got, want := fb.String(), msg; got != want {
		t.Errorf(
			"Buffered string incorrect\n got: %q\nwant: %q",
			got,
			want,
		)
	}

	if !fb.flushed {
		t.Errorf("Flush did not set flushed")
	}
}
