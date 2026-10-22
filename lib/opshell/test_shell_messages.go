package opshell

/*
 * test_shell_messages.go
 * Test for shell messages
 * By J. Stuart McMurray
 * Created 20240925
 * Last Modified 20260730
 */

import (
	"bytes"
	"fmt"
	"testing"
)

// errorFer is anything that has an Errorf, like [testing.T].
type errorFer interface {
	Errorf(format string, args ...any)
	Helper()
}

// testTErrorf is an errorFer that just buffers the error messages, for
// testing failed tests.
type testTErrorf bytes.Buffer

// Errorf writes the formatted string and a newline to its internal buffer.
func (t *testTErrorf) Errorf(format string, args ...any) {
	fmt.Fprintf((*bytes.Buffer)(t), format, args...)
	(*bytes.Buffer)(t).WriteRune('\n')
}

// Helper is a no-op.
func (testTErrorf) Helper() { _ = struct{}{} /* Test coverage. */ }

// String returns t's internal buffer as a string.
func (t *testTErrorf) String() string { return (*bytes.Buffer)(t).String() }

// ExpectShellMessages makes sure that all of the lines in wantCLines are read
// from och in order.
//
// This is no longer done in a subtest, but may again in the future when
// subtests (i.e. [testing.T.Run]) are supported by [synctest.Test].
func ExpectShellMessages(t *testing.T, och <-chan CLine, wantCLines ...CLine) {
	t.Helper()
	expectShellMessages(t, och, wantCLines...)
}

// expectShellMessages does what ExpectShellMessages says it does, but with
// an errorFer for testing failures.
func expectShellMessages(t errorFer, och <-chan CLine, wantCLines ...CLine) {
	t.Helper()
	/* Make sure we get the shell messages we expect. */
	// t.Run("shell_messages", func(t *testing.T) { /* One day... */
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
			break
		}
		if got != want {
			t.Errorf(
				"Incorrect shell message %d/%d:\n"+
					" got: %#v\n"+
					"want: %#v",
				i+1, len(wantCLines),
				got,
				want,
			)
		}
	}
	// }) /* One day... */
}

// ExpectNoShellMessages calls t.Errorf for each message read from och.
func ExpectNoShellMessages(t *testing.T, och <-chan CLine) {
	t.Helper()
	expectNoShellMessages(t, och)

}

// expectNoShellMessages does what ExpectNoShellMessages says it does, but with
// an errorFer for testing failures.
func expectNoShellMessages(t errorFer, och <-chan CLine) {
	t.Helper()
	for extra := range och {
		t.Errorf("Leftover shell message: %#v", extra)
	}
}
