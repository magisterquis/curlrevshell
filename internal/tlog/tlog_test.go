package tlog

/*
 * tlog_test.go
 * Tests for tlog.go
 * By J. Stuart McMurray
 * Created 20251212
 * Last Modified 20260214
 */

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"testing"
	"testing/synctest"
	"time"
)

func TestLogBuffer_Smoketest(t *testing.T) { NewLogBuffer() }

func TestLogBufferWrite(t *testing.T) {
	lb, _ := NewLogBuffer()

	/* Can we add up to the limit? */
	msgs := make([]string, BufLen)
	for i := range msgs {
		msgs[i] = fmt.Sprintf("message-%d", i+1)
		if _, err := lb.Write([]byte(msgs[i])); nil != err {
			t.Fatalf(
				"Error writing message %d/%d: %s",
				i+1,
				len(msgs),
				err,
			)
		}
	}

	/* Do we get an error if we go past the limit? */
	if _, err := lb.Write([]byte("too much")); nil == err {
		t.Errorf("Able to write when full")
	} else if !errors.Is(err, ErrBufferFull) {
		t.Errorf("Unexpected error writing when full: %s", err)
	}

	/* Are the messages buffered correctly? */
	for i, want := range msgs {
		got, ok := <-lb.buf
		if !ok {
			t.Fatalf("Channel unexpectedly closed")
		}
		if got != want {
			t.Errorf(
				"Buffered message %d incorrect\n"+
					" got: %s\n"+
					"want: %s",
				i+1,
				got,
				want,
			)
		}
	}

	/* Was anything else left about? */
	if got := len(lb.buf); 0 != got {
		t.Errorf("Buffer should be empty, has %d messages", got)
	}
}

// Does logging with slog work?
func TestLogBufferWrite_Slog(t *testing.T) {
	/* Buffer a single message. */
	lb, sl := NewLogBuffer()
	sl.Info("It works", "k1", "v1", "k2", true, "k3", 3)
	/* Does a message get buffered? */
	if got, want := len(lb.buf), 1; got != want {
		t.Errorf(
			"Incorrect number of buffered messages\n"+
				" got: %d\n"+
				"want: %d",
			got,
			want,
		)
	}
	/* Is it correct? */
	close(lb.buf)
	s, ok := <-lb.buf
	if !ok {
		t.Errorf("Did not receive buffered message")
	} else if got, want := s, `{"level":"INFO","msg":"It works",`+
		`"k1":"v1","k2":true,"k3":3}`; got != want {
		t.Errorf(
			"Incorrect buffered message\n"+
				" got: %q\n"+
				"want: %q",
			got,
			want,
		)
	}
}

// Can we write multiple lines at once?
func TestLogBufferWrite_Multiline(t *testing.T) {
	var (
		lb, _ = NewLogBuffer()
		have  = "\nline one\n\nline two\nline 3\n\n"
		wants = []string{
			"",
			"line one",
			"",
			"line two",
			"line 3",
			"",
		}
	)
	/* Can we write a multiline message? */
	if n, err := lb.Write([]byte(have)); nil != err {
		t.Fatalf("Write error")
	} else if got, want := n, len(have); got != want {
		t.Fatalf(
			"Write returned incorrect byte count\n"+
				"have: %q\n"+
				" got: %d\n"+
				"want: %d",
			have,
			got,
			want,
		)
	}
	/* Buffer the correct number of messages? */
	if got, want := len(lb.buf), len(wants); got != want {
		t.Fatalf(
			"Incorrect number of buffered messages\n"+
				" got: %d\n"+
				"want: %d",
			got,
			want,
		)
	}
	/* Get the right writes in the right order? */
	for i, want := range wants {
		/* Need something to compare to. */
		if 0 == len(lb.buf) {
			t.Errorf("Leftover expected message: %q", wants)
			continue
		}
		/* Are these the same? */
		if got := <-lb.buf; got != want {
			t.Errorf(
				"Incorrect message %d/%d\n got: %q\nwant: %q",
				i+1,
				len(wants),
				got,
				want,
			)
		}
	}
}

// Does the buffer fill up?
func TestLogBufferWrite_Full(t *testing.T) {
	lb, _ := NewLogBuffer()
	/* Fill the buffer. */
	for i := range BufLen {
		n := i + 1
		if _, err := lb.Write([]byte(fmt.Sprintf(
			"msg-%d",
			n,
		))); nil != err {
			t.Fatalf("Write %d/%d failed: %s", n, BufLen, err)
		}
	}
	if l, c, want := len(lb.buf), cap(lb.buf), BufLen; l != c ||
		c != want {
		t.Fatalf(
			"Did not fill buffer\n len: %d\n cap: %d\nwant: %d",
			l,
			c,
			want,
		)
	}
	if _, err := lb.Write([]byte("overflow")); nil == err {
		t.Errorf("No error writing to full buffer")
	} else if got, want := err, ErrBufferFull; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error writing to full buffer\n"+
				"got: %s\n"+
				"want: %s",
			got,
			want,
		)
	}
}

// Can we clone a LogBuffer?
func TestLogBufferClone(t *testing.T) {
	/* Pair of Logbuffers. */
	makeLBs := func() (oldLB, newLB *LogBuffer) {
		lb, _ := NewLogBuffer()
		return lb, lb.Clone()
	}

	/* Do we share a channel? */
	check := func(t *testing.T, dst, src *LogBuffer) {
		/* Do we share a buffer? */
		t.Run("write", func(t *testing.T) {
			msg := "kittens"
			/* Buffers empty before we start? */
			if got, want := len(src.buf), 0; got != want {
				t.Fatalf(
					"Source buffer not empty before write",
				)
			} else if got, want := len(dst.buf), 0; got != want {
				t.Fatalf(
					"Source buffer not empty before write",
				)
			}
			/* Can we write? */
			if _, err := src.Write([]byte(msg)); nil != err {
				t.Fatalf("Write failed: %s", err)
			}
			/* Buffers still empty? */
			if got, want := len(src.buf), 1; got != want {
				t.Errorf("Write not buffered in Source")
			}
			if got, want := len(dst.buf), 1; got != want {
				t.Fatalf("Write not buffered in Dest")
			}

			/* Write work ok? */
			if got, ok := <-dst.buf; !ok {
				t.Fatalf("Did not read from buffer")
			} else if want := msg; got != want {
				t.Errorf(
					"Incorrect message buffered\n"+
						" got: %s\n"+
						"want: %s",
					got,
					want,
				)
			}
		})

		/* Do we share a close and close buffer? */
		t.Run("close", func(t *testing.T) {
			/* Do we share a close? */
			if closed(src) {
				t.Fatalf("Source closed before Close()")
			} else if closed(dst) {
				t.Fatalf("Dest closed before Close()")
			}
			if err := src.Close(); nil != err {
				t.Fatalf("Error closing source: %s", err)
			}
			if !closed(src) {
				t.Fatalf("Source not closed after Close()")
			} else if !closed(dst) {
				t.Fatalf("Dest not closed after Close()")
			}

			/* Can we write to the close buffer? */
			msg := "kittens"
			if got, want := 0, len(src.cBuf); got != want {
				t.Fatalf(
					"Source close buffer not empty " +
						"before post-Close write",
				)
			} else if got, want := 0, len(dst.cBuf); got != want {
				t.Fatalf(
					"Dest close buffer not empty " +
						"before post-Close write",
				)
			} else if _, err := src.Write(
				[]byte(msg),
			); nil == err {
				t.Fatalf("Post-close Write returned no error")
			} else if !errors.Is(err, ErrBufferClosed) {
				t.Fatalf("Post-close Write failed: %s", err)
			}

			/* Did it work? */
			if got, want := len(src.cBuf), 1; got != want {
				t.Errorf(
					"Post-Close Write " +
						"not buffered in Source",
				)
			}
			if got, want := len(dst.cBuf), 1; got != want {
				t.Fatalf(
					"Post-Close Write " +
						"not buffered in Dest",
				)
			} else if got, ok := <-dst.cBuf; !ok {
				t.Fatalf("Did not read from close buffer")
			} else if want := msg; got != want {
				t.Errorf(
					"Incorrect post-Close message "+
						"buffered\n"+
						" got: %s\n"+
						"want: %s",
					got,
					want,
				)
			}
		})
	}

	t.Run("old<-new", func(t *testing.T) {
		oldLB, newLB := makeLBs()
		check(t, oldLB, newLB)
	})
	t.Run("new<-old", func(t *testing.T) {
		oldLB, newLB := makeLBs()
		check(t, newLB, oldLB)
	})
}

// Can we close the LogBuffer?
func TestLogBufferClose(t *testing.T) {
	lb, _ := NewLogBuffer()

	/* Make sure we're all nice and clean. */
	if closed(lb) {
		t.Fatalf("Closed after NewLogBuffer")
	} else if 0 != len(lb.buf) {
		t.Fatalf("Buffer not empty after NewLogBuffer")
	} else if 0 != len(lb.cBuf) {
		t.Fatalf("Close buffer not empty after NewLogBuffer")
	}

	/* Pre-close write should work. */
	beforeMsg := "kittens"
	if _, err := lb.Write([]byte(beforeMsg)); nil != err {
		t.Fatalf("Error writing before close: %s", err)
	}
	var got string
	select {
	case got = <-lb.buf:
	default:
		t.Fatalf("Did not get buffered message before close")
	}
	if want := beforeMsg; got != want {
		t.Fatalf(
			"Got incorrect message before close\n"+
				" got: %s\n"+
				"want: %s",
			got,
			want,
		)
	}

	/* Does close work? */
	if err := lb.Close(); nil != err {
		t.Fatalf("Close failed: %s", err)
	} else if !closed(lb) {
		t.Fatalf("Close did not change closed")
	}

	/* Does a write after close work? */
	afterMsg := "moose"
	if _, err := lb.Write([]byte(afterMsg)); nil == err {
		t.Errorf("Write after close returned no error")
	} else if !errors.Is(err, ErrBufferClosed) {
		t.Fatalf("Unexpected error writing after close: %s", err)
	}
	select {
	case got = <-lb.cBuf:
	default:
		t.Fatalf("Did not get buffered message after close")
	}
	if want := afterMsg; got != want {
		t.Fatalf(
			"Got incorrect message after close\n"+
				" got: %s\n"+
				"want: %s",
			got,
			want,
		)
	}
}

// Can we dequeue a log message?
func TestLogBufferNextMessage(t *testing.T) {
	var (
		have = "kittens"
		want = M.Info(have)
	)
	/* Can we just read a normal message? */
	t.Run("one_message", func(t *testing.T) {
		lb, sl := NewLogBuffer()
		/* Queue the message. */
		sl.Info(have)
		/* Did it work? */
		got, err := lb.NextMessage(t.Context())
		if nil != err {
			t.Fatalf("Error getting message: %s", err)
		}
		if !got.Equal(want) {
			t.Errorf(
				"Dequeued message incorrect\n"+
					" got: %s\n"+
					"want: %s",
				got,
				want,
			)
		}
	})

	/* Can we block while looking for a message? */
	t.Run("blocking", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			lb, sl := NewLogBuffer()
			var (
				err    error
				got    Msg
				pause  = time.Minute
				start  = time.Now()
				waited time.Duration
			)
			/* Block waiting for a message. */
			go func() {
				got, err = lb.NextMessage(t.Context())
				waited = time.Since(start)
			}()
			/* Send the message after a bit. */
			time.Sleep(pause)
			sl.Info(have)
			synctest.Wait()
			/* Did it work? */
			if got, want := waited, pause; got != want {
				t.Errorf(
					"Blocked wrong amount of time\n"+
						" got: %s\n"+
						"want: %s",
					got,
					want,
				)
			}
			if nil != err {
				t.Errorf("Error getting message: %s", err)
			}
			if !got.Equal(want) {
				t.Errorf(
					"Dequeued message incorrect\n"+
						" got: %s\n"+
						"want: %s",
					got,
					want,
				)
			}
		})
	})

	/* Can we not block? */
	t.Run("nonblocking", func(t *testing.T) {
		lb, _ := NewLogBuffer()
		got, err := lb.WithNoWait().NextMessage(t.Context())
		if nil == err {
			t.Errorf("Unexpected success")
		} else if !errors.Is(err, ErrBufferEmpty) {
			t.Errorf("Unexpected error: %s", err)
		}
		if !got.IsZero() {
			t.Errorf("Got unexpected nonzero message: %#v", got)
		}
	})

	/* Will the context stop us blocking? */
	t.Run("context_cancel", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			lb, _ := NewLogBuffer()
			var (
				pause       = time.Minute
				start       = time.Now()
				ctx, cancel = context.WithTimeout(
					t.Context(),
					pause,
				)
			)
			defer cancel()
			/* Block waiting for a message. */
			got, err := lb.NextMessage(ctx)
			/* Did we actually get cancelled? */
			if got, want := time.Since(start), pause; got != want {
				t.Errorf(
					"Blocked wrong amount of time\n"+
						" got: %s\n"+
						"want: %s",
					got,
					want,
				)
			}
			/* Were we told we were cancelled? */
			if nil == err {
				t.Errorf("Got nil error")
			} else if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Incorrect error: %s", err)
			}
			/* Did we get a message anyways? */
			if !got.IsZero() {
				t.Errorf(
					"Got unexpected nonzero message: %#v",
					got,
				)
			}
		})
	})
}

// Can we check for ordered messages?  It'd be nice to test for incorrect
// messages, but then we'd fail tests.
func TestLogBuffer_Expect(t *testing.T) {
	/* Since we can't really test this without things failing, the below
	can be switched on to manually verify things. */
	var (
		/* Log a message that we're not expecting. */
		addUnexpectedLogMessage = false
		/* Expect a message that's not logged. */
		addMissingLogMessage = false
	)

	/* Queue up a bunch of log messages. */
	newLB := func() (*LogBuffer, []Msg) {
		lb, sl := NewLogBuffer()
		var (
			want []Msg
			msg  string
			asl  = sl            /* To line things up. */
			a    = func(m Msg) { /* To line things up. */
				want = append(want, m)
			}
		)
		/* Standalone messages. */
		msg = "Message 1"
		asl.Info(msg)
		a(M.Info(msg))
		msg = "Message 2"
		k1 := "lk1"
		v1 := "lv1"
		asl.Debug(msg, k1, v1)
		a(M.With(k1, v1).Debug(msg))
		msg = "Message 3"
		asl.With("k1", "v1").Warn(msg)
		a(M.With("k1", "v1").Warn(msg))
		msg = "Message 4"
		asl.WithGroup("g1").With("g1k1", "g1v1").Error(msg)
		a(M.WithGroup("g1").With("g1k1", "g1v1").Error(msg))

		/* Knob-controlled messages. */
		if addUnexpectedLogMessage {
			asl.Error("Unexpected")
		}
		if addMissingLogMessage {
			a(M.Error("Missing"))
		}

		/* Built-up messages. */
		msg = "Start"
		asl.Info(msg)
		a(M.Info(msg))
		msg = "One pair"
		bsl := asl.With("bk1", "bv1")
		mmm := (M).With("bk1", "bv1")
		m := mmm
		bsl.Warn(msg)
		a(m.Warn(msg))
		msg = "Second pair"
		bsl = bsl.With("bk2", "bv2")
		mmm = (m).With("bk2", "bv2")
		m = mmm
		bsl.Debug(msg)
		a(m.Debug(msg))
		msg = "A group"
		bsl = bsl.WithGroup("bg1")
		mmm = (m).WithGroup("bg1")
		m = mmm
		bsl.Error(msg)
		a(m.Error(msg))
		msg = "Group's attributes"
		bsl = bsl.With("bg1k1", "bg1v1")
		mmm = (m).With("bg1k1", "bg1v1")
		m = mmm
		bsl.Log(t.Context(), slog.Level(2), msg)
		a(m.Log("INFO+2", msg))
		return lb, want
	}

	/* Does it work? */
	t.Run("ordered", func(t *testing.T) {
		lb, want := newLB()
		lb.Expect(t.Context(), t, want...)
	})

	t.Run("unordered", func(t *testing.T) {
		lb, want := newLB()
		uwant := slices.Clone(want)
		for slices.EqualFunc(want, uwant, func(m1, m2 Msg) bool {
			return m1.Equal(m2)
		}) {
			rand.Shuffle(len(uwant), func(i, j int) {
				uwant[i], uwant[j] = uwant[j], uwant[i]
			})
		}
		lb.WithExpectUnordered().Expect(t.Context(), t, want...)
	})
}

// Can we expect a message with an empty group?
func TestLogBuffer_Expect_EmptyGroup(t *testing.T) {
	var (
		lb, sl = NewLogBuffer()
		group  = "g1"
		msg    = "kittens"
	)
	sl.
		WithGroup(group).Info(msg)
	lb.Expect(t.Context(), t, M.
		WithGroup(group).Info(msg),
	)
}

// Can we make sure we've an empty buffer?  Kinda hard to test without failing
// tests...
func TestLogBuffer_Expect_ExpectEmpty(t *testing.T) {
	// We don't really have a good way to test this without causing the
	// test to fail (or writing a large test harness).
	// Set shouldFail to true to see Expect cause a test failure.
	shouldFail := false

	var (
		lb, sl = NewLogBuffer()
		msg    = "kittens"
	)
	sl.Info(msg)
	if shouldFail {
		sl.Debug("This should cause a test failure")
	}
	lb.WithExpectEmpty().Expect(t.Context(), t, M.Info(msg))
}

// Can we make sure we won't wait on an empty buffer?  Kinda hard to test
// without failing tests...
func TestLogBuffer_Expect_NoWait(t *testing.T) {
	// We don't really have a good way to test this without causing the
	// test to fail (or writing a large test harness).
	// Set shouldFail to true to see Expect cause a test failure.
	shouldFail := false

	var (
		lb, sl = NewLogBuffer()
		msg    = "kittens"
	)
	if !shouldFail {
		sl.Info(msg)
	}
	lb.WithNoWait().Expect(t.Context(), t, M.Info(msg))

}

// Can we get a list of messages sent after Close?  Kinda hard to test without
// failing tests...
func TestTestEmptyAfterClose(t *testing.T) {
	// We don't really have a good way to test this without causing the
	// test to fail (or writing a large test harness).
	// Set shouldFail to true to see Expect cause a test failure.
	shouldFail := false

	lb, sl := NewLogBuffer()

	/* Send a message pre-close, for just in case. */
	msg := "kittens"
	sl.Info(msg)
	want := M.Info(msg)

	/* Close and (maybe) send some more messages. */
	if err := lb.Close(); nil != err {
		t.Fatalf("Close returned error: %s", err)
	}
	if shouldFail {
		for i := range 4 {
			sl.Error(fmt.Sprintf("AfterClose-%d", i))
		}
	}

	/* Should get the normally-logged message. */
	lb.Expect(t.Context(), t, want)

	/* Shouldn't get anything after closing, unless we should. */
	lb.TestEmptyAfterClose(t)
}

// Does TestEmptyAfterClose also Close?
func TestTestEmptyAfterClose_NoCloseFirst(t *testing.T) {
	lb, sl := NewLogBuffer()
	/* Send a message pre-close, for just in case. */
	msg := "kittens"
	sl.Info(msg)
	want := M.Info(msg)
	lb.Expect(t.Context(), t, want)

	/* Shouldn't get anything after closing, but should also close. */
	lb.TestEmptyAfterClose(t)

	/* Did it close? */
	if !closed(lb) {
		t.Errorf(
			"LogBuffer not closed after call to " +
				"TestEmptyAFterClose",
		)
	}

	/* If we write another log message, does it go to the right place? */
	msg = "moose"
	sl.Info(msg)
	if got, want := len(lb.cBuf), 1; got != want {
		t.Errorf(
			"Incorrect number of log lines buffered after close\n"+
				" got: %d\n"+
				"want: %d",
			got,
			want,
		)
	}

	/* Should have got one line. */
	if 1 <= len(lb.cBuf) {
		if got, want := <-lb.cBuf, M.Info(msg).String(); got != want {
			t.Errorf(
				"Incorrect log buffered after close\n"+
					" got: %s\n"+
					"want: %s",
				got,
				want,
			)
		}
	}

	/* Shouldn't have got more than one line. */
	for 0 < len(lb.cBuf) {
		t.Errorf("Unexpected log line after close: %s", <-lb.cBuf)
	}

}

// closed indicates if lb.Close has been called.
func closed(lb *LogBuffer) bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return *lb.closed
}
