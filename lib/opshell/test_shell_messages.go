/*
 * shell_messages.go
 * Test for shell messages
 * By J. Stuart McMurray
 * Created 20240925
 * Last Modified 20240926
 */

package opshell

import (
	"testing"
)

// ExpectShellMessages makes sure that all of the lines in wantCLines are read
// from och.  This is done in a subtest.  t.FailNow will be called if the
// shell messages are incorrect.
func ExpectShellMessages(t *testing.T, och <-chan CLine, wantCLines ...CLine) {
	t.Helper()
	/* Make sure we get the shell messages we expect. */
	t.Run("shell_messages", func(t *testing.T) {
		t.Helper()
		var nGot int
		for i, want := range wantCLines {
			got, ok := <-och
			if !ok {
				t.Errorf(
					"Only got %d/%d shell messages",
					i,
					len(wantCLines),
				)
				for _, l := range wantCLines[i:] {
					t.Errorf(
						"Missing shell message: %#v",
						l,
					)
				}
				t.FailNow()
			}
			nGot++
			if got != want {
				t.Errorf(
					"Incorrect shell message:\n"+
						" got: %#v\n"+
						"want: %#v",
					got,
					want,
				)
			}
		}
	})
	if t.Failed() {
		t.FailNow()
	}
}

// ExpectNoShellMessages calls shutdown, if not nil, and runs a subtest to
// make sure nothing is read from och before it closes.
func ExpectNoShellMessages(t *testing.T, och <-chan CLine, shutdown func()) {
	t.Helper()
	/* Make sure we have no leftovers. */
	if nil != shutdown {
		shutdown()
	}
	t.Run("no_more_shell_messages", func(t *testing.T) {
		t.Helper()
		for extra := range och {
			t.Errorf("Extra shell message: %#v", extra)
		}
	})
}
