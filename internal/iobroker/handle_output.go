package iobroker

/*
 * handle_output.go
 * Hook up shell output streams to ich
 * By J. Stuart McMurray
 * Created 20260628
 * Last Modified 20260722
 */

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// HandleOutput proxies from the connected shell output stream out to och,
// ensuring that there's only one output stream connected,
// that the ID matches the input stream's ID, if applicable,
// and so on.
// out.Close may be called before HandleOutput returns.
// If out.Read returns an error other than [io.EOF] or an error occuring after
// the context is done or b is finished it will be returned.
func (b *Broker) HandleOutput(
	ctx context.Context,
	sl *slog.Logger,
	id string, /* Stream ID, to match input stream. */
	tag string, /* For messages to the user, shell's IP is good. */
	out io.ReadCloser, /* From connected shell. */
) error {
	return handleCommon(
		ctx,
		b,
		sl,
		id,
		tag,
		b.osiCh,
		LVOutput,
		&b.outStreamID,
		&b.inStreamID,
		out,
		proxyOutput,
	)
}

// proxyInput proxies from out osi.och.
func proxyOutput( /* Shuffles between channel and stream. */
	ctx context.Context,
	sl *slog.Logger,
	osi outputStreamInfo,
	out io.ReadCloser,
) error {
	/* Send output to och. */
	var (
		bch  = make(chan []byte, 1) /* Output lines. */
		pool = sync.Pool{New: func() any {
			b := make([]byte, os.Getpagesize())
			return &b
		}}
		rerr error /* Read error, valid after bch is closed. */
	)
	go func() {
		defer close(bch)
		for {
			/* Get a new buffer. */
			b := *(pool.Get().(*[]byte))
			b = b[:cap(b)]
			/* Grab a chunk of output. */
			n, err := out.Read(b)
			/* Send back what we got. */
			if 0 != n {
				b = b[:n]
				sl.Info(LMShellIO, LKData, string(b))
				bch <- b
			}
			/* Give up on error. */
			if nil != err {
				/* Though, EOF isn't a real error. */
				if !errors.Is(err, io.EOF) &&
					!errors.Is(err, io.ErrUnexpectedEOF) {
					rerr = err
				}
				return
			}
		}
	}()

	/* proxyChunk proxies a chunk read from bch (i.e. from out) to
	osi.och.  It returns true if the proxy went as planned or false if
	we should give up. */
	proxyChunk := func() bool {
		/* Buffer we may or may not get from bch, which we'll have to
		stick back in the pool if we get. */
		var b []byte
		defer func() {
			if nil != b {
				pool.Put(&b)
			}
		}()

		/* Try to get a chunk. */
		var ok bool
		select {
		case b, ok = <-bch: /* Got (maybe) output. */
			if !ok { /* out was closed. */
				return false
			}
		case <-osi.brokerDone: /* Time to give up. */
			return false
		case <-ctx.Done(): /* Time to give up. */
			return false
		}

		/* Try to send to och. */
		select {
		case osi.och <- opshell.CLine{
			Line:  string(b),
			Plain: true,
		}:
			return true
		case <-osi.brokerDone: /* Time to give up. */
			return false
		case <-ctx.Done(): /* Time to give up. */
			return false
		}

	}

	/* Proxy output. */
	for proxyChunk() {
		/* Keep going. */
	}

	/* Done proxying.  Close the input stream, drain the chunk channel, and
	tell everybody what's going on. */
	err := rerr /* Before we close out. */
	out.Close()
	for range bch {
		/* Drainage. */
	}
	return err
}
