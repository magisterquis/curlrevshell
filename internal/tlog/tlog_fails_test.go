package tlog

/*
 * tlog_fails_test.go
 * Tests for failed tests for tlog.go
 * By J. Stuart McMurray
 * Created 20260215
 * Last Modified 20260216
 */

import (
	"log/slog"
	"testing"
)

// newMockLog returns the bits needed to test failed tests cases with a
// LogBuffer.
func newFailBuffer() (*slog.Logger, *LogBuffer, *mockT) {
	var (
		lb, sl = NewLogBuffer()
		mt     = newMockT()
	)
	return sl, lb, mt
}

// Does CloseExpectEmpty handle not empty properly?
func TestLogBufferCloseExpectEmpty_NotEmpty(t *testing.T) {
	var (
		msg        = "kittens"
		sl, lb, mt = newFailBuffer()
	)
	sl.Info(msg)
	lb.closeExpectEmpty(t.Context(), mt)
	mt.Expect(t, []mockTMessage{
		newMockTMessage(
			false,
			leftoverLogMessageMessage{Msg: M.Info(msg).String()},
		),
	})
}

// Does Expect handle incorrect messages?
func TestLogBufferExpect_IncorrectMessages(t *testing.T) {
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
	lb.expect(t.Context(), mt, wantMsgs...)
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
func TestLogBufferTestEmptyAfterClose_NotEmpty(t *testing.T) {
	var (
		msg        = "kittens"
		sl, lb, mt = newFailBuffer()
	)
	if err := lb.Close(); nil != err {
		t.Errorf("Error closing LogBuffer: %s", err)
	}
	sl.Info(msg)
	lb.testEmptyAfterClose(mt)
	mt.Expect(t, []mockTMessage{
		newMockTMessage(
			false,
			messageSentAfterCloseMessage{Msg: M.Info(msg).String()},
		),
	})
}

// Does WithExpectEmpty work when the LogBuffer's not empty?
func TestLogBufferWithExpectEmpty_NotEmpty(t *testing.T) {
	var (
		m1         = "kittens"
		m2         = "moose"
		sl, lb, mt = newFailBuffer()
	)
	sl.Info(m1)
	sl.Error(m2)
	lb.WithExpectEmpty().expect(t.Context(), mt, M.Info(m1))
	mt.Expect(t, []mockTMessage{
		newMockTMessage(
			false,
			leftoverLogMessageMessage{Msg: M.Error(m2).String()},
		),
	})
}

// Can we detect an incorrect message out of order?
func TestLogBufferWithExpectUnordered_Incorrect(t *testing.T) {
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
	lb.WithExpectUnordered().expect(t.Context(), mt, wantMsgs...)
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
func TestLogBufferWithNoWait_MissingMessages(t *testing.T) {
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
	lb.WithNoWait().expect(t.Context(), mt, wantMsgs...)
	mt.Expect(t, []mockTMessage{newMockTMessage(
		false,
		errorWaitingForMessageMessage{
			Idx: 3,
			Len: len(wantMsgs),
			Err: ErrBufferEmpty,
		},
	)})
}
