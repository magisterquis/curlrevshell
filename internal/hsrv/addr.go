package hsrv

/*
 * addr.go
 * Work out our address(es)
 * By J. Stuart McMurray
 * Created 20260729
 * Last Modified 20260801
 */

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var (
	// testErrorCBAddr is an address passed to allCallbackAddresses which
	// causes allCallbackAddresses to return errAllCallbackAddresses.
	testErrorCBAddr string

	// testEmptyCBAddr is an address passed to allCallbackAddresses which
	// causes allCallbackAddresses to return (nil, nil).
	testEmptyCBAddr string

	// errAllCallbackAddresses is returned by allCallbackAddresses if
	// testErrorCBAddr is passed as one of the addresses.
	errAllCallbackAddresses = errors.New(
		"allCallbackAddresses injected error",
	)
)

func init() {
	if testing.Testing() {
		testErrorCBAddr = "error-address" + rand.Text()
		testEmptyCBAddr = "empty-address" + rand.Text()
	}
}

// allCallbackAddresses gets all of the addresses we have for the box, sorted.
func (s *Server) allCallbackAddresses(cbAddrs []string) ([]string, error) {
	var addrs []string

	/* For testing, we may bail here. */
	if testing.Testing() {
		if slices.Contains(cbAddrs, testErrorCBAddr) {
			return nil, errAllCallbackAddresses
		} else if slices.Contains(cbAddrs, testEmptyCBAddr) {
			return nil, nil
		}
	}

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
			s.errorLogf(
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
				s.errorLogf(
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
