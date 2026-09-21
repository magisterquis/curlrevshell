package tlog

/*
 * stringsbuffer_test.go
 * Buffer which buffers strings
 * By J. Stuart McMurray
 * Created 20260214
 * Last Modified 20260215
 */

import (
	"fmt"
	"slices"
	"testing"
)

// Can we buffer strings?
func TestStringsBufferPrintf(t *testing.T) {
	var (
		sb    stringsBuffer
		haves = []struct {
			fmt  string
			args []any
		}{{
			fmt: "simple string",
		}, {
			fmt:  "string with args: %s %d %t",
			args: []any{"kittens", 123, true},
		}}
		wants = make([]string, len(haves))
	)

	/* Make sure we didn't start with a buffer. */
	if 0 != len(sb.buffer) {
		t.Fatalf("Buffer not initially empty")
	}

	/* Buffer some messages. */
	for i, have := range haves {
		sb.Printf(have.fmt, have.args...)
		wants[i] = fmt.Sprintf(have.fmt, have.args...)
	}

	/* Did it work? */
	if got := sb.buffer; !slices.Equal(got, wants) {
		t.Errorf(
			"Buffer incorrect\n"+
				"have: %+v\n"+
				" got: %s\n"+
				"want: %s",
			haves,
			got,
			wants,
		)
	}
}

// Can we retrieve the buffer?
func TestStringsBufferGet(t *testing.T) {
	var sb stringsBuffer

	/* Should be initially empty. */
	if got := sb.Get(); nil == got {
		t.Errorf("Initial get returned nil")
	} else if 0 != len(got) {
		t.Errorf("Initial got buffer not empty: %q", got)
	}

	/* Add some lines, make sure we can get them. */
	want := []string{"kittens", "moose"}
	for _, v := range want {
		sb.Printf("%s", v)
	}
	if got := sb.Get(); !slices.Equal(got, want) {
		t.Errorf(
			"Got incorrect buffered strings\n"+
				" got: %s\n"+
				"want: %s",
			got,
			want,
		)
	}

	/* Should be empty again. */
	if got := sb.Get(); nil == got {
		t.Errorf("Get after get returned nil")
	} else if 0 != len(got) {
		t.Errorf("Get after get buffer not empty: %q", got)
	}
}
