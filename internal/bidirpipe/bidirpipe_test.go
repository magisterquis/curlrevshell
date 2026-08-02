package bidirpipe

/*
 * bidirpipe_test.go
 * Tests for bidirpipe.go
 * By J. Stuart McMurray
 * Created 20260626
 * Last Modified 20260627
 */

import (
	"errors"
	"io"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/internal/tlog"
)

// Can we do normal pipe things?
func TestBidirPipe(t *testing.T) {
	synctest.Test(t, testBidirPipe)
}
func testBidirPipe(t *testing.T) {
	var (
		p1, p2 = New()
		wg     = new(sync.WaitGroup)
		msg1   = tlog.S("msg-1")
		msg2   = tlog.S("msg-2")
	)
	wg.Go(func() { /* Write 1 -> 2 */
		n, err := p1.Write([]byte(msg1))
		if got, want := n, len(msg1); got != want {
			t.Errorf(
				"Write 1->2: wrong length\n got: %d\nwant: %d",
				got,
				want,
			)
		}
		if nil != err {
			t.Errorf("Write 1->2: error: %v", err)
		}
	})
	wg.Go(func() { /* Write 2 -> 1 */
		n, err := p2.Write([]byte(msg2))
		if got, want := n, len(msg2); got != want {
			t.Errorf(
				"Write 2->1: wrong length\n got: %d\nwant: %d",
				got,
				want,
			)
		}
		if nil != err {
			t.Errorf("Write 2->1: error: %v", err)
		}
	})
	wg.Go(func() { /* Read 1 -> 2 */
		b := make([]byte, len(msg1))
		if _, err := io.ReadFull(p2, b); nil != err {
			t.Errorf("Read 1->2: error: %v", err)
		}
		if got, want := string(b), msg1; got != want {
			t.Errorf(
				"Read 1->2: incorrect message\n"+
					" got: %q\n"+
					"want: %q",
				got,
				want,
			)
		}
	})
	wg.Go(func() { /* Read 2 -> 1 */
		b := make([]byte, len(msg2))
		if _, err := io.ReadFull(p1, b); nil != err {
			t.Errorf("Read 2->1: error: %v", err)
		}
		if got, want := string(b), msg2; got != want {
			t.Errorf(
				"Read 2->1: incorrect message\n"+
					" got: %q\n"+
					"want: %q",
				got,
				want,
			)
		}
	})
	wg.Wait()

	/* Errors after close correct? */
	if err := p1.Close(); nil != err {
		t.Errorf("Close 1: %v", err)
	}
	if err := p2.Close(); nil != err {
		t.Errorf("Close 2: %v", err)
	}
	wg = new(sync.WaitGroup)
	wg.Go(func() { /* Write 1 -> 2 */
		_, err := p1.Write([]byte(msg1))
		if got, want := err, io.ErrClosedPipe; !errors.Is(got, want) {
			t.Errorf(
				"Write 1->2 after close: incorrect error\n"+
					" gat: %v\n"+
					"want: %v",
				got,
				want,
			)
		}
	})
	wg.Go(func() { /* Write -> 1 */
		_, err := p2.Write([]byte(msg1))
		if got, want := err, io.ErrClosedPipe; !errors.Is(got, want) {
			t.Errorf(
				"Write 2->1 after close: incorrect error\n"+
					" gat: %v\n"+
					"want: %v",
				got,
				want,
			)
		}
	})
	wg.Go(func() { /* Read 1 -> 2 */
		_, err := p1.Read([]byte(msg1))
		if got, want := err, io.ErrClosedPipe; !errors.Is(got, want) {
			t.Errorf(
				"Read 1->2 after close: incorrect error\n"+
					" gat: %v\n"+
					"want: %v",
				got,
				want,
			)
		}
	})
	wg.Go(func() { /* Read 2 -> 1 */
		_, err := p1.Read([]byte(msg1))
		if got, want := err, io.ErrClosedPipe; !errors.Is(got, want) {
			t.Errorf(
				"Read 1->2 after close: incorrect error\n"+
					" gat: %v\n"+
					"want: %v",
				got,
				want,
			)
		}
	})
	wg.Wait()

	/* Make sure a second close is ok. */
	if err := p1.Close(); nil != err {
		t.Errorf("Second close 1: %v", err)
	}
	if err := p2.Close(); nil != err {
		t.Errorf("Second close 2: %v", err)
	}
}

// Can we stop writes?
func TestBidirPipeCloseRead(t *testing.T) {
	synctest.Test(t, testBidirPipeCloseRead)
}
func testBidirPipeCloseRead(t *testing.T) {
	var (
		p1, p2 = New()
		msg    = tlog.S("msg")
	)

	/* Can we close for reading? */
	if err := p1.CloseRead(); nil != err {
		t.Fatalf("Error calling CloseRead: %v", err)
	}
	if err := p1.CloseRead(); nil != err {
		t.Fatalf("Error calling CloseRead again: %v", err)
	}

	/* Correct errors if we try to read/write in the closed direction? */
	_, err := p1.Read(make([]byte, 1))
	if got, want := err, io.ErrClosedPipe; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error reading after CloseRead\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
	_, err = p2.Write([]byte(msg))
	if got, want := err, io.ErrClosedPipe; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error writing after CloseRead\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}

	/* Make sure a Close is ok. */
	if err := p1.Close(); nil != err {
		t.Errorf("Error closing CloseRead end: %v", err)
	}
	if err := p2.Close(); nil != err {
		t.Errorf("Error closing non-CloseRead end: %v", err)
	}
}

// Can we stop reads?
func TestBidirPipeCloseWrite(t *testing.T) {
	synctest.Test(t, testBidirPipeCloseWrite)
}
func testBidirPipeCloseWrite(t *testing.T) {
	var (
		p1, p2 = New()
		msg    = tlog.S("msg")
	)

	/* Can we close for reading? */
	if err := p1.CloseWrite(); nil != err {
		t.Fatalf("Error calling CloseWrite: %v", err)
	}
	if err := p1.CloseWrite(); nil != err {
		t.Fatalf("Error calling CloseWrite again: %v", err)
	}

	/* Correct errors if we try to read/write in the closed direction? */
	_, err := p1.Write([]byte(msg))
	if got, want := err, io.ErrClosedPipe; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error writing after CloseWrite\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
	_, err = p2.Read(make([]byte, 1))
	if got, want := err, io.EOF; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error reading after CloseWrite\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}

	/* Make sure a Close is ok. */
	if err := p1.Close(); nil != err {
		t.Errorf("Error closing CloseWrite end: %v", err)
	}
	if err := p2.Close(); nil != err {
		t.Errorf("Error closing non-CloseWrite end: %v", err)
	}
}
