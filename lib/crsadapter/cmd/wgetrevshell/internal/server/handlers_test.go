package server

/*
 * handlers_test.go
 * Tests for handlers.go
 * By J. Stuart McMurray
 * Created 20260812
 * Last Modified 20260812
 */

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
)

// Do we return nicely if the context is closed from the get-go?
func TestHandlersHandleIn_ContextClosedAtStart(t *testing.T) {
	var (
		ctx, cancel = context.WithCancel(t.Context())
		tb, sl      = tlog.NewBuffer()

		h = newHandlers(sl, "", nil)
	)
	cancel()

	/* Grab the mutex so the handler sees the context. */
	<-h.inMu

	/* Do we see the cancelled context and return forthwith?  We'd better
	or we'll panic when we hit the nil writer. */
	h.handleIn(nil, httptest.NewRequestWithContext(ctx, "", "/", nil))

	tb.CloseExpectEmpty(t.Context(), t)
}

// Do we return nicely if someone else is connected?
func TestHandlersHandleIn_InputAlreadyConnected(t *testing.T) {
	var (
		rr     = httptest.NewRecorder()
		tb, sl = tlog.NewBuffer()

		h = newHandlers(sl, "", nil)
	)

	/* Grab the mutex so the handler sees someone else is connected. */
	<-h.inMu

	/* Shouldn't take long to notice someone has the mutex. */
	h.handleIn(
		rr,
		httptest.NewRequestWithContext(t.Context(), "", "/", nil),
	)

	/* Status correct? */
	if got, want := rr.Code, http.StatusTooManyRequests; got != want {
		t.Errorf("Incorrect status\n got: %d\nwant: %d", got, want)
	}

	tb.CloseExpectEmpty(t.Context(), t)
}

// Do we return nicely if we can't start duplex comms?
func TestHandlersHandleIn_NoDuplexComms(t *testing.T) {
	var (
		rr     = httptest.NewRecorder()
		tb, sl = tlog.NewBuffer()

		h = newHandlers(sl, "", nil)
	)

	/* This'll break if httptest.ResponseRecorder ever supports full
	duplex. */
	h.handleIn(
		rr,
		httptest.NewRequestWithContext(t.Context(), "", "/", nil),
	)

	/* Status correct? */
	if got, want := rr.Code, http.StatusBadRequest; got != want {
		t.Errorf("Incorrect status\n got: %d\nwant: %d", got, want)
	}

	tb.WithExpectEmpty().Expect(t.Context(), t,
		tlog.M.
			With(iobroker.LKDirection, iobroker.LVInput).
			Warn(LMDuplexError),
	)
}

// Do we return nicely if we can't contact curlrevshell?
func TestHandlersHandleIn_HandshakeError(t *testing.T) {
	var (
		tb, sl = tlog.NewBuffer()

		h = newHandlers(sl, t.TempDir(), nil)
	)

	/* Serve and try to GET.  Should fail quickly. */
	svr := httptest.NewServer(http.HandlerFunc(h.handleIn))
	defer svr.Close()
	res, err := svr.Client().Get(svr.URL)
	if nil != err {
		t.Fatalf("Error making GET request: %v", err)
	}
	defer res.Body.Close()

	/* Logs ok? */
	tb.WithExpectEmpty().Expect(t.Context(), t,
		tlog.M.
			With(iobroker.LKDirection, iobroker.LVInput).
			With(iobroker.LKError, fmt.Errorf(
				"dial: %w",
				&net.OpError{
					Op:  "dial",
					Net: "unix",
					Addr: &net.UnixAddr{
						Name: h.aSock,
						Net:  "unix",
					},
					Err: &os.SyscallError{
						Syscall: "connect",
						Err:     syscall.ENOTSOCK,
					},
				},
			)).
			Error(LMInputDialError),
	)
}

// Do we return nicely if we can't actually send input?
func TestHandlersHandleIn_WriteInputError(t *testing.T) {
	var (
		eg, ctx = ctxerrgroup.WithContext(t.Context())
		msg     = tlog.S("msg")
		tb, sl  = tlog.NewBuffer()
	)

	/* Fake curlrevshell. */
	l, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: filepath.Join(t.TempDir(), "l"),
		Net:  "unix",
	})
	if nil != err {
		t.Fatalf("Listen error: %v", err)
	}
	defer l.Close()

	/* Accept a connection, handshake, tell the client we've handshook,
	and send shell input. */
	eg.GoTag(ctx, "curlrevshell write", func(ctx context.Context) error {
		/* Get a client. */
		c, err := l.AcceptUnix()
		if nil != err {
			return err
		}
		defer c.Close()
		/* Don't care what it sends. */
		done := make(chan struct{})
		eg.GoTag(ctx, "discard", func(ctx context.Context) error {
			defer close(done)
			_, err := io.Copy(io.Discard, c)
			return err
		})
		/* Handshake. */
		if _, err := c.Write([]byte("{}")); nil != err {
			return fmt.Errorf("json reply: %v", err)
		}
		/* Send input. */
		if _, err := io.WriteString(c, msg+"\n"); nil != err {
			return fmt.Errorf("sending: %w", err)
		}

		/* Wait until we're done. */
		select {
		case <-done:
		case <-ctx.Done():
		}

		return nil
	})

	/* Server which proxies to l. */
	svr := httptest.NewUnstartedServer(nil)
	svr.Config = &http.Server{
		Handler: http.HandlerFunc(
			newHandlers(sl, l.Addr().String(), nil).handleIn,
		),
		ConnContext: func(
			ctx context.Context,
			c net.Conn,
		) context.Context {
			return context.WithValue(
				ctx,
				testCloseWriteConnContextKey{},
				c,
			)
		},
	}
	svr.Start()
	defer svr.Close()

	/* Connect as a shell getting input. */
	c, err := net.Dial(
		svr.Listener.Addr().Network(),
		svr.Listener.Addr().String(),
	)
	if nil != err {
		t.Fatalf("Error connecting to HTTP server: %v", err)
	}
	if _, err := fmt.Fprintf(
		c,
		"GET / HTTP/1.1\r\nHost: %s\r\n\r\n",
		svr.Listener.Addr().String(),
	); nil != err {
		t.Fatalf("Error sending GET request: %v", err)
	}
	if _, err := io.ReadAll(c); nil != err {
		t.Errorf("Error reading GET response: %v", err)
	}

	/* Logs ok? */
	tb.WithExpectEmpty().Expect(t.Context(), t,
		tlog.M.
			With(LKSize, 0).
			With(iobroker.LKError, &net.OpError{
				Op:     "write",
				Net:    "tcp",
				Addr:   c.LocalAddr(),
				Source: c.RemoteAddr(),
				Err: &os.SyscallError{
					Syscall: "write",
					Err:     syscall.EPIPE,
				},
			}).
			With(iobroker.LKDirection, iobroker.LVInput).
			Warn(iobroker.LMShellIO),
	)

	/* All done? */
	if err := eg.Wait(); nil != err {
		t.Errorf("Server error: %v", err)
	}

}
