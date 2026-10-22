package bufferedconn

/*
 * bufferedconn_test.go
 * Tests for bufferedconn.go
 * By Stuart McMurray
 * Created 20260325
 * Last Modified 20260502
 */

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// Can we just do a send and receive?  Inspired by nettest.TestConn.
func TestConn_BasicIO(t *testing.T) {
	var (
		c1, c2 = NewPair()
		have   = []byte("kittens")
		wg     sync.WaitGroup
	)
	defer c1.Close()
	defer c2.Close()
	/* Send a chunk of bytes. */
	wg.Go(func() {
		defer func() {
			if err := c1.Close(); nil != err {
				t.Errorf("Closing write end: %s", err)
			}
		}()
		n, err := c1.Write(have)
		if got, want := n, len(have); got != want {
			t.Errorf(
				"Incorrect Write length\n"+
					"have: %q\n"+
					" got: %d\n"+
					"want: %d",
				have,
				got,
				want,
			)
		}
		if nil != err {
			t.Errorf("Write error: %s", err)
		}
	})
	/* Receive the bytes. */
	wg.Go(func() {
		got, err := io.ReadAll(c2)
		if !bytes.Equal(got, have) {
			t.Errorf(
				"Read incorrect\n got: %q\nwant: %q",
				got,
				have,
			)
		}
		if nil != err {
			t.Errorf("Read error: %s", err)
		}
	})
	wg.Wait()
}

// Does a timeout work?
func TestConnSetReadDeadline(t *testing.T) {
	synctest.Test(t, testConnSetReadDeadline)
}
func testConnSetReadDeadline(t *testing.T) {
	var (
		c1, _ = NewPair()
		b     = make([]byte, 1)
		n     int
		err   error
		wg    sync.WaitGroup
	)
	defer c1.Close()
	/* Start blocking on a read. */
	wg.Go(func() { n, err = c1.Read(b) })
	/* Set a read deadline for the future. */
	to := time.Now().Add(time.Hour)
	c1.SetReadDeadline(to)
	/* Wait for the read to give up. */
	wg.Wait()
	if 0 != n {
		t.Errorf("Non-zero byte count: %d", n)
	}
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Errorf("Incorrect read error: %s", err)
	}
	if got, want := time.Now(), to; !got.Equal(want) {
		t.Errorf(
			"Read unblocked at incorrect time\n"+
				" got: %s\n"+
				"want: %s",
			got.Format(time.RFC3339),
			want.Format(time.RFC3339),
		)
	}
}

// Can we queue a nothing happily?
func TestConnWrite_EmptySlice(t *testing.T) {
	c1, _ := NewPair()
	defer c1.Close()
	if n, err := c1.Write([]byte{}); nil != err {
		t.Errorf("Unexpected error: %s", err)
	} else if 0 != n {
		t.Errorf("Non-zero write count: %d", n)
	}
}

// Does closing a pipe for writing work?
func TestConn_CloseWriter(t *testing.T) {
	c1, _ := NewPair()
	defer c1.Close()
	if err := c1.Close(); nil != err {
		t.Errorf("Error closing writer: %s", err)
		return
	}
	if n, err := c1.Write([]byte("kittens")); !errors.Is(
		err,
		io.ErrClosedPipe,
	) {
		t.Errorf("Unexpected error writing after close: %s", err)
	} else if 0 != n {
		t.Errorf(
			"Non-zero write byte count writing after close: %d",
			n,
		)
	}
}

// Do we get an error when the peer closes while while we're blocked writing?
func TestConnWrite_CloseWhileBlocked(t *testing.T) {
	try := func(t *testing.T, wc, cc *Conn, wantErr error) {
		ech := make(chan error, 1)
		/* Fill the channel, next write should block. */
		fillCh(wc.txCh)
		go func() { _, err := wc.Write(make([]byte, 1)); ech <- err }()
		/* Wait until we're blocking. */
		synctest.Wait()
		/* Close the conn, should get an error. */
		if err := cc.Close(); nil != err {
			t.Errorf("Close error: %s", err)
		}
		if got, want := <-ech, wantErr; !errors.Is(got, want) {
			t.Errorf(
				"Incorrect write error after close\n"+
					" got: %s\n"+
					"want: %s",
				got,
				want,
			)
		}
	}
	t.Run("local_close", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			cl, _ := NewPair()
			defer cl.Close()
			try(t, cl, cl, io.ErrClosedPipe)
		})
	})
	t.Run("peer_close", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			cl, cr := NewPair()
			defer cl.Close()
			try(t, cl, cr, io.EOF)
		})
	})
}

// Do we get the right errors setting deadlines on closed Conns?
func TestConn_SetDeadlinesOnClosedConns(t *testing.T) {
	var (
		n      = time.Now()
		c1, c2 = NewPair()
	)
	defer c1.Close()
	defer c2.Close()
	if err := c2.Close(); nil != err {
		t.Fatalf("Close error: %s", err)
	}

	if err := c1.SetReadDeadline(n); !errors.Is(err, io.EOF) {
		t.Errorf(
			"Incorrect error setting read deadline "+
				"with closed peer: %s",
			err,
		)
	}
	if err := c1.SetWriteDeadline(n); !errors.Is(err, io.EOF) {
		t.Errorf(
			"Incorrect error setting write deadline "+
				"with closed peer: %s",
			err,
		)
	}
	if err := c2.SetReadDeadline(n); !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf(
			"Incorrect error setting read deadline "+
				"after close: %s",
			err,
		)
	}
	if err := c2.SetWriteDeadline(n); !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf(
			"Incorrect error setting write deadline "+
				"after close: %s",
			err,
		)
	}
}

// Can we close a conn while it's blocked on read.
func TestConnRead_CloseWhileBlocked(t *testing.T) {
	synctest.Test(t, testConnReadCloseWhileBlocked)
}
func testConnReadCloseWhileBlocked(t *testing.T) {
	var (
		ech   = make(chan error, 1)
		c1, _ = NewPair()
		b     = make([]byte, 1)
	)
	defer c1.Close()

	/* Wait until we're blocked on read. */
	go func() { _, err := c1.Read(b); ech <- err }()
	synctest.Wait()
	if 0 != len(ech) {
		t.Fatalf("Read finished before close")
	}

	/* Close the conn to unblock the read. */
	if err := c1.Close(); nil != err {
		t.Errorf("Close error: %s", err)
	}

	/* Did read unblock properly? */
	if err := <-ech; !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("Incorrect read error: %s", err)
	}

}

// Do we get the right error when writing to a closed peer?
func TestConnWrite_ClosedPeer(t *testing.T) {
	var (
		c1, c2 = NewPair()
		b      = make([]byte, 1)
	)
	defer c1.Close()
	defer c2.Close()
	if err := c2.Close(); nil != err {
		t.Fatalf("Close error: %s", err)
	}
	if _, err := c1.Write(b); !errors.Is(err, io.EOF) {
		t.Errorf("Incorrect write error: %s", err)
	}
}

// Do we stop reading when we're closed or timed out?
func TestConnRead_NoRead(t *testing.T) {
	t.Run("closed", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var (
				c1, _ = NewPair()
				buf   = make([]byte, 1)
			)
			c1.Close()
			_, got := c1.Read(buf)
			if want := io.ErrClosedPipe; !errors.Is(got, want) {
				t.Errorf(
					"Incorrect error\n got: %v\nwant: %v",
					got,
					want,
				)
			}
		})
	})
	t.Run("deadline_passed", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var (
				c1, _ = NewPair()
				buf   = make([]byte, 1)
			)
			defer c1.Close()
			c1.SetDeadline(time.Now())
			time.Sleep(time.Second)
			_, got := c1.Read(buf)
			if want := os.ErrDeadlineExceeded; !errors.Is(
				got,
				want,
			) {
				t.Errorf(
					"Incorrect error\n got: %v\nwant: %v",
					got,
					want,
				)
			}
		})
	})
}

// De we give up if we write after the write deadline has passed?
func TestConnWrite_AfterDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var (
			c1, _ = NewPair()
			buf   = make([]byte, 1)
		)
		defer c1.Close()
		c1.SetWriteDeadline(time.Now())
		time.Sleep(time.Second)
		_, got := c1.Write(buf)
		if want := os.ErrDeadlineExceeded; !errors.Is(
			got,
			want,
		) {
			t.Errorf(
				"Incorrect error\n got: %v\nwant: %v",
				got,
				want,
			)
		}
	})
}

// Do we give up if the write deadline happens while blocking?
func TestConnWrite_DeadlineWhileBlocked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var (
			c1, _ = NewPair()
			buf   = make([]byte, 1)
			wg    sync.WaitGroup
			got   error
		)
		defer c1.Close()
		/* Fill the buffer. */
		fillCh(c1.txCh)

		/* Next one should block. */
		wg.Go(func() { _, got = c1.Write(buf) })

		/* Wait until it blocks, then unblock it. */
		synctest.Wait()
		if err := c1.SetDeadline(time.Now()); nil != err {
			t.Errorf("Error setting deadline: %s", err)
		}
		time.Sleep(time.Minute)
		synctest.Wait()

		/* Did we get the right error? */
		wg.Wait()
		if want := os.ErrDeadlineExceeded; !errors.Is(got, want) {
			t.Errorf(
				"Incorrect error\n got: %v\nwant: %v",
				got,
				want,
			)
		}
	})
}

// Can we get addresses from the conn?
func TestConn_Addrs(t *testing.T) {
	cl, cr := NewPair()
	var pairNum *uint64
	for n, c := range map[string]struct {
		got  net.Addr
		want Addr
	}{"left/local": {
		got:  cl.LocalAddr(),
		want: Addr{Side: SideLeft},
	}, "left/remote": {
		got:  cl.RemoteAddr(),
		want: Addr{Side: SideRight},
	}, "right/local": {
		got:  cr.LocalAddr(),
		want: Addr{Side: SideRight},
	}, "right/remote": {
		got:  cr.RemoteAddr(),
		want: Addr{Side: SideLeft},
	}} {
		t.Run(n, func(t *testing.T) {
			/* Address should be an Addr under the hood. */
			got, ok := c.got.(Addr)
			if !ok {
				t.Fatalf(
					"Address was a %T, not an %T",
					c.got,
					got,
				)
			}

			/* We don't have a priori knowledge of the number, but
			they should all be the same anyways. */
			if nil == pairNum {
				pairNum = new(uint64(got.PairNum))
			}
			c.want.PairNum = *pairNum

			if got != c.want {
				t.Errorf(
					"Address incorrect\n"+
						" got: %#v\n"+
						"want: %#v",
					c.got,
					c.want,
				)
			}
		})
	}
}

// Can we close a conn for reading?
func TestConnCloseRead(t *testing.T) {
	/* Do we unblock a read when closing the read side? */
	run := func(t *testing.T) {
		/* Block on read. */
		var (
			cr, _ = NewPair()
			ech   = make(chan error, 1)
		)
		go func() { _, err := io.ReadAll(cr); ech <- err }()
		/* Wait until we (should be) blocked. */
		synctest.Wait()
		select {
		case err := <-ech:
			t.Fatalf("Got error instead of blocking: %v", err)
		default: /* Good. */
		}
		/* Close for reading, should unblock. */
		if err := cr.CloseRead(); nil != err {
			t.Fatalf("CloseRead returned error: %v", err)
		}
		errorIs(t, "Read after unblock", <-ech, io.ErrClosedPipe)
	}
	t.Run("unblock/read", func(t *testing.T) { synctest.Test(t, run) })

	/* Do we unblock a write when closing the read side? */
	run = func(t *testing.T) {
		/* Block on read. */
		var (
			cr, cw = NewPair()
			ech    = make(chan error, 1)
		)
		fillCh(cw.txCh)
		go func() { _, err := cw.Write(make([]byte, 1)); ech <- err }()
		/* Wait until we (should be) blocked. */
		synctest.Wait()
		select {
		case err := <-ech:
			t.Fatalf("Got error instead of blocking: %v", err)
		default: /* Good. */
		}
		/* Close for reading, should unblock. */
		if err := cr.CloseRead(); nil != err {
			t.Fatalf("CloseRead returned error: %v", err)
		}
		/* Writer should find out, too. */
		errorIs(t, "Write after unblock", <-ech, io.EOF)
	}
	t.Run("unblock/write", func(t *testing.T) { synctest.Test(t, run) })

	/* Does closing for reading also prevent future reads/writes? */
	run = func(t *testing.T) {
		var (
			cr, cw = NewPair()
			buf    = make([]byte, 1)
			err    error
		)

		/* Close a conn for reading, other ops should fail. */
		cr.CloseRead()

		/* Future Reads and Writes should fail. */
		_, err = cr.Read(buf)
		errorIs(t, "Read after CloseRead", err, io.ErrClosedPipe)
		_, err = cw.Write(buf)
		errorIs(t, "Write after CloseRead", err, io.EOF)
	}
	t.Run("after_close", func(t *testing.T) { synctest.Test(t, run) })
}

// Can we close a conn for writing?
func TestConnCloseWrite(t *testing.T) {
	/* Do we unblock a read when closing the write side? */
	run := func(t *testing.T) {
		/* Block on read. */
		var (
			cr, cw = NewPair()
			ech    = make(chan error, 1)
		)
		go func() { _, err := cr.Read(make([]byte, 1)); ech <- err }()
		/* Wait until we (should be) blocked. */
		synctest.Wait()
		select {
		case err := <-ech:
			t.Fatalf("Got error instead of blocking: %v", err)
		default: /* Good. */
		}
		/* Close for reading, should unblock. */
		if err := cw.CloseWrite(); nil != err {
			t.Fatalf("CloseWrite returned error: %v", err)
		}
		errorIs(t, "Read after unblock", <-ech, io.EOF)
	}
	t.Run("unblock/read", func(t *testing.T) { synctest.Test(t, run) })

	/* Do we unblock a write when closing the write side? */
	run = func(t *testing.T) {
		/* Block on read. */
		var (
			_, cw = NewPair()
			ech   = make(chan error, 1)
		)
		fillCh(cw.txCh)
		go func() { _, err := cw.Write(make([]byte, 1)); ech <- err }()
		/* Wait until we (should be) blocked. */
		synctest.Wait()
		select {
		case err := <-ech:
			t.Fatalf("Got error instead of blocking: %v", err)
		default: /* Good. */
		}
		/* Close for writing, should unblock. */
		if err := cw.CloseWrite(); nil != err {
			t.Fatalf("CloseWrite returned error: %v", err)
		}
		/* Writer should find out, too. */
		errorIs(t, "Write after unblock", <-ech, io.ErrClosedPipe)
	}
	t.Run("unblock/write", func(t *testing.T) { synctest.Test(t, run) })

	/* Does closing for writing also prevent future reads/writes? */
	run = func(t *testing.T) {
		var (
			cr, cw = NewPair()
			buf    = make([]byte, 1)
			err    error
		)

		/* Close a conn for write, other ops should fail. */
		cw.CloseWrite()

		/* Future Reads and Writes should fail. */
		_, err = cr.Read(buf)
		errorIs(t, "Read after CloseWrite", err, io.EOF)
		_, err = cw.Write(buf)
		errorIs(t, "Write after CloseWrite", err, io.ErrClosedPipe)
	}
	t.Run("after_close", func(t *testing.T) { synctest.Test(t, run) })
}

// fillCh fills ch with one-byte byte slices.
func fillCh(ch chan<- []byte) {
	for {
		select {
		case ch <- make([]byte, 1): /* Added a buffer. */
		default: /* Full */
			return
		}
	}
}

// errorIs calls t.Errorf if !errors.Is(got, want).  from is the source of the
// error, and appended to "Unexpected error from".
func errorIs(t *testing.T, from string, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Errorf(
			"Unexpected error from %s\n got: %v\nwant: %v",
			from,
			got,
			want,
		)
	}
}
