package server

/*
 * handlers.go
 * HTTP handlers
 * By J. Stuart McMurray
 * Created 20260810
 * Last Modified 20260812
 */

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/hsrv"
	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/lib/crsadapter"
)

// handler contains HTTP handler functions and supporting bits.
type handlers struct {
	sl     *slog.Logger
	aSock  string
	inMu   chan struct{}
	crsOut io.Writer
}

// mux sets up an http.ServeMux with two routes.  urlPrefix will be prepended
// to pathIn and pathOut.
func newHandlers(sl *slog.Logger, aSock string, crsOut io.Writer) handlers {
	/* Input mutex. */
	inMu := make(chan struct{}, 1)
	inMu <- struct{}{}

	return handlers{
		sl:     sl,
		aSock:  aSock,
		inMu:   inMu,
		crsOut: crsOut,
	}
}

// testCloseWriteConnContextKey is used to retrieve a request's [net.Conn]
// from its context so it can be closed before writing.
type testCloseWriteConnContextKey struct{}

// HandleIn handles input to the shell.
func (h handlers) handleIn(w http.ResponseWriter, r *http.Request) {
	sl := h.sl.With(iobroker.LKDirection, iobroker.LVInput)
	/* Prevent multiple input requests fighting over input. */
	select {
	case <-h.inMu:
		/* Got the mutex. */
	case <-r.Context().Done():
		return
	default:
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	defer func() { h.inMu <- struct{}{} }()

	/* Gonna need a bidirectional stream. */
	defer r.Body.Close()
	if err := hsrv.StartFullDuplex(w, r); nil != err {
		sl.Warn(LMDuplexError)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	/* Connect to curlrevshell. */
	crsIn, err := handshake(
		r.Context(),
		h.aSock,
		crsadapter.ShellStreamDirectionInput,
	)
	if nil != err {
		sl.Error(LMInputDialError, iobroker.LKError, err)
		/* StartFullDuplex already sent a 200. */
		return
	}
	defer crsIn.Close()

	/* Close the curlrevshell side on disconnect. */
	defer context.AfterFunc(r.Context(), func() { crsIn.Close() })()

	/* Proxy comms. */
	var (
		rc      = http.NewResponseController(w)
		scanner = bufio.NewScanner(crsIn)
		sent    int64
	)
	defer func() { logProxy(sl, sent, err) }()
	for scanner.Scan() {
		/* For testing, we may want to kill the connection before
		sending a string */
		if testing.Testing() {
			if c, ok := r.Context().Value(
				testCloseWriteConnContextKey{},
			).(net.Conn); ok {
				/* Should have CloseWrite. */
				c.(interface {
					CloseWrite() error
				}).CloseWrite()
			}
		}
		var n int
		/* Does w.Write ever return an error? */
		n, err = io.WriteString(w, scanner.Text()+"\n")
		if nil == err {
			err = rc.Flush()
		}
		if nil != err {
			return
		}
		sent += int64(n)
	}
	if err = scanner.Err(); errors.Is(err, net.ErrClosed) {
		/* We expect a read after close if the context expires. */
		err = nil
	}
}

// HandleOut handles output from the shell.
func (h handlers) handleOut(w http.ResponseWriter, r *http.Request) {
	n, err := io.Copy(h.crsOut, r.Body)
	logProxy(h.sl.With(iobroker.LKDirection, iobroker.LVOutput), n, err)
}

// logProxy logs the result of a proxy.
func logProxy(sl *slog.Logger, size int64, err error) {
	sl = sl.With(LKSize, size)
	f := sl.Debug
	if 0 != size {
		f = sl.Info
	}
	if nil != err {
		sl = sl.With(iobroker.LKError, err)
		f = sl.Warn
	}
	f(iobroker.LMShellIO)
}
