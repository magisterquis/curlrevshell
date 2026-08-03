package iobroker

/*
 * handle_input_test.go
 * Tests for handle_input.go
 * By J. Stuart McMurray
 * Created 20260721
 * Last Modified 20260801
 */

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// What happens if we send input to a closed channel?
func TestBrokerHandleInput_AfterContextDone(t *testing.T) {
	var (
		ctx, cancel = context.WithCancel(t.Context())
		ech         = make(chan error, 1)
		id          = tlog.S("id")
		msg         = tlog.S("msg")
		pr, pw      = io.Pipe()
		tag         = tlog.S("tag")
		wantErr     = io.ErrClosedPipe

		b, ich, och, _, tb, sl = newTestBroker(t, ctx, nil)
	)
	defer cancel()

	/* Connect a closed input stream. */
	pr.Close()
	go func() { ech <- b.HandleInput(t.Context(), sl, id, tag, pw) }()
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: LogColor,
		Line:  taggedString(tag, "Input connected: ID %s", id),
	})

	/* Send some input, should fail. */
	ich <- msg
	if err := <-ech; !errors.Is(err, wantErr) {
		t.Errorf(
			"HandleInput returned incorrect error\n"+
				" got: %v\n"+
				"want: %v",
			err,
			wantErr,
		)
	}

	/* Tidy up. */
	cancel()
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ErrColor,
		Line:  taggedString(tag, "Input connection closed"),
	})
	m := tlog.M.
		With(LKDirection, LVInput).
		With(LKID, id)
	tb.Expect(t.Context(), t,
		m.
			Info(LMNewConnection),
		m.
			With(LKError, wantErr).
			Info(LMConnectionClosed),
	)

}

// Can we proxy input to an http.ResponseWriter?
func TestProxyInput_FlusherResponseWriter(t *testing.T) {
	var (
		tb, sl     = tlog.NewBuffer()
		rr         = httptest.NewRecorder()
		buf        = new(bytes.Buffer)
		ich        = make(chan string, testChanLen)
		brokerDone = make(chan struct{})
		ech        = make(chan error, 1)
		msg        = tlog.S("msg")
	)

	/* Cause the handler to handle. */
	rr.Body = buf
	go func() {
		ech <- proxyInput(t.Context(), sl, inputStreamInfo{
			ich:        ich,
			brokerDone: brokerDone,
		}, rr)
	}()

	/* Send it a message, make sure we get it. */
	ich <- msg
	tb.Expect(t.Context(), t,
		tlog.M.
			With(LKData, msg+"\n").
			Info(LMShellIO),
	)

	/* Finish up. */
	close(ich)
	if err := <-ech; nil != err {
		t.Errorf("Error: %v", err)
	}

	/* Shouldn't have any more logs. */
	tb.CloseExpectEmpty(t.Context(), t)

	/* Make sure we got the message and that flush was called. */
	if !rr.Flushed {
		t.Errorf("ResponseRecorder not flushed")
	}
	if got, want := buf.String(), msg+"\n"; got != want {
		t.Errorf(
			"Buffered string incorrect\n got: %q\nwant: %q",
			got,
			want,
		)
	}
}

// Can we proxy input to an http.ResponseWriter?
func TestProxyInput_FlusherInterface(t *testing.T) {
	var (
		tb, sl     = tlog.NewBuffer()
		fb         = &testFlusherBuffer{Buffer: new(bytes.Buffer)}
		ich        = make(chan string, testChanLen)
		brokerDone = make(chan struct{})
		ech        = make(chan error, 1)
		msg        = tlog.S("msg")
	)

	/* Cause the handler to handle. */
	go func() {
		ech <- proxyInput(t.Context(), sl, inputStreamInfo{
			ich:        ich,
			brokerDone: brokerDone,
		}, fb)
	}()

	/* Send it a message, make sure we get it. */
	ich <- msg
	tb.Expect(t.Context(), t,
		tlog.M.
			With(LKData, msg+"\n").
			Info(LMShellIO),
	)

	/* Finish up. */
	close(ich)
	if err := <-ech; nil != err {
		t.Errorf("Error: %v", err)
	}

	/* Shouldn't have any more logs. */
	tb.CloseExpectEmpty(t.Context(), t)

	/* Make sure we got the message and that flush was called. */
	if !fb.flushed {
		t.Errorf("ResponseRecorder not flushed")
	}
	if got, want := fb.String(), msg+"\n"; got != want {
		t.Errorf(
			"Buffered string incorrect\n got: %q\nwant: %q",
			got,
			want,
		)
	}

}

// Do we get a warning if the writer doesn't have a flush method?
func TestProxyInput_NotAFlusher(t *testing.T) {
	var (
		tb, sl     = tlog.NewBuffer()
		cr, cw     = net.Pipe()
		buf        = new(bytes.Buffer)
		ich        = make(chan string, testChanLen)
		brokerDone = make(chan struct{})
		msg        = tlog.S("msg")
		wg         sync.WaitGroup
	)

	/* This test relies on the writer having no Flush method. */
	if _, ok := any(cw).(interface{ Flush() error }); ok {
		t.Fatalf("%T has a Flush method, need a Flushless type", cw)
	}

	/* Cause the handler to handle. */
	wg.Go(func() {
		defer cw.Close()
		if err := proxyInput(t.Context(), sl, inputStreamInfo{
			ich:        ich,
			brokerDone: brokerDone,
		}, cw); nil != err {
			t.Errorf("proxyInput returned error: %v", err)
		}
	})
	wg.Go(func() {
		if _, err := io.Copy(buf, cr); nil != err {
			t.Errorf("Error copying from pipe: %v", err)
		}
	})

	/* Send it a message, make sure we get it. */
	ich <- msg
	tb.Expect(t.Context(), t,
		tlog.M.
			With(LKType, reflect.TypeOf(cw).String()).
			Debug(LMUnknownInputType),
		tlog.M.
			With(LKData, msg+"\n").
			Info(LMShellIO),
	)

	/* Finish up. */
	close(ich)
	wg.Wait()

	/* Shouldn't have any more logs. */
	tb.CloseExpectEmpty(t.Context(), t)

	/* Make sure we got the message anyways. */
	if got, want := buf.String(), msg+"\n"; got != want {
		t.Errorf(
			"Buffered string incorrect\n got: %q\nwant: %q",
			got,
			want,
		)
	}

}
