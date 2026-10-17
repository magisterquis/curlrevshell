package bufferedconn

/*
 * deadline_test.go
 * Tests for deadline.go
 * By Stuart McMurray
 * Created 20260402
 * Last Modified 20260402
 */

import (
	"testing"
	"testing/synctest"
	"time"
)

// Can we set deadlines in several edge cases?
func TestConnSetDeadline_EdgeCases(t *testing.T) {
	synctest.Test(t, testConnSetDeadlineEdgeCases)
}
func testConnSetDeadlineEdgeCases(t *testing.T) {
	cl, _ := NewPair()
	defer cl.Close()

	/* Deadline which we'll pass. */
	if err := cl.SetReadDeadline(time.Now().Add(time.Minute)); nil != err {
		t.Errorf("Error setting deadline in the future: %s", err)
	}
	time.Sleep(2 * time.Minute)

	/* Set another deadline, to re-enable reading. */
	if err := cl.SetReadDeadline(time.Now().Add(time.Minute)); nil != err {
		t.Errorf(
			"Error setting second deadline in the future: %s",
			err,
		)
	}
	time.Sleep(2 * time.Minute)

	/* Set a zero read deadline, no deadline. */
	if err := cl.SetReadDeadline(time.Time{}); nil != err {
		t.Errorf("Error setting zero deadline: %s", err)
	}
}
