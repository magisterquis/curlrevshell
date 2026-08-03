package iobroker

/*
 * run_input_test.go
 * Tests for run_input.go
 * By J. Stuart McMurray
 * Created 20260721
 * Last Modified 20260801
 */

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// Can we buffer a line before input connects?
func TestBrokerRunInput_BufferedLine(t *testing.T) {
	synctest.Test(t, testBrokerRunInputBufferedLine)
}
func testBrokerRunInputBufferedLine(t *testing.T) {
	var (
		bCh       = make(chan string, testChanLen)
		id        = tlog.S("id")
		pr, pw    = io.Pipe()
		tag       = tlog.S("tag")
		postLines = []string{ /* After input connects. */
			tlog.S("a-line-1"),
			tlog.S("a-line-2"),
		}
		preLines = []string{ /* Before input connects. */
			tlog.S("b-line-1"),
			tlog.S("b-line-2"),
		}
		b, ich, och, done, tb, sl = newTestBroker(
			t,
			t.Context(),
			&testBrokerConfig{wantErr: ErrInputClosed},
		)
	)
	t.Cleanup(func() {
		pr.Close()
		pw.Close()
	})

	/* Buffer some lines before input connects. */
	for _, l := range preLines {
		ich <- l
	}

	/* Get a warning? */
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: WarnColor,
		Line:  SMBufferingInput,
	})

	/* Connect up an input. */
	eg, ctx := ctxerrgroup.WithContext(t.Context())
	eg.GoTag(ctx, "input_stream", func(ctx context.Context) error {
		defer pw.Close()
		return b.HandleInput(ctx, sl, id, tag, pw)
	})
	eg.GoTag(ctx, "shell_input_buffer", func(ctx context.Context) error {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			select {
			case bCh <- scanner.Text():
			case <-ctx.Done():
				return nil
			}
		}
		return scanner.Err()
	})

	/* Connection make it? */
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: LogColor,
		Line:  taggedString(tag, "Input connected: ID %s", id),
	})

	/* Get the buffered lines? */
	checkGotLines := func(ls []string) {
		for _, want := range ls {
			if got := <-bCh; got != want {
				t.Errorf(
					"Incorrect input line\n got: %q\nwant: %q",
					got,
					want,
				)
			}
		}
	}
	checkGotLines(preLines)

	/* Can we still send more lines? */
	eg.GoTag(ctx, "post_lines_send", func(ctx context.Context) error {
		for _, l := range postLines {
			select {
			case ich <- l:
			case <-ctx.Done():
				return nil
			}
		}
		return nil
	})
	checkGotLines(postLines)

	/* All is good, stop things. */
	close(ich)
	<-done

	/* Everything run happily? */
	if err := eg.Wait(); nil != err {
		t.Fatalf("Unexpected error: %v", err)
	}

	/* Rest of the output look ok? */
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ErrColor,
		Line:  taggedString(tag, "Input connection closed"),
	})

	/* Logs look ok? */
	m := tlog.M.
		With(LKDirection, LVInput).
		With(LKID, id)
	tb.Expect(t.Context(), t,
		m.
			Info(LMNewConnection),
	)
	for _, ls := range [][]string{preLines, postLines} {
		for _, l := range ls {
			tb.Expect(t.Context(), t,
				m.
					With(LKData, l+"\n").
					Info(LMShellIO),
			)
		}
	}
	tb.Expect(t.Context(), t,
		m.
			Info(LMConnectionClosed),
	)
}

// Do we discard buffered but unsent input?
func TestBrokerRunInput_UnsentBuffer(t *testing.T) {
	synctest.Test(t, testBrokerRunInputUnsentBuffer)
}
func testBrokerRunInputUnsentBuffer(t *testing.T) {
	var (
		id1                    = tlog.S("id1")
		id2                    = tlog.S("id2")
		pr1, pw1               = io.Pipe()
		pr2, pw2               = io.Pipe()
		tag1                   = tlog.S("tag1")
		tag2                   = tlog.S("tag2")
		msg1a                  = tlog.S("msg1a")
		msg1b                  = tlog.S("msg1b")
		msg2                   = tlog.S("msg2")
		b, ich, och, _, tb, sl = newTestBroker(
			t,
			t.Context(),
			&testBrokerConfig{wantErr: ErrInputClosed},
		)
	)
	t.Cleanup(func() {
		pr1.Close()
		pw1.Close()
		pr2.Close()
		pw2.Close()
	})

	/* Buffer some input. */
	eg, ctx := ctxerrgroup.WithContext(t.Context())
	eg.GoTag(ctx, "first_input", func(ctx context.Context) error {
		/* This message will attempt to be sent to a blocked pw1. */
		ich <- msg1a
		/* This message will (should) stay buffered. */
		ich <- msg1b
		/* This should drain the buffer. */
		return pr1.Close()
	})
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: WarnColor,
		Line:  SMBufferingInput,
	})
	/* Connect, but don't actually read the input. */
	eg.GoTag(ctx, "first_conn", func(ctx context.Context) error {
		err := b.HandleInput(ctx, sl, id1, tag1, pw1)
		if errors.Is(err, io.ErrClosedPipe) { /* Expected. */
			err = nil
		}
		return err
	})
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: LogColor,
		Line:  taggedString(tag1, "Input connected: ID %s", id1),
	})

	/* Should disconnect quickly. */
	if err := eg.Wait(); nil != err {
		t.Fatalf("Error during first connection: %v", err)
	}

	/* Output look correct? */
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ErrColor,
		Line:  taggedString(tag1, "Input connection closed"),
	})

	/* Logs look correct? */
	m := tlog.M.
		With(LKDirection, LVInput).
		With(LKID, id1)
	tb.Expect(t.Context(), t,
		m.
			Info(LMNewConnection),
		m.
			With(LKError, io.ErrClosedPipe).
			Info(LMConnectionClosed),
	)

	/* Connect again and send another line, to make sure we flushed the
	last one properly. */
	eg, ctx = ctxerrgroup.WithContext(t.Context())
	gotBuf := new(bytes.Buffer)
	eg.GoTag(ctx, "second_conn", func(ctx context.Context) error {
		defer pw2.Close()
		return b.HandleInput(ctx, sl, id2, tag2, pw2)
	})
	eg.GoTag(ctx, "input_buffer", func(ctx context.Context) error {
		_, err := io.Copy(gotBuf, pr2)
		return err
	})
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: LogColor,
		Line:  taggedString(tag2, "Input connected: ID %s", id2),
	})
	eg.GoTag(ctx, "second_input", func(ctx context.Context) error {
		/* This message will attempt to be sent to a blocked pw1. */
		ich <- msg2
		close(ich)
		synctest.Wait()
		return nil
	})
	if err := eg.Wait(); nil != err {
		t.Fatalf("Error during second connection: %v", err)
	}

	/* Output look correct? */
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ErrColor,
		Line:  taggedString(tag2, "Input connection closed"),
	})

	/* Logs look correct? */
	m = tlog.M.
		With(LKDirection, LVInput).
		With(LKID, id2)
	tb.Expect(t.Context(), t,
		m.
			Info(LMNewConnection),
		m.
			With(LKData, msg2+"\n").
			Info(LMShellIO),
		m.
			Info(LMConnectionClosed),
	)
}

// Does the broker properly not do anything weird if we stop it while an input
// line is buffered?
func TestBroker_ContextDoneAfterBuffer(t *testing.T) {
	var (
		msg         = tlog.S("msg")
		ctx, cancel = context.WithCancel(t.Context())

		_, ich, och, _, _, _ = newTestBroker(t, ctx, nil)
	)
	defer cancel()

	/* Buffer a message. */
	ich <- msg
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: WarnColor,
		Line:  SMBufferingInput,
	})
	/* Cancel the context. */
	cancel()
}

// Can we handle ich closing before anything else happens?
func TestBrokerRunInput_InputClosedBeforeConnection(t *testing.T) {
	synctest.Test(t, testBrokerRunInputInputClosedBeforeConnection)
}
func testBrokerRunInputInputClosedBeforeConnection(t *testing.T) {
	_, ich, _, done, _, _ := newTestBroker(
		t,
		t.Context(),
		&testBrokerConfig{wantErr: ErrInputClosed},
	)
	synctest.Wait()
	close(ich)
	<-done
}
