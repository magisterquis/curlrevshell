package iobroker

/*
 * handle_input.go
 * Hook up shell input streams to ich
 * By J. Stuart McMurray
 * Created 20260628
 * Last Modified 20260804
 */

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/bidirpipe"
	"golang.org/x/net/websocket"
)

// HandleInput proxies from b's input channel (ich)  to the connected shell
// input stream in, ensuring that
// there's only one input stream connected,
// that the ID matches the output stream's ID, if applicable,
// and so on.
// in will not be closed before returning.
// HandleInput will add the id to sl.
//
// Cancel the context when in is no longer valid, e.g. when the connection
// disconnects.
// Returned errors will have already been logged.
// When the input from the user is ended or the conext is cancelled, nil is
// returned.
func (b *Broker) HandleInput(
	ctx context.Context,
	sl *slog.Logger,
	id string, /* Stream ID, to match output stream. */
	tag string, /* For messages to the user, shell's IP is good. */
	in io.Writer, /* To connected shell. */
) error {
	return handleCommon(
		ctx,
		b,
		sl,
		id,
		tag,
		b.isiCh,
		LVInput,
		&b.inStreamID,
		&b.outStreamID,
		in,
		proxyInput,
	)
}

// proxyInput proxies from isi.ich to in.
func proxyInput(
	ctx context.Context,
	sl *slog.Logger,
	isi inputStreamInfo,
	in io.Writer,
) error {
	/* Work out how to flush this thing. */
	var flush func() error
	if rw, ok := in.(http.ResponseWriter); ok {
		rc := http.NewResponseController(rw)
		flush = rc.Flush
	} else if f, ok := in.(interface{ Flush() error }); ok {
		flush = f.Flush
	} else if _, ok := in.(*websocket.Conn); ok {
		/* Doesn't need a flush. */
		flush = func() error { return nil }
	} else if testing.Testing() {
		switch in.(type) {
		case *io.PipeWriter, bidirpipe.Pipe:
			flush = func() error { return nil }
		}
	}
	if nil == flush {
		sl.Debug(
			LMUnknownInputType,
			LKType, reflect.TypeOf(in).String(),
		)
		flush = func() error { return nil }
	}

	/* send sends a line to in.  If send returns an error, it will have
	already been logged. */
	send := func(s string) error {
		/* Add back a missing newline. */
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		/* Send it forth. */
		n, err := io.WriteString(in, s)
		if 0 != n {
			sl.Info(LMShellIO, LKData, s[:n])
		}
		if nil == err {
			err = flush()
		}
		return err
	}

	/* Send the buffered line if we have one. */
	if nil != isi.bufferedLine {
		if err := send(*isi.bufferedLine); nil != err {
			return err
		}
	}

	/* Proxy input. */
	for {
		select {
		case l, ok := <-isi.ich: /* Got (maybe) input. */
			if !ok { /* ich is closed. */
				return nil
			}
			if err := send(l); nil != err {
				return err
			}
		case <-isi.brokerDone: /* Time to give up. */
			return nil
		case <-ctx.Done(): /* Time to give up. */
			return nil
		}
	}
}
