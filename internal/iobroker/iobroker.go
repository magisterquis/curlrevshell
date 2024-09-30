// Package iobroker converts io.Read/Writers into opshell channels.
package iobroker

/*
 * iobroker.go
 * Turn stream I/O into shell-friendly I/O
 * By J. Stuart McMurray
 * Created 20240919
 * Last Modified 20240930
 */

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/magisterquis/curlrevshell/lib/opshell"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// bidirKeyLen is the size of the bidirectional sentinel key, in bytes.
const bidirKeyLen = 1024

// sDirection is a stream direction
type sDirection string

// Broker handles I/O From shells.  It ensures only one shell is connected
// at once, but also makes sure it disconnects properly.
type Broker struct {
	mu        sync.Mutex
	key       string
	cancelIn  func()
	cancelOut func()
	bidirKey  string /* Bidirectional sentinel key. */
	wg        sync.WaitGroup
	noMore    bool

	evMu        sync.Mutex
	evCh        chan Event
	evListeners map[chan<- Event]struct{}

	ich <-chan string
	och chan<- opshell.CLine
}

// New returns a new Broker, ready for use.  New's methods are safe for
// concurrent usage.
func New(ich <-chan string, och chan<- opshell.CLine) (*Broker, error) {
	/* Work out a bidirectional sentinel key. */
	bidirKeyBuf := make([]byte, bidirKeyLen)
	if _, err := rand.Read(bidirKeyBuf); nil != err {
		return nil, fmt.Errorf(
			"generatting random bidirectional sentinel key: %w",
			err,
		)
	}
	return &Broker{
		ich:         ich,
		och:         och,
		bidirKey:    string(bidirKeyBuf),
		evCh:        make(chan Event, EVChanLen),
		evListeners: make(map[chan<- Event]struct{}),
	}, nil
}

// Do starts the broker going.  Specifically, it starts events processing.
func (b *Broker) Do(ctx context.Context) error {
	eg, ectx := errgroup.WithContext(ctx)
	eg.Go(func() error { b.processEvents(ectx); return nil })
	eg.Go(func() error {
		<-ectx.Done() /* Wait for a shutdown. */
		/* Don't allow more connections. */
		b.mu.Lock()
		b.noMore = true
		b.mu.Unlock()
		/* Wait for connections to finish. */
		b.wg.Wait()
		return nil
	})
	return eg.Wait()
}

// ConnectIn connects w to a shell with the given key, which should match
// a corresponding call to ConnectOut.  Addr is used for logging.
func (b *Broker) ConnectIn(
	ctx context.Context,
	sl *slog.Logger,
	addr string,
	w io.Writer,
	key string,
) {
	b.connect(
		ctx,
		sl,
		addr,
		&b.cancelIn,
		&b.cancelOut,
		LVInput,
		key,
		func(ctx context.Context, sl *slog.Logger) error {
			return b.proxyIn(ctx, sl, w)
		},
	)
}

// ConnectOut connects r to a shell with the given key, which should match
// a corresponding call to ConnectOut.
func (b *Broker) ConnectOut(
	ctx context.Context,
	sl *slog.Logger,
	addr string,
	r io.Reader,
	key string,
) {
	b.connect(
		ctx,
		sl,
		addr,
		&b.cancelOut,
		&b.cancelIn,
		LVOutput,
		key,
		func(ctx context.Context, sl *slog.Logger) error {
			return b.proxyOut(ctx, sl, r)
		},
	)
}

// ConnectInOut connects a bidirectional connection to a shell.  w and r may
// be the same io.ReadWriter.
func (b *Broker) ConnectInOut(
	ctx context.Context,
	sl *slog.Logger,
	addr string,
	w io.Writer,
	r io.Reader,
) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		b.ConnectIn(ctx, sl, addr, w, b.bidirKey)
	}()
	go func() {
		defer wg.Done()
		b.ConnectOut(ctx, sl, addr, r, b.bidirKey)
	}()
	wg.Wait()
}

// connect makes sure we can use this stream.  It makes sure there's not
// already a cancel function in f and that the key is correct.
func (b *Broker) connect(
	ctx context.Context,
	sl *slog.Logger,
	addr string,
	cancelUs *func(),
	cancelOther *func(),
	dir sDirection,
	key string,
	proxy func(context.Context, *slog.Logger) error,
) {
	b.mu.Lock()
	defer b.mu.Unlock()

	/* Make sure we're not no longer accepting connections. */
	if b.noMore {
		return
	}
	b.wg.Add(1)
	defer b.wg.Done()

	/* dirT is the direction, capitalized. */
	dirT := cases.Title(language.English).String(string(dir))

	/* Need a key. */
	if "" == key {
		sl.Error(LMKeyMissing)
		b.Errorf(addr, "Missing Key")
		return
	}

	/* Log with the proper direction. */
	sl = sl.With(LKDirection, dir)

	/* Make sure the previous shell isn't still disconnecting. */
	if "" == b.key && (nil != *cancelUs || nil != *cancelOther) {
		sl.Error(LMDisconnecting)
		if key == b.bidirKey {
			b.Errorf(
				addr,
				"Rejected %s side of bidirectional "+
					"connection while waiting for shell "+
					"disconnect",
				string(dir),
			)
		} else {
			b.Errorf(
				addr,
				"Rejected %s connection with ID %q "+
					"while waiting for shell disconnect",
				string(dir),
				key,
			)
		}
		return
	}

	/* Don't double-connect. */
	if nil != *cancelUs {
		sl.Error(LMAlreadyConnected)
		if key == b.bidirKey {
			b.Errorf(
				addr,
				"Rejected unexpected %s side of "+
					"bidirectinoal connection",
				string(dir),
			)
		} else {
			b.Errorf(
				addr,
				"Rejected unexpected %s connection with ID %q",
				string(dir),
				key,
			)
		}
		return
	}

	/* Make sure we have the right key if something's already connected. */
	if "" != b.key && 1 != subtle.ConstantTimeCompare(
		[]byte(key),
		[]byte(b.key),
	) {
		sl.Error(
			LMIncorrectKey,
			LKKey, b.key,
			LKIncorrectKey, key,
		)
		if key == b.bidirKey {
			b.Errorf(
				addr,
				"Rejected %s side of bidirectonal "+
					"connection, expected unidirectional "+
					"%s connection with ID %q",
				string(dir),
				string(dir),
				b.key,
			)
		} else {
			b.Errorf(
				addr,
				"Rejected %s connection with ID %q, "+
					"expected %q",
				string(dir),
				key,
				b.key,
			)
		}
		return
	}

	/* Looks like we're all set. */
	cctx, cancel := context.WithCancel(ctx)
	*cancelUs = cancel

	/* Note we've a new connection. */
	sl.Info(LMNewConnection)
	if key != b.bidirKey {
		b.Logf(addr, "%s connected: ID %q", dirT, key)
	}

	/* If we've got both sides, let the user know. */
	if nil != *cancelUs && nil != *cancelOther {
		b.Logf(addr, "%s", ShellReadyMessage)
		b.evCh <- Event{Type: EventTypeConnected}
	}

	/* Everything looks good.  Set the key to prevent the wrong output
	connection and unlock b for now.
	We'll lock it again befor we exit. */
	b.key = key
	b.mu.Unlock()

	/* Actually do the proxy. */
	ct := "connection"
	if key == b.bidirKey {
		ct = "side of bidirectional " + ct
	}
	msg := fmt.Sprintf("%s %s closed", dirT, ct)
	if err := proxy(cctx, sl); nil != err {
		sl.Error(LMDisconnected, LKError, err)
		b.Errorf(addr, "%s: %s", msg, err)
	} else {
		sl.Info(LMDisconnected)
		b.Errorf(addr, "%s", msg)
	}

	/* Relock B, which will be unlocked by a defer, above, and start the
	shell disconnecting. */
	b.mu.Lock()
	b.key = ""
	*cancelUs = nil
	if f := *cancelOther; nil != f {
		go f() /* Avoid deadlock. */
	}

	/* If both sides of the shell are gone, tell the user. */
	if nil == *cancelUs && nil == *cancelOther {
		b.Errorf(addr, "%s", ShellDisconnectedMessage)
		b.evCh <- Event{Type: EventTypeDisconnected}
	}
}

// ProxyIn proxies from the ich passed to New. to the writer set by b.ConnectIn
// or b.ConnectInOut.
func (b *Broker) proxyIn(
	ctx context.Context,
	sl *slog.Logger,
	w io.Writer,
) error {
	/* Set up to flush the writer, if it's flushable. */
	flush := func() error { return nil }
	if f, ok := w.(interface{ FlushError() error }); ok {
		flush = f.FlushError
	} else if f, ok := w.(http.Flusher); ok {
		flush = func() error { f.Flush(); return nil }
	}

	/* Proxy. */
	for {
		select {
		case l, ok := <-b.ich:
			l += "\n" /* Add back newline. */
			if !ok {  /* Input channel closed. */
				return nil
			}
			if _, err := io.WriteString(w, l); nil != err {
				return fmt.Errorf("sending line: %w", err)
			}
			if err := flush(); nil != err {
				return fmt.Errorf("flushing line: %w", err)
			}
			sl.Info(LMShellIO, LKData, l)
		case <-ctx.Done(): /* Something else told us to stop. */
			if err := context.Cause(ctx); !errors.Is(
				err,
				context.Canceled,
			) {
				return err
			}
			return nil
		}
	}
}

// proxyOut proxies from the writer set by b.ConnectOut or b.ConnectInOut to
// the och passed to New.
func (b *Broker) proxyOut(
	ctx context.Context,
	sl *slog.Logger,
	r io.Reader,
) error {
	/* Make read data available to us. */
	type outRet struct {
		o   string
		err error
	}
	och := make(chan outRet, 2)
	go func() {
		defer close(och)
		var (
			buf = make([]byte, 2048)
			n   int
			err error
		)
		for nil == ctx.Err() && nil == err {
			n, err = r.Read(buf) /* Try to read a bit. */
			if 0 != n {          /* Send data if we have it. */
				och <- outRet{o: string(buf[:n])}
			}
			if nil != err { /* And an error if we have one. */
				och <- outRet{err: err}
			}
		}
	}()

	/* Proxy output until something happens. */
	var err error
	for nil == err {
		select {
		case o, ok := <-och: /* Chunk of output. */
			if !ok {
				err = io.EOF /* Will be cleared later. */
				break
			}
			/* If we got output. send it forth. */
			if "" != o.o {
				select {
				case b.och <- opshell.CLine{
					Line:  o.o,
					Plain: true,
				}:
					sl.Info(LMShellIO, LKData, o.o)
				case <-ctx.Done(): /* Should stop. */
				}
			}
			/* If we got an error, we're done. */
			if nil != o.err {
				err = o.err
			}
		case <-ctx.Done(): /* Someone told us to stop. */
		}
		/* If the context is done, save the error if we don't have
		a better one. */
		if nil != ctx.Err() {
			if nil == err {
				err = context.Cause(ctx)
			}
		}
	}

	/* Some errors just indicate "normal" termination. */
	if errors.Is(err, io.EOF) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, io.ErrUnexpectedEOF) {
		return nil
	}

	return err
}
