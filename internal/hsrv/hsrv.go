// Package hsrv - HTTP server
package hsrv

/*
 * hsrv.go
 * HTTP server
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20250924
 */

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/lib/crstemplate"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
	"github.com/magisterquis/curlrevshell/lib/sstls"
)

// Log messages and keys.
const (
	LMListening               = "Listener started"
	LMOneShellClosingListener = "Got one shell, closing listener"
	LMURLPaths                = "Non-Default URL Paths"

	LKError      = "error"
	LKListenAddr = "address"
)

// ShutdownWait is how long we wait for cilents to disconnect on shutdown.
const ShutdownWait = 4 * time.Second

// ErrOneShellClosed indicates that the listener was closed as expected after
// receiving a single shell.
var ErrOneShellClosed = errors.New("closed after shell received")

// Server serves implants over HTTPS.
type Server struct {
	sl       *slog.Logger
	ich      <-chan string
	och      chan<- opshell.CLine
	iob      *iobroker.Broker
	l        sstls.Listener
	dw       io.Writer /* Debug writer. */
	oneShell bool      /* Close listener after getting a shell. */

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
	ich <-chan string, /* Stdin -> Shell. */
	och chan<- opshell.CLine, /* Stdout <- Shell. */
	iob *iobroker.Broker,
	certFile string, /* Cert cache file. */
	cbAddrs []string, /* Callback addresses, for one-liners. */
	printIPv6 bool, /* Print IPv6 interface addresses. */
	oneShell bool, /* Shut down listener after first shell. */
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
	var err error
	if l, err = sstls.Listen("tcp", addr, "", 0, certFile); nil != err {
		return nil, fmt.Errorf("listening on %s: %w", addr, err)
	}
	params.ListenAddress = l.Addr().String()
	params.PubkeyFP = l.Fingerprint
	sl.Info(LMListening, LKListenAddr, l.Addr().String())

	/* Work out where to send debug messages. */
	dw := io.Discard
	if printDebug {
		dw = pinkSender{och}
	}

	/* Server to return. */
	s := &Server{
		sl:        sl,
		ich:       ich,
		och:       och,
		iob:       iob,
		l:         l,
		dw:        dw,
		tmplf:     tmplf,
		params:    params,
		printIPv6: printIPv6,
		oneShell:  oneShell,
	}

	/* Work out our listen addresses, for user help. */
	if s.lAddrs, err = s.listenAddresses(cbAddrs); nil != err {
		l.Close()
		return nil, fmt.Errorf(
			"determining listen addresses: %w",
			err,
		)
	}
	if 0 == len(s.lAddrs) {
		l.Close()
		return nil, errors.New("no listen addresses")
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

	return s, nil
}

// Do actually serves HTTPS clients.
func (s *Server) Do(ctx context.Context) error {
	/* Sign up to watch IO Broker events. */
	evCh := make(chan iobroker.Event, iobroker.EVChanLen)
	s.iob.AddEventListener(evCh)
	defer func() {
		s.iob.RemoveEventListener(evCh)
		close(evCh)
	}()

	/* Tell the user we're listening. */
	s.Logf(opshell.ColorNone, "Listening on %s", s.l.Addr())

	/* Tell user where to get static files. */
	if "" != s.params.StaticFilesDir {
		s.printStaticFileHelp()
	}

	/* Tell user how to get a callback. */
	s.printCallbackHelp()

	/* Serve clients and watch events. */
	eg, ectx := ctxerrgroup.WithContext(ctx)
	eg.GoTag(ectx, "http_server", s.serveHTTP) /* Handle HTTP. */
	eg.GoTag(ectx, "iowatch", func(            /* Process IOB events. */
		ctx context.Context,
	) error {
		s.watchIOBEvents(ctx, evCh)
		return nil
	})
	return eg.Wait()
}

// listenAddresseses gets all of the addresses we have for the box, sorted.
func (s *Server) listenAddresses(cbAddrs []string) ([]string, error) {
	var addrs []string

	/* Parse the listen address and port, which we'll need for
	manually-added callback addresses. */
	ls := s.l.Addr().String()
	ap, err := netip.ParseAddrPort(ls)
	if nil != err {
		return nil, fmt.Errorf(
			"parsing listen address %s: %w",
			ls,
			err,
		)
	}
	port := strconv.Itoa(int(ap.Port()))

	/* Add extra addresses, for just in case. */
	for _, a := range cbAddrs {
		/* Make sure we have a port. */
		if _, p, err := net.SplitHostPort(a); "" == p || nil != err {
			a = net.JoinHostPort(a, port)
		}
		addrs = append(addrs, a)
	}

	/* If the listen address isn't a wildcard address, we're good with
	just i. */
	if !ap.Addr().IsUnspecified() {
		return sortAddresses(append(addrs, ap.String())), nil
	}

	/* Get all the addresses we know about. */
	nifs, err := net.Interfaces()
	if nil != err {
		return nil, fmt.Errorf("enumerating interfaces: %w", err)
	}
	for _, nif := range nifs {
		/* Dont print loopback addresses. */
		if 0 != net.FlagLoopback&nif.Flags {
			continue
		}
		/* Get this interface's addresses. */
		ifas, err := nif.Addrs()
		if nil != err {
			s.ErrorLogf(
				"Error getting addresses for %s: %s",
				nif.Name,
				err,
			)
			continue
		}
		/* Keep hold of each address on this interface. */
		for _, ifa := range ifas {
			ps := ifa.String()
			/* If we have a netmask, remove it. */
			p, err := netip.ParsePrefix(ps)
			if nil != err {
				s.ErrorLogf(
					"Error parsing "+
						"callback address %s: %s",
					ps,
					err,
				)
				continue
			}
			/* If it's IPv6, make sure we want it. */
			if p.Addr().Is6() && !s.printIPv6 {
				continue
			}
			/* Save the address with the listen port. */
			addrs = append(addrs, net.JoinHostPort(
				p.Addr().String(),
				port,
			))
		}
	}
	addrs = sortAddresses(addrs)

	/* If we haven't any addresses by this point, something's wrong. */
	if 0 == len(addrs) {
		return nil, fmt.Errorf("no interfaces have addresses")
	}

	return addrs, nil
}

// watchIOBEvents watches for events from the IO Broker and takes action.  Its
// only job is to either send the reconnect message when the shell dies or to
// kill the listener, if we have -one-shell.
func (s *Server) watchIOBEvents(
	ctx context.Context,
	evCh <-chan iobroker.Event,
) {
	/* Watch for events. */
	var (
		ev iobroker.Event
		ok bool
	)
	for {
		/* Grab the next event. */
		select {
		case ev, ok = <-evCh: /* Pop an event. */
			if !ok { /* No more events. */
				return
			}
			/* Handled below. */
		case <-ctx.Done(): /* Or not. */
			return
		}
		/* Do a thing. */
		switch ev.Type {
		case iobroker.EventTypeConnected:
			if s.oneShell {
				s.Logf(
					ConnectedColor,
					"%s",
					ClosingListenerMessage,
				)
				s.sl.Debug(LMOneShellClosingListener)
				s.l.Close()
			}
		case iobroker.EventTypeDisconnected:
			/* Print the callback help when the shell dies. */
			if !s.oneShell {
				s.printCallbackHelp()
			}
		}
	}
}

// serveHTTP starts HTTP Service going.
func (s *Server) serveHTTP(ctx context.Context) error {
	/* Set up a server. */
	hsvr := http.Server{
		Handler:  s.newMux(),
		ErrorLog: log.New(s.dw, "Server error: ", log.Lmsgprefix),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	/* Serve until we fail or the context is cancelled. */
	var ech = make(chan error, 1)
	go func() {
		err := hsvr.Serve(s.l)
		/* If we're only running a single shell, a closed listener is
		to be expected. */
		if errors.Is(err, net.ErrClosed) && s.oneShell {
			err = ErrOneShellClosed
		}
		ech <- err
	}()
	var err error
	select {
	case err = <-ech:
	case <-ctx.Done():
	}

	/* Shutdown the server. */
	toctx, cancel := context.WithTimeout(
		context.Background(),
		ShutdownWait,
	)
	defer cancel()
	serr := hsvr.Shutdown(toctx)

	/* Return the first non-nil error. */
	return cmp.Or(err, serr)
}

// sortAddresses sorts a slice of addresses as string.  Non-IP:Port pairs come
// first, sorted lexicographically, then come IP addresses, sorted as per
// netip.AddrPort.Compare.  The returned slice is deduped via slices.Compact..
func sortAddresses(as []string) []string {
	slices.SortFunc(as, func(a, b string) int {
		/* If either both address are addresses or both aren't sort
		as normal. */
		aa, ea := netip.ParseAddrPort(a)
		ab, eb := netip.ParseAddrPort(b)
		if ea == nil && eb == nil {
			return aa.Compare(ab)
		} else if ea != nil && eb != nil {
			return strings.Compare(a, b)
		}
		/* Failing that, non-IP addresses sort before normal addreses,
		as they're likely what we were asked to print. */
		if ea == nil && eb != nil {
			return 1
		} else if ea != nil && eb == nil {
			return -1
		}
		/* Unpossible. */
		return 0
	})
	return slices.Compact(as)
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
