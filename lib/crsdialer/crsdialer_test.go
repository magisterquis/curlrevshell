package crsdialer

/*
 * crsdialer_test.go
 * Tests for crsdialer.go
 * By J. Stuart McMurray
 * Created 20250905
 * Last Modified 20260815
 */

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/hsrv"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/sstls"
	"golang.org/x/sync/errgroup"
)

// Does it work?
func TestDial(t *testing.T) {
	/* Test server. */
	var (
		done  = make(chan struct{})
		ready = make(chan struct{})
		sL    sync.Mutex
		serr  = make(chan error, 1)
		sr    *http.Request
		sw    http.ResponseWriter
		herr  = make(chan error, 1)
	)
	l, err := sstls.Listen("tcp", "127.0.0.1:0", "", 0, "")
	if nil != err {
		t.Fatalf("Error listening: %s", err)
	}
	defer l.Close()
	svr := http.Server{
		BaseContext: func(net.Listener) context.Context {
			return t.Context()
		},
		Handler: http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			sL.Lock()
			defer sL.Unlock()
			defer close(herr)
			if nil != sw {
				panic("duplicate ResponseWriter")
			} else if nil != sr {
				panic("duplicate Request")
			}
			if err := hsrv.StartFullDuplex(w); nil != err {
				panic("starting full duplex: " + err.Error())
			}
			sw, sr = w, r
			close(ready)
			<-done
		}),
	}
	go func() { serr <- svr.Serve(l) }()

	/* Get a connection. */
	var (
		c       *net.UnixConn
		eg, ctx = ctxerrgroup.WithContext(t.Context())
	)
	eg.GoContext(ctx, func(ctx context.Context) error {
		var err error
		c, err = Dial(
			t.Context(),
			fmt.Sprintf("https://%s", l.Addr()),
			l.Fingerprint,
		)
		return err
	})
	eg.GoContext(ctx, func(ctx context.Context) error {
		select {
		case <-ready: /* Good. */
		case <-ctx.Done(): /* Less good. */
		}
		return nil
	})
	if err := eg.Wait(); nil != err {
		t.Fatalf("Dial failed: %s", err)
	} else if nil == sr {
		t.Fatalf("Did not get Request from server")
	} else if nil == sw {
		t.Fatalf("Did not get ResponseWriter from server")
	}

	/* Testing waitgroup. */
	t.Run("tx/rx", func(t *testing.T) {
		/* Can we send? */
		t.Run("client to server", func(t *testing.T) {
			t.Parallel()
			var (
				eg   errgroup.Group
				have = []byte("kittens")
				got  = make([]byte, len(have))
			)
			eg.Go(func() error {
				if _, err := c.Write(have); nil != err {
					return fmt.Errorf("sending: %w", err)
				}
				return nil
			})
			eg.Go(func() error {
				if _, err := io.ReadFull(
					sr.Body,
					got,
				); nil != err {
					return fmt.Errorf("receiving: %w", err)
				}
				return nil
			})

			/* Did it work? */
			if err := eg.Wait(); nil != err {
				t.Fatalf("Error: %s", err)
			}
			if want := have; !bytes.Equal(want, got) {
				t.Errorf(
					"Incorrect tx/rx\n got: %s\nwant: %s",
					got,
					want,
				)
			}
		})

		/* Can we receive? */
		t.Run("server to client", func(t *testing.T) {
			t.Parallel()
			var (
				eg   errgroup.Group
				have = []byte("moose")
				got  = make([]byte, len(have))
			)
			eg.Go(func() error {
				if _, err := sw.Write(have); nil != err {
					return fmt.Errorf("sending: %w", err)
				}
				if err := http.NewResponseController(
					sw,
				).Flush(); nil != err {
					return fmt.Errorf("flushing: %w", err)
				}
				return nil
			})
			eg.Go(func() error {
				if _, err := io.ReadFull(c, got); nil != err {
					return fmt.Errorf("receiving: %w", err)
				}
				return nil
			})

			/* Did it work? */
			if err := eg.Wait(); nil != err {
				t.Fatalf("Error: %s", err)
			}
			if want := have; !bytes.Equal(want, got) {
				t.Errorf(
					"Incorrect tx/rx\n got: %s\nwant: %s",
					got,
					want,
				)
			}
		})
	})

	/* Does disconnecting work? */
	if err := c.Close(); nil != err {
		t.Errorf("Close failed: %s", err)
	}
	b, err := io.ReadAll(sr.Body)
	if nil != err {
		t.Errorf("Error reading remainder of Request: %s", err)
	}
	if 0 != len(b) {
		t.Errorf("Unexpected read from request: %q", b)
	}

	/* Did the server shut down nicely? */
	close(done)
	if err := svr.Shutdown(t.Context()); nil != err {
		t.Errorf("Error shutting down server: %s", err)
	}
	if err := <-serr; nil == err {
		t.Errorf("Server finished without expected error")
	} else if !errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Server finished unexpected error: %s", err)
	}

	/* Connection should be closed now. */
	b, err = io.ReadAll(c)
	if nil == err {
		t.Errorf(
			"Unexpected success reading from expected-closed " +
				"connection",
		)
	} else if !errors.Is(err, net.ErrClosed) {
		t.Errorf(
			"Unexpected error reading from closed connection: %s",
			err,
		)
	}
	if 0 != len(b) {
		t.Errorf("Unexpected read from closed connection: %q", err)
	}

}
