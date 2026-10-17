package tlog

/*
 * tlog_fails_test.go
 * Tests for failed tests for tlog.go
 * By J. Stuart McMurray
 * Created 20260215
 * Last Modified 20260613
 */

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
)

// newMockLog returns the bits needed to test failed tests cases with a
// Buffer.
func newFailBuffer() (*slog.Logger, *Buffer, *mockT) {
	var (
		lb, sl = NewBuffer()
		mt     = newMockT()
	)
	return sl, lb, mt
}

// Does CloseExpectEmpty handle not empty properly?
func TestBufferCloseExpectEmpty_NotEmpty(t *testing.T) {
	var (
		msg        = "kittens"
		sl, lb, mt = newFailBuffer()
	)
	sl.Info(msg)
	ok := lb.closeExpectEmpty(t.Context(), mt)
	if ok {
		t.Errorf("closeExpectEmpty returned false, but shouldn't have")
	}
	mt.Expect(t, []mockTMessage{
		newMockTMessage(
			false,
			leftoverLogMessageMessage{Msg: M.Info(msg).String()},
		),
	})
	if ok {
		t.Errorf("Expect returned false, but shouldn't have")
	}
}

// Does Expect handle incorrect messages?
func TestBufferExpect_IncorrectMessages(t *testing.T) {
	var (
		sl, lb, mt = newFailBuffer()
		wantMsg    = M.Warn("m2")
		gotMsg     = "incorrect-m2"
		wantMsgs   []Msg
	)
	/* Log a correct message. */
	sl.Info("m1")
	wantMsgs = append(wantMsgs, M.Info("m1"))
	/* Log an incorrect message. */
	sl.Warn(gotMsg)
	wantMsgs = append(wantMsgs, wantMsg)
	/* Log another correct message. */
	sl.Debug("m3")
	wantMsgs = append(wantMsgs, M.Debug("m3"))

	/* Do we find the incorrect message? */
	ok := lb.expect(t.Context(), mt, wantMsgs...)
	if ok {
		t.Errorf("expect returned true, but shouldn't have")
	}
	mt.Expect(t, []mockTMessage{newMockTMessage(
		false,
		incorrectLogMessageMessage{
			Idx:  2,
			Len:  len(wantMsgs),
			Got:  M.Warn(gotMsg),
			Want: wantMsg,
		},
	)})
}

// Does TestEmptyAfterClose catch an extra message?
func TestBufferTestEmptyAfterClose_NotEmpty(t *testing.T) {
	var (
		msg        = "kittens"
		sl, lb, mt = newFailBuffer()
	)
	if err := lb.Close(); nil != err {
		t.Errorf("Error closing Buffer: %s", err)
	}
	sl.Info(msg)
	ok := lb.testEmptyAfterClose(mt)
	if ok {
		t.Errorf(
			"testEmptyAfterClose returned true, " +
				"but shouldn't have",
		)
	}
	mt.Expect(t, []mockTMessage{
		newMockTMessage(
			false,
			messageSentAfterCloseMessage{Msg: M.Info(msg).String()},
		),
	})
}

// Does WithExpectEmpty work when the Buffer's not empty?
func TestBufferWithExpectEmpty_NotEmpty(t *testing.T) {
	var (
		m1         = "kittens"
		m2         = "moose"
		sl, lb, mt = newFailBuffer()
	)
	sl.Info(m1)
	sl.Error(m2)
	ok := lb.WithExpectEmpty().expect(t.Context(), mt, M.Info(m1))
	if ok {
		t.Errorf("expect returned true, but shouldn't have")
	}
	mt.Expect(t, []mockTMessage{
		newMockTMessage(
			false,
			leftoverLogMessageMessage{Msg: M.Error(m2).String()},
		),
	})
}

// Can we detect an incorrect message out of order?
func TestBufferWithExpectUnordered_Incorrect(t *testing.T) {
	var (
		sl, lb, mt = newFailBuffer()
		wantMsg    = M.Warn("m2")
		gotMsg     = "incorrect-m2"
		wantMsgs   []Msg
	)
	/* Log messages, in order. */
	sl.Info("m1")
	sl.Warn(gotMsg)
	sl.Debug("m3")

	/* Work out what we expect, out of order. */
	wantMsgs = append(wantMsgs, M.Debug("m3"))
	wantMsgs = append(wantMsgs, M.Info("m1"))
	wantMsgs = append(wantMsgs, wantMsg)

	/* Do we find the incorrect message? */
	ok := lb.WithExpectUnordered().expect(t.Context(), mt, wantMsgs...)
	if ok {
		t.Errorf("expect returned true, but shouldn't have")
	}
	mt.Expect(t, []mockTMessage{
		newMockTMessage(
			false,
			unexpectedLogMessageMessage{Msg: M.Warn(gotMsg)},
		),
		newMockTMessage(
			false,
			unfoundLogMessageMessage{
				Idx:  3,
				Len:  len(wantMsgs),
				Want: wantMsg,
			},
		),
	})
}

// Can we handle not having as many messages as we expect?
func TestBufferWithNoWait_MissingMessages(t *testing.T) {
	var (
		sl, lb, mt = newFailBuffer()
		wantMsgs   []Msg
	)
	/* Log messages we'll expect. */
	sl.Info("m1")
	wantMsgs = append(wantMsgs, M.Info("m1"))
	sl.Info("m2")
	wantMsgs = append(wantMsgs, M.Info("m2"))
	/* Also expect a message we don't have. */
	wantMsgs = append(wantMsgs, M.Error("m3"))
	wantMsgs = append(wantMsgs, M.Error("m4"))

	/* Do we get an error or a panic? */
	ok := lb.WithNoWait().expect(t.Context(), mt, wantMsgs...)
	if ok {
		t.Errorf("expect returned true, but shouldn't have")
	}
	mt.Expect(t, []mockTMessage{newMockTMessage(
		false,
		errorWaitingForMessageMessage{
			Idx: 3,
			Len: len(wantMsgs),
			Err: ErrBufferEmpty,
		},
	)})
}

// Do we get the right error checking unordered messages with an empty buffer
// and a done context?
func TestBufferCheckUnordered_EmptyBufferAndExpiredContext(t *testing.T) {
	var (
		_, lb, mt   = newFailBuffer()
		cause       = fmt.Errorf("%s", S("cancel-cause"))
		ctx, cancel = context.WithCancelCause(t.Context())
		msg         = M.Info(S("msg"))
	)

	/* Cancel the context, so getting the next message ends early. */
	cancel(cause)

	/* Shouldn't get a message. */
	ok := lb.WithExpectUnordered().expect(ctx, mt, msg)
	if ok {
		t.Errorf("ExpectReturned true, but shouldn't have")
	}

	/* Should get a test fail. */
	mt.Expect(t, []mockTMessage{
		newMockTMessage(false, errorWaitingForMessageMessage{
			Idx: 1,
			Len: 1,
			Err: cause,
		}),
		newMockTMessage(false, unfoundLogMessageMessage{
			Idx:  1,
			Len:  1,
			Want: msg,
		}),
	})
}

// Can we detect unordred messages after a .WithExpectUnordered() ?
func TestBufferWithExpectUnordered_CloneAffectsParent(t *testing.T) {
	var (
		sl, lb, mt = newFailBuffer()
		m1         = S("m1")
		m2         = S("m2")
		m3         = S("m3")
		m4         = S("m4")
	)

	/* Do an unordered check. */
	sl.Info(m1)
	sl.Info(m2)
	lb.WithExpectUnordered().Expect(t.Context(), t,
		M.Info(m2),
		M.Info(m1),
	)

	/* Now an ordered check, should fail */
	sl.Info(m3)
	sl.Info(m4)
	lb.expect(t.Context(), mt,
		M.Info(m4),
		M.Info(m3),
	)

	mt.Expect(t, []mockTMessage{
		newMockTMessage(false, incorrectLogMessageMessage{
			Idx:  1,
			Len:  2,
			Got:  M.Info(m3),
			Want: M.Info(m4),
		}),
		newMockTMessage(false, incorrectLogMessageMessage{
			Idx:  2,
			Len:  2,
			Got:  M.Info(m4),
			Want: M.Info(m3),
		}),
	})

}
