package iobroker

/*
 * handle_output_test.go
 * Tests for handle_output.go
 * By J. Stuart McMurray
 * Created 20260721
 * Last Modified 20260814
 */

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/lib/opshell"
	"github.com/magisterquis/curlrevshell/lib/tlog"
)

// Can we stop the output stream by stopping the broker while waiting on a
// read?
func TestBrokerHandleOutput_StopRead(t *testing.T) {
	t.Run("stop_broker", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			testBrokerHandleOutputStopRead(t, true)
		})
	})
	t.Run("stop_handler", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			testBrokerHandleOutputStopRead(t, false)
		})
	})
}

// testBrokerHandleOutputStopRead stops an output stream while waiting on a
// read by either stopping the broker or cancelling the handler's context.
func testBrokerHandleOutputStopRead(t *testing.T, stopBroker bool) {
	var (
		bCtx, bCancel = context.WithCancel(t.Context())
		hCtx, hCancel = context.WithCancel(t.Context())

		b, _, och, done, tb, sl = newTestBroker(t, bCtx, nil)
		ech                     = make(chan error, 1)
		id, tag                 = tlog.S("id"), tlog.S("tag")
		pr, _                   = io.Pipe()
	)
	defer bCancel()
	defer hCancel()

	/* Start handling output. */
	go func() { ech <- b.HandleOutput(hCtx, sl, id, tag, pr) }()
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: LogColor,
		Line:  fmt.Sprintf("[%s] Output connected: ID %s", tag, id),
	})
	m := tlog.M.
		With(LKDirection, LVOutput).
		With(LKID, id)
	tb.Expect(t.Context(), t,
		m.
			Info(LMNewConnection),
	)

	/* Stop the broker or the handler. */
	if stopBroker {
		bCancel()
		<-done
	} else {
		hCancel()
	}

	/* Stop ok? */
	if err := <-ech; nil != err {
		t.Errorf("Unexpected error from HandleOutput: %v", err)
	}
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ErrColor,
		Line:  taggedString(tag, "Output connection closed"),
	})
	tb.Expect(t.Context(), t,
		m.
			Info(LMConnectionClosed),
	)

}

// Can we stop the output stream by stopping the broker while waiting on a
// send?
func TestBrokerHandleOutput_StopSend(t *testing.T) {
	t.Run("stop_broker", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			testBrokerHandleOutputStopSend(t, true)
		})
	})
	t.Run("stop_handler", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			testBrokerHandleOutputStopSend(t, false)
		})
	})
}

// testBrokerHandleOutputStopSend stops an output stream while waiting on a
// send by either stopping the broker or cancelling the handler's context.
func testBrokerHandleOutputStopSend(t *testing.T, stopBroker bool) {
	var (
		bCtx, bCancel = context.WithCancel(t.Context())

		b, _, och, done, tb, sl = newTestBroker(t, bCtx, nil)
		ech                     = make(chan error, 1)
		id, tag                 = tlog.S("id"), tlog.S("tag")
		pr, pw                  = io.Pipe()
		wantLines               = make([]string, 0, cap(och))
	)
	defer bCancel()

	/* Start handling output. */
	hoCtx, hoCancel := context.WithCancel(t.Context())
	defer hoCancel()
	go func() { ech <- b.HandleOutput(hoCtx, sl, id, tag, pr) }()
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: LogColor,
		Line:  fmt.Sprintf("[%s] Output connected: ID %s", tag, id),
	})
	m := tlog.M.
		With(LKDirection, LVOutput).
		With(LKID, id)
	tb.Expect(t.Context(), t,
		m.
			Info(LMNewConnection),
	)

	/* Fill up och so we block on a write. */
	n := 0
	for len(och) < cap(och) {
		/* Send a chunk. */
		msg := tlog.S("msg-" + strconv.Itoa(n))
		wantLines = append(wantLines, msg)
		if _, err := io.WriteString(pw, msg); nil != err {
			t.Fatalf("Error writing to pipe: %v", err)
		}
		n++
		/* Let this one get buffered. */
		synctest.Wait()
	}
	/* Should be full now. */
	if l, c := len(och), cap(och); c != l {
		t.Fatalf("Output channel not full\nlen: %d\ncap: %d", l, c)
	}

	/* Send just a bit more, to make sure we block on write. */
	var (
		wech = make(chan error, 1)
		lMsg = tlog.S("last-message")
		bMsg = tlog.S("blocked-message")
	)
	go func() {
		for _, msg := range []string{lMsg, bMsg} {
			if _, err := io.WriteString(pw, msg); nil != err {
				wech <- fmt.Errorf("writing %s: %w", msg, err)
			}
		}
		wech <- nil
	}()

	/* Let everything settle so we're blocked trying to send to och. */
	synctest.Wait()

	/* Stop the broker or the handler. */
	if stopBroker {
		bCancel()
	} else {
		hoCancel()
	}

	/* Should get loads of output.  Also need to free up space in och. */
	var (
		wantCLines = make([]opshell.CLine, len(wantLines))
		wantLogs   = make([]tlog.Msg, len(wantLines))
	)
	for i, want := range wantLines {
		wantCLines[i] = opshell.CLine{
			Line:  want,
			Plain: true,
		}
		wantLogs[i] = m.
			With(LKData, want).
			Info(LMShellIO)
	}
	opshell.ExpectShellMessages(t, och, wantCLines...)
	tb.Expect(t.Context(), t, wantLogs...)
	/* Last line will have been logged, at least. */
	tb.Expect(t.Context(), t,
		m.
			With(LKData, lMsg).
			Info(LMShellIO),
		m.
			With(LKData, bMsg).
			Info(LMShellIO),
	)

	/* Wait until everything's stopped. */
	if err := <-wech; nil != err {
		t.Errorf("Error writing last message: %v", err)
	}
	if err := <-ech; nil != err {
		t.Errorf("Unexpected error from HandleOutput: %v", err)
	}
	bCancel()
	<-done
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ErrColor,
		Line:  taggedString(tag, "Output connection closed"),
	})
	tb.Expect(t.Context(), t,
		m.
			Info(LMConnectionClosed),
	)
}
