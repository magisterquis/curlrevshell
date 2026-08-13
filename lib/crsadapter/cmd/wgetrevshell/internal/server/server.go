// Package server - Wgetrevshell server logic
package server

/*
 * server.go
 * Wgetrevshell server logic
 * By J. Stuart McMurray
 * Created 20260810
 * Last Modified 20260812
 */

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path"
	"time"

	"github.com/magisterquis/curlrevshell/lib/crsadapter"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
)

const (
	// crsTag is the tag we use for user-visible log in curlrevshell.
	crsTag = "wgetrevshell"
	// crsIDPrefix is the prefix on the ID we use in curlrevshell logs.
	crsIDPrefix = "wrs-"
	// shutdownTimeout is how long we wait for the HTTP server to shut
	// down when the context is done.
	shutdownTimeout = 5 * time.Second
)

// Log messages and keys.
const (
	LMDuplexError    = "Error enabling duplex comms"
	LMInputDialError = "Error establishing input stream"

	LKSize = "size"
)

// Serve runs the server.
func Serve(
	ctx context.Context,
	sl *slog.Logger,
	l net.Listener, /* For https service. */
	aSock string, /* Curlrevshell adapter socket. */
	pathPrefix string,
	urlPathIn string,
	urlPathOut string,
) error {
	/* Shell output, which will persist as long as Serve runs. */
	crsOut, err := handshake(
		ctx,
		aSock,
		crsadapter.ShellStreamDirectionOutput,
	)
	if nil != err {
		return fmt.Errorf("establishing output stream: %w", err)
	}
	defer crsOut.Close()

	/* HTTP handlers. */
	var (
		h   = newHandlers(sl, aSock, crsOut)
		mux = http.NewServeMux()
		p   = func(s string) string {
			return path.Join("/", pathPrefix, s)
		}
	)
	mux.Handle(p(urlPathIn), http.HandlerFunc(h.handleIn))
	mux.Handle(p(urlPathOut), http.HandlerFunc(h.handleOut))

	/* Serve HTTP clients. */
	eg, ctx := ctxerrgroup.WithContext(ctx)
	hsrv := &http.Server{
		Handler:     mux,
		ErrorLog:    slog.NewLogLogger(sl.Handler(), slog.LevelDebug),
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	hsrv.SetKeepAlivesEnabled(false)
	eg.GoTag(ctx, "http", func(ctx context.Context) error {
		err := hsrv.Serve(l)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		return err
	})
	eg.GoTag(ctx, "exitwatcher", func(ctx context.Context) error {
		/* Wait for the context to expire. */
		<-ctx.Done()
		/* Done with the connection to curlrevshell. */
		crsOut.Close()
		/* Stop HTTP service. */
		toCtx, cancel := context.WithTimeout(
			context.Background(),
			shutdownTimeout,
		)
		defer cancel()
		return hsrv.Shutdown(toCtx)
	})
	return eg.Wait()
}
