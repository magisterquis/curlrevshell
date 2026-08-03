// Package hsrv - HTTP server
package hsrv

/*
 * hsrv.go
 * HTTP server
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20260801
 */

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/lib/crstemplate"
	"github.com/magisterquis/curlrevshell/lib/opshell"
	"github.com/magisterquis/curlrevshell/lib/sstls"
)

// Log messages and keys.
const (
	LMFileRequested           = "File requested"
	LMListenerStarted         = "Listener started"
	LMOneShellClosingListener = "Got one shell, closing listener"
	LMURLPaths                = "Non-Default URL Paths"

	LKError          = "error"
	LKFingerprint    = "fingerprint"
	LKListenAddr     = "listen_address"
	LKRequestInfo    = "http_request"
	LKRequestedFile  = "requested_file"
	LKStaticFilesDir = "static_files_dir"
)

// shutdownWait is how long we wait for cilents to disconnect on shutdown.
const shutdownWait = 4 * time.Second

// Server serves implants over HTTPS.
type Server struct {
	sl         *slog.Logger
	iob        *iobroker.Broker
	l          sstls.Listener
	printDebug bool

	/* Template generation. */
	tmplf  string /* Template file. */
	params crstemplate.Params

	/* Things for printing help. */
	lAddrs    []string /* Listen addresses, for help. */
	printIPv6 bool
}

// New returns a new Server, listening on addr.  Call its Do method to start it
// serving.
// tmplf is non-empty, it is taken as a file from which to read the -template
// template.
// params.StaticFilesDir and params.URLPaths may be set by the caller; all
// other fields will be set by New or its handlers.
func New(
	sl *slog.Logger,
	addr string, /* Listen address. */
	tmplf string, /* Template file. */
	iob *iobroker.Broker,
	certFile string, /* Cert cache file. */
	cbAddrs []string, /* Callback addresses, for one-liners. */
	printIPv6 bool, /* Print IPv6 interface addresses. */
	printDebug bool, /* Send the user debug (red) messages. */
	params crstemplate.Params, /* Template params. */
) (*Server, error) {
	var l sstls.Listener

	/* Make sure we actually have URL Paths. */
	crstemplate.CleanURLPaths(&params.URLPaths)

	/* Make sure the listen address has a port, and if not ask the OS to
	choose one for us. */
	if _, p, err := net.SplitHostPort(addr); "" == p || nil != err {
		addr = net.JoinHostPort(addr, "0")
	}

	/* Start our listener. */
	var (
		err     error
		success bool
	)
	if l, err = sstls.Listen("tcp", addr, "", 0, certFile); nil != err {
		return nil, listenError{Addr: addr, Err: err}
	}
	defer func() {
		if !success {
			l.Close()
		}
	}()
	params.ListenAddress = l.Addr().String()
	params.PubkeyFP = l.Fingerprint

	/* Server to return. */
	s := &Server{
		sl:         sl,
		iob:        iob,
		l:          l,
		printDebug: printDebug,
		tmplf:      tmplf,
		params:     params,
		printIPv6:  printIPv6,
	}

	/* Tell everybody we're listening. */
	s.logf(opshell.ColorNone, "Listening on %s", s.l.Addr())
	sl.Info(
		LMListenerStarted,
		LKListenAddr, l.Addr().String(),
		LKFingerprint, l.Fingerprint,
	)

	/* Work out our callback addresses, for user help. */
	if s.lAddrs, err = s.allCallbackAddresses(cbAddrs); nil != err {
		return nil, fmt.Errorf(
			"determining callback addresses: %w",
			err,
		)
	}
	if 0 == len(s.lAddrs) {
		return nil, errNoCallbackAddresses
	}
	params.CallbackAddresses = slices.Clone(s.lAddrs)

	/* Log the paths we're using if they're not the defaults. */
	if s.params.URLPaths != crstemplate.DefaultURLPaths {
		sl.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			LMURLPaths,
			slogAttrsFromURLPaths(s.params.URLPaths)...,
		)
	}

	/* Finally made it, prevent listener from being closed. */
	success = true
	return s, nil
}

// Do actually serves HTTPS clients.
func (s *Server) Do(ctx context.Context) error {

	/* Work out where to send debug messages. */
	dw := io.Discard
	if s.printDebug {
		dw = pinkSender{s.iob}
	}

	/* Set up a server. */
	hsvr := http.Server{
		Handler:  s.newMux(),
		ErrorLog: log.New(dw, "Server error: ", log.Lmsgprefix),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	/* Serve until we fail or the context is cancelled. */
	var ech = make(chan error, 1)
	go func() {
		err := hsvr.Serve(s.l)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		ech <- err
	}()
	var err error
	select {
	case err = <-ech:
		return err
	case <-ctx.Done():
		/* Shutdown the server. */
		toctx, cancel := context.WithTimeout(
			context.Background(),
			shutdownWait,
		)
		defer cancel()
		return hsvr.Shutdown(toctx)
	}
}

// slogAttrsFromURLPaths turns p.URLPaths into Attrs suitable for sending to
// one of slog.Logger's methods.
func slogAttrsFromURLPaths(p crstemplate.URLPaths) []slog.Attr {
	/* Introspect the URLPaths in p. */
	v := reflect.ValueOf(p)
	t := v.Type()
	/* We'll return as many attrs as there are paths. */
	ret := make([]slog.Attr, t.NumField())
	/* Grab each path and turn into an attr. */
	for i := range ret {
		ret[i] = slog.String(t.Field(i).Name, v.Field(i).String())
	}
	/* Sort, which makes testing that much easier. */
	slices.SortFunc(ret, func(a, b slog.Attr) int {
		return strings.Compare(a.Key, b.Key)
	})
	return ret
}
