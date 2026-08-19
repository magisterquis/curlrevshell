// Package tlog - Testing-friendly logger
package tlog

/*
 * tlog.go
 * Testing-friendly logger
 * By J. Stuart McMurray
 * Created 20251212
 * Last Modified 20260613
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

// BufLen is the size of Buffer's internal buffers.
const BufLen = 1024

// Buffer holds log messages received from the [slog.Logger] returned by
// New.
// The With* methods return a copy of Buffer with options set but with the
// same underlying storage.
type Buffer struct {
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

	/* NB: Buffer.Clone makes a shallow copy of itself, so
	pointer fields are shared between clones but
	non-pointer fields are not shared between clones. */
}

// NewBuffer returns a new Buffer and logger to write to the buffer.
// Logs will be written at level DEBUG.
// The buffer will have space for BufLen log entries.
func NewBuffer() (*Buffer, *slog.Logger) {
	lb := &Buffer{
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
func (l *Buffer) Clone() *Buffer {
	l.mu.RLock()
	defer l.mu.RUnlock()
	n := *l
	return &n
}

// Write adds the lines b to l's internal buffer with no newlines.
// It returns 0, [ErrBufferFull] if l's internal buffer is full.
func (l *Buffer) Write(b []byte) (int, error) {
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
func (l *Buffer) writeLine(line string) (int, error) {
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
func (l *Buffer) Close() error {
	l.close()
	return nil
}

// close does what Close says it does, but doesn't pretend to return an error.
func (l *Buffer) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !*l.closed {
		*l.closed = true
		close(l.buf)
	}
}

// CloseExpectEmpty is a convenience method which first closes l and then
// makes sure there are no buffered logs.
func (l *Buffer) CloseExpectEmpty(ctx context.Context, t *testing.T) bool {
	t.Helper()
	return l.closeExpectEmpty(ctx, t)
}

// closeExpectEmpty does what CloseExpectEmpty says it does, but takes a ter
// for testing the unhappy path.
func (l *Buffer) closeExpectEmpty(ctx context.Context, t ter) bool {
	t.Helper()
	l.close()
	return l.WithExpectEmpty().expect(ctx, t)
}

// IsClosed indicates if l.Close has been called.
func (l *Buffer) IsClosed() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return *l.closed
}

// Expect checks that the next log messages are equivalent to msgs
// (which may be no messages) and marks t as failed and prints a messages if
// not.
// Unless WithExpectUnordered was used, messages are expected to be in the
// order passed to Expect.
func (l *Buffer) Expect(ctx context.Context, t *testing.T, msgs ...Msg) bool {
	t.Helper()
	return l.expect(ctx, t, msgs...)
}

// expect does what Expect says it does, but takes a ter for testing the
// unhappy path.
func (l *Buffer) expect(ctx context.Context, t ter, msgs ...Msg) bool {
	t.Helper()
	ok := true
	/* Check for messages which should be there. */
	if l.expectUnordered {
		if !l.checkUnordered(ctx, t, msgs) {
			ok = false
		}
	} else {
		if !l.checkOrdered(ctx, t, msgs) {
			ok = false
		}
	}

	/* If we don't expect any more, make sure there's no more. */
	for l.expectEmpty && 0 != len(l.buf) {
		ok = false
		t.Error(leftoverLogMessageMessage{Msg: <-l.buf})
	}

	return ok
}

// checkOrdered checks that all of the messages in msgs are buffered in the
// order they're in in msgs.
func (l *Buffer) checkOrdered(ctx context.Context, t ter, msgs []Msg) bool {
	t.Helper()
	ok := true
	for i, wantMsg := range msgs {
		n := i + 1
		gotMsg, err := l.NextMessage(ctx)
		if nil != err {
			t.Error(errorWaitingForMessageMessage{
				Idx: n,
				Len: len(msgs),
				Err: err,
			})
			return false
		}
		if !wantMsg.Equal(gotMsg) {
			ok = false
			t.Error(incorrectLogMessageMessage{
				Idx:  n,
				Len:  len(msgs),
				Got:  gotMsg,
				Want: wantMsg,
			})
		}
	}
	return ok
}

// checkUnordered checks that all of the Msgs in msgs are buffered in any
// order.
func (l *Buffer) checkUnordered(ctx context.Context, t ter, msgs []Msg) bool {
	t.Helper()
	ok := true
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
			ok = false
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
			ok = false
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
		ok = false
		t.Error(unfoundLogMessageMessage{
			Idx:  i + 1,
			Len:  len(want),
			Want: *p,
		})
	}

	return ok
}

// NextMessage returns the next buffered message, according to l's
// configuration.
func (l *Buffer) NextMessage(ctx context.Context) (Msg, error) {
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
		return Msg{}, context.Cause(ctx)
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
		return Msg{}, context.Cause(ctx)
	case msg, ok := <-l.buf: /* Normal buffered message. */
		return ret(msg, ok)
	}
}

// WithExpectEmpty returns a copy of l configured such that Expect also notes a
// test failure if there were more buffered messages than passed to expect.
// Leftover buffered messages will be unbuffered and logged as test failures.
func (l *Buffer) WithExpectEmpty() *Buffer {
	n := l.Clone()
	n.expectEmpty = true
	return n
}

// WithNoWait returns a copy of l configured such that expect does not block
// waiting for as many log messages as it was passed.
func (l *Buffer) WithNoWait() *Buffer {
	n := l.Clone()
	n.expectNoWait = true
	return n
}

// WithExpectUnordered returns a copy of l configured such that Expect does not
// expect logged messages to be in order.
func (l *Buffer) WithExpectUnordered() *Buffer {
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
func (l *Buffer) TestEmptyAfterClose(t *testing.T) bool {
	t.Helper()
	return l.testEmptyAfterClose(t)
}

// testEmptyAfterClose does what TestEmptyAfterClose says it does, but takes a
// ter for testing the unhappy path.
func (l *Buffer) testEmptyAfterClose(t ter) bool {
	t.Helper()
	l.close()
	ok := true
	for {
		select {
		case m := <-l.cBuf:
			ok = false
			t.Error(messageSentAfterCloseMessage{Msg: m})
		default:
			/* No (more) messages. */
			return ok
		}
	}
}

// Strings unbuffers and returns the strings in l.
func (l *Buffer) Strings() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var ret []string
	/* f appends the strings in ch to ss. */
	f := func(ss []string, ch chan string) []string {
		for 0 < len(ch) {
			ss = append(ss, <-ch)
		}
		return ss
	}
	ret = f(ret, l.buf)
	ret = f(ret, l.cBuf)
	return slices.Clip(ret)
}
