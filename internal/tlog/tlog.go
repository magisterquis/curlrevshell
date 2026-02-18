// Package tlog - Testing-friendly logger
package tlog

/*
 * tlog.go
 * Testing-friendly logger
 * By J. Stuart McMurray
 * Created 20251212
 * Last Modified 20260216
 */

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"testing"
)

// BufLen is the size of LogBuffer's internal buffers.
const BufLen = 1024

// LogBuffer holds log messages received from the [slog.Logger] returned by
// New.
// The With* methods return a copy of LogBuffer with options set but with the
// same underlying storage.
type LogBuffer struct {
	mu *sync.RWMutex

	/* Main buffer. */
	buf chan string

	/* Close() things. */
	closed *bool
	cBuf   chan string

	/* Configurables, not locked by mu. */
	expectEmpty     bool
	expectNoWait    bool
	expectUnordered bool

	/* NB: Logbuffer.Clone makes a shallow copy of itself, so
	pointer fields are shared between clones but
	non-pointer fields are not shared between clones. */
}

// NewLogBuffer returns a new Buffer and logger to write to the buffer.
// Logs will be written at level DEBUG.
// The buffer will have space for BufLen log entries.
func NewLogBuffer() (*LogBuffer, *slog.Logger) {
	lb := &LogBuffer{
		mu:     new(sync.RWMutex),
		buf:    make(chan string, BufLen),
		closed: new(bool),
		cBuf:   make(chan string, BufLen),
	}
	sl := slog.New(slog.NewJSONHandler(lb, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		/* Remove the timestamp. */
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if 0 == len(groups) && slog.TimeKey == a.Key {
				return slog.Attr{}
			}
			return a

		},
	}))
	return lb, sl
}

// Clone returns a copy of l which shares l's underlying buffers and closed
// atomic Bool.
func (l *LogBuffer) Clone() *LogBuffer {
	n := l
	return n
}

// Write adds the lines b to l's internal buffer with no newlines.
// It returns 0, [ErrBufferFull] if l's internal buffer is full.
func (l *LogBuffer) Write(b []byte) (int, error) {
	var (
		tot       = 0
		errClosed error
	)
	for line := range bytes.Lines(b) {
		/* Trim trailing newline. */
		nlTrimmed := 0
		if '\n' == line[len(line)-1] {
			line = line[:len(line)-1]
			nlTrimmed = 1
		}
		/* Buffer the line somewhere. */
		n, err := l.writeLine(string(line))
		tot += n
		if errors.Is(err, ErrBufferClosed) {
			errClosed = err
		} else if nil != err {
			return tot, err
		}
		/* Only consider the newline written if the write succeeded. */
		tot += nlTrimmed
	}
	return tot, errClosed
}

// writeLine writes a single line to l.
func (l *LogBuffer) writeLine(line string) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	/* Buffer we use depends on if we're closed. */
	var err error
	ch := l.buf
	if *l.closed {
		err = ErrBufferClosed
		ch = l.cBuf
	}

	/* Buffer the line. */
	select {
	case ch <- line: /* Ok. */
	default: /* Too full. */
		return 0, ErrBufferFull
	}

	return len(line), err
}

// Close marks l as closed and returns nil.  It may be called more than once.
// Further writes will return an error but will be noted for a later call to
// l.TestEmptyAfterClose.
func (l *LogBuffer) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !*l.closed {
		*l.closed = true
		close(l.buf)
	}
	return nil
}

// CloseExpectEmpty is a convenience method which first closes l and then
// makes sure there are no buffered logs.
func (l *LogBuffer) CloseExpectEmpty(ctx context.Context, t *testing.T) {
	t.Helper()
	l.closeExpectEmpty(ctx, t)
}

// closeExpectEmpty does what CloseExpectEmpty says it does, but takes a ter
// for testing the unhappy path.
func (l *LogBuffer) closeExpectEmpty(ctx context.Context, t ter) {
	if err := l.Close(); nil != err {
		/* Unpossible. */
		t.Errorf("Error closing LogBuffer: %s", err)
	}
	l.WithExpectEmpty().expect(ctx, t)
}

// IsClosed indicates if l.Close has been called.
func (l *LogBuffer) IsClosed() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return *l.closed
}

// Expect checks that the next log messages are equivalent to msgs
// (which may be no messages) and marks t as failed and prints a messages if
// not.
// Unless WithExpectUnordered was used, messages are expected to be in the
// order passed to Expect.
func (l *LogBuffer) Expect(ctx context.Context, t *testing.T, msgs ...Msg) {
	t.Helper()
	l.expect(ctx, t, msgs...)
}

// expect does what Expect says it does, but takes a ter for testing the
// unhappy path.
func (l *LogBuffer) expect(ctx context.Context, t ter, msgs ...Msg) {
	/* Check for messages which should be there. */
	if l.expectUnordered {
		l.checkUnordered(ctx, t, msgs)
	} else {
		l.checkOrdered(ctx, t, msgs)
	}

	/* If we don't expect any more, make sure there's no more. */
	for l.expectEmpty && 0 != len(l.buf) {
		t.Error(leftoverLogMessageMessage{Msg: <-l.buf})
	}
}

// checkOrdered checks that all of the messages in msgs are buffered in the
// order they're in in msgs.
func (l *LogBuffer) checkOrdered(ctx context.Context, t ter, msgs []Msg) {
	for i, wantMsg := range msgs {
		n := i + 1
		gotMsg, err := l.NextMessage(ctx)
		if nil != err {
			t.Error(errorWaitingForMessageMessage{
				Idx: n,
				Len: len(msgs),
				Err: err,
			})
			return
		}
		if !wantMsg.Equal(gotMsg) {
			t.Error(incorrectLogMessageMessage{
				Idx:  n,
				Len:  len(msgs),
				Got:  gotMsg,
				Want: wantMsg,
			})
		}
	}
}

// checkUnordered checks that all of the Msgs in msgs are buffered in any
// order.
func (l *LogBuffer) checkUnordered(ctx context.Context, t ter, msgs []Msg) {
	/* Work out the ones we want.  This'll be something like O(n**2), but
	with set sizes small enough a map isn't worth the effort. */
	want := make([]*Msg, len(msgs))
	rem := len(want)
	for i := range msgs {
		want[i] = &msgs[i]
	}

	/* Mark off the messages we have. */
	for 0 < rem {
		/* Get the next buffered message. */
		gotMsg, err := l.NextMessage(ctx)
		if nil != err {
			t.Error(errorWaitingForMessageMessage{
				Idx: 1 + len(want) - rem,
				Len: len(want),
				Err: err,
			})
			break
		}
		rem--
		/* Work out where it is in the slice. */
		idx := slices.IndexFunc(want, func(p *Msg) bool {
			if nil == p {
				return false
			}
			return (*p).Equal(gotMsg)
		})
		if -1 == idx { /* Unexpected message. */
			t.Error(unexpectedLogMessageMessage{Msg: gotMsg})
			continue
		}
		/* Note we've seen it. */
		want[idx] = nil
	}

	/* Anything left over is a problem. */
	for i, p := range want {
		if nil == p { /* Good. */
			continue
		}
		t.Error(unfoundLogMessageMessage{
			Idx:  i + 1,
			Len:  len(want),
			Want: *p,
		})
	}
}

// NextMessage returns the next buffered message, according to l's
// configuration.
func (l *LogBuffer) NextMessage(ctx context.Context) (Msg, error) {
	/* ret prepares a receive from l.buf for returning. */
	ret := func(line string, ok bool) (Msg, error) {
		if !ok {
			return Msg{}, ErrBufferClosed
		}
		msg, err := NewMsgFromJSON(line)
		if nil != err {
			return Msg{}, fmt.Errorf(
				"parsing line %q: %w",
				line,
				err,
			)
		}
		return msg, nil
	}

	/* Read, without blocking. */
	select {
	case <-ctx.Done():
		return Msg{}, ctx.Err()
	case line, ok := <-l.buf: /* Normal buffered message. */
		return ret(line, ok)
	default: /* Didn't get anything. */
		if l.expectNoWait {
			return Msg{}, ErrBufferEmpty
		}
	}

	/* We must me meant to block. */
	select {
	case <-ctx.Done():
		return Msg{}, ctx.Err()
	case msg, ok := <-l.buf: /* Normal buffered message. */
		return ret(msg, ok)
	}
}

// WithExpectEmpty returns a copy of l configured such that Expect also notes a
// test failure if there were more buffered messages than passed to expect.
// Leftover buffered messages will be unbuffered and logged as test failures.
func (l *LogBuffer) WithExpectEmpty() *LogBuffer {
	n := l.Clone()
	n.expectEmpty = true
	return n
}

// WithNoWait returns a copy of l configured such that expect does not block
// waiting for as many log messages as it was passed.
func (l *LogBuffer) WithNoWait() *LogBuffer {
	n := l.Clone()
	n.expectNoWait = true
	return n
}

// WithExpectUnordered returns a copy of l configured such that Expect does not
// expect logged messages to be in order.
func (l *LogBuffer) WithExpectUnordered() *LogBuffer {
	n := l.Clone()
	n.expectUnordered = true
	return n
}

// TestEmptyAfterClose notes test failures if messages were written to l after
// l.Close was called.
// Each message written after close before TestEmptyAfterClose are called will
// be one test failure, messages written during the call to TestEmptyAfterClose
// may be noted as a test failure, and messages written after
// TestEmptyAfterClose will not be noted as a test failure unless another call
// to TestEmptyAfterClose is made.
// if l.Close has not been closed, TestEmptyAfterClose closes l.
func (l *LogBuffer) TestEmptyAfterClose(t *testing.T) {
	t.Helper()
	l.testEmptyAfterClose(t)
}

// testEmptyAfterClose does what TestEmptyAfterClose says it does, but takes a
// ter for testing the unhappy path.
func (l *LogBuffer) testEmptyAfterClose(t ter) {
	if err := l.Close(); nil != err {
		/* Unpossible. */
		t.Fatalf("Error closing LogBuffer: %s", err)
	}
	for {
		select {
		case m := <-l.cBuf:
			t.Error(messageSentAfterCloseMessage{Msg: m})
		default:
			/* No (more) messages. */
			return
		}
	}
}
