package opshell

/*
 * chanwriter_test.go
 * Tests for chanwriter.go
 * By J. Stuart McMurray
 * Created 20260730
 * Last Modified 20260814
 */

import (
	"strconv"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/lib/tlog"
)

// Does a ChanWriter work as expected?
func TestChanWriter(t *testing.T) {
	synctest.Test(t, testChanWriter)
}
func testChanWriter(t *testing.T) {
	var (
		nWants = 10

		wants = make([]string, nWants, nWants+1)
		ch    = make(chan string, nWants+1)
		cw    = ChanWriter(ch)
	)

	/* Does write work? */
	for i := range wants {
		wants[i] = tlog.S("have-" + strconv.Itoa(i+1))
		wantN := len(wants[i])
		gotN, err := cw.Write([]byte(wants[i]))
		if gotN != wantN {
			t.Errorf(
				"[%d/%d] Incorrect length\n"+
					"have: %q\n"+
					" got: %d\n"+
					"want: %d",
				i+1, len(wants),
				wants[i],
				gotN,
				wantN,
			)
		}
		if nil != err {
			t.Errorf(
				"[%d/%d] Unexpected error: %v",
				i+1, len(wants),
				err,
			)
		}
	}

	/* Does writing an empty string work? */
	wants = append(wants, "")
	gotN, err := cw.Write([]byte{})
	if 0 != gotN {
		t.Errorf("Incorrect size writing empty slice: %d", gotN)
	}
	if nil != err {
		t.Errorf("Unexpected error writing empty slice: %v", err)
	}

	/* Everything buffer ok? */
	for i, want := range wants {
		if 0 == len(ch) {
			t.Fatalf(
				"[%d/%d] Channel empty, expected %q",
				i+1, len(wants),
				want,
			)
		}
		if got := <-ch; got != want {
			t.Errorf(
				"[%d/%d] Incorrect string buffered\n"+
					" got: %q\n"+
					"want: %q",
				i+1, len(wants),
				got,
				want,
			)
		}
	}
}
