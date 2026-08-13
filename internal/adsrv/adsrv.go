// Package adsrv - Adapter (and such) over Unix sockets server
package adsrv

/*
 * adsrv.go
 * Adapter (and such) over Unix sockets server
 * By J. Stuart McMurray
 * Created 20260808
 * Last Modified 20260809
 */

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/bidirpipe"
	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
)

// Log messages, keys, and values.
var (
	LMConnRequestArgsError   = "Connection request arguments error"
	LMConnRequestError       = "Connection request error"
	LMConnRequestReceived    = "Connection request received"
	LMConnResponseError      = "Connection response error"
	LMConnResponseSent       = "Connection response sent"
	LMAdapterListenerStarted = "Adapter listener started"
	LMNewAdapterConn         = "New adapter connection"

	LKAdapterInfo    = "adapter_info"
	LKConnRequest    = "connection_request"
	LKConnResponse   = "connection_response"
	LKConnectionName = "connection_name"
	LKError          = "error"
	LKSocketPath     = "socket_path"
)

// AdapterConnPrefix is used in naming adapters with no unix path.
const AdapterConnPrefix = "adapter-conn-"

// Server serves requests from adapters and other such things.
type Server struct {
	sl  *slog.Logger
	l   *net.UnixListener
	iob *iobroker.Broker
}

// testClientsChannelContextKey extracts a channel from a context on which are
// sent what would be connected clients without the need for unix socket
// syscalls.
// It should only be used for testing.
type testClientsChannelContextKey struct{}

// New listens on the unix socket lPath, unlinking it if it exists, and returns
// a new Server ready for a call to Run.
func New(
	sl *slog.Logger,
	lPath string, /* Listen path. */
	iob *iobroker.Broker,
) (*Server, error) {
	/* Start the listener, first unlinking the path if it already exists.
	Still a bit racy, but one of those unsolved problems. */
	if err := os.RemoveAll(lPath); nil != err {
		return nil, fmt.Errorf(
			"unlinking existing socket path: %w",
			err,
		)
	}
	l, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: lPath,
		Net:  "unix",
	})
	if nil != err {
		return nil, fmt.Errorf("starting listener: %w", err)
	}

	/* Tell everybody we're listening. */
	sl.Info(
		LMAdapterListenerStarted,
		LKSocketPath, l.Addr().String(),
	)

	/* Looks good. */
	return &Server{
		sl:  sl,
		l:   l,
		iob: iob,
	}, nil
}

// Run handles adapter (and such) clients.
func (s *Server) Run(
	ctx context.Context,
) error {
	defer s.l.Close()

	/* Wrangler of goroutines. */
	eg, ctx := ctxerrgroup.WithContext(ctx)
	eg.GoTag(ctx, "listener-stopper", func(ctx context.Context) error {
		<-ctx.Done()
		return s.l.Close()
	})

	/* Accept clients, handle. */
	var nConns int
	eg.GoTag(ctx, "clients", func(ctx context.Context) error {
		for {
			var c bidirIO
			/* Grab the next client. */
			if testing.Testing() {
				if ch, ok := ctx.Value(
					testClientsChannelContextKey{},
				).(chan bidirpipe.Pipe); ok {
					var ok bool
					select {
					case c, ok = <-ch:
						if !ok {
							return nil
						}
					case <-ctx.Done():
						return nil
					}
				}
			}
			if nil == c { /* Didn't get one from a test. */
				var err error
				if c, err = s.l.AcceptUnix(); errors.Is(
					err,
					net.ErrClosed,
				) && nil != ctx.Err() {
					return nil
				} else if nil != err {
					return err
				}
			}
			/* Connection number. */
			nConns++
			n := nConns
			/* Work out what we'll call it.  Can't rely on a
			non-empty remote address. */
			var name string
			if ra, ok := c.(interface {
				RemoteAddr() net.Addr
			}); ok {
				name = ra.RemoteAddr().String()
			}
			if "" == name || "@" == name {
				name = AdapterConnPrefix + strconv.Itoa(n)
			}
			/* Handle this client. */
			eg.GoTag(ctx, name, func(ctx context.Context) error {
				return s.handle(
					ctx,
					s.sl.With(LKConnectionName, name),
					c,
				)
			})
		}
	})

	/* Wait until something goes wrong. */
	return eg.Wait()
}

// Addr returns s's listener's address.
func (s *Server) Addr() net.Addr { return s.l.Addr() }
