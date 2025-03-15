package crstemplate

/*
 * request.go
 * Turn a request into a Request.
 * By J. Stuart McMurray
 * Created 20250126
 * Last Modified 20250214
 */

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"slices"
	"strconv"
	"testing"

	"golang.org/x/net/idna"
)

// C2Param is a URL parameter or header which may be set in requetss to
// /c to give the URL to which to call back.
const C2Param = "c2"

// testingIgnoreHostHeader is a Host: header which causes c2Addr to pretend
// it didn't get one, if we're in testing.
const testingIgnoreHostHeader = "ignore_this_please"

// ErrRequestAlreadyAdded is returned by AddRequest if the Params to which
// a request was to be added already had one.
var ErrRequestAlreadyAdded = errors.New("Params already has a request")

// AddRequest returns a copy of p with information from r set, for the script
// template.
func AddRequest(p Params, r *http.Request) (Params, error) {
	/* Make sure we don't already have a request. */
	if nil != p.Request {
		return Params{}, ErrRequestAlreadyAdded
	}

	/* Clone the parameters. */
	ret := p
	ret.CallbackAddresses = slices.Clone(p.CallbackAddresses)

	/* Get the local address.  If this isn't a TCP address, something has
	gone terribly wrong. */
	la := localAddrFromRequest(r)

	/* Work out the C2 address. */
	a, err := c2Addr(r, la)
	if nil != err {
		return Params{}, fmt.Errorf("determining C2 address: %w", err)
	}

	/* Grab creds from the request, if available.  We do this here because
	ret.BasicAuth three times was really long. .*/
	var ba BasicAuth
	ba.Username, ba.Password, ba.Ok = r.BasicAuth()

	/* Add in the request bits. */
	if !(testing.Testing() && "" != ret.ID) {
		ret.ID = strconv.FormatUint(rand.Uint64(), 36)
	}
	ret.C2Addr = a
	ret.Request = r
	ret.BasicAuth = ba
	ret.LocalAddress = la.String()

	return ret, nil
}

// localAddrFromRequest returns the address on which r came in.
func localAddrFromRequest(r *http.Request) *net.TCPAddr {
	return r.Context().Value(http.LocalAddrContextKey).(*net.TCPAddr)
}

// c2Addr tries to get a C2 URL from r.  We try a query/form parameter, a
// c2: header, the Host: header, and the SNI, in that order.  The returned
// address' port will be, in order of precedence, what's sent from the client,
// or the port on which the connection arrived.
func c2Addr(r *http.Request, localAddress *net.TCPAddr) (string, error) {
	/* Get the port on which the connection arrived. */
	/* withPort makes adds the port from the request to a, if a doesn't
	already have a port. */
	withPort := func(a string) string {
		return EnsurePort(strconv.Itoa(localAddress.Port), a)
	}

	/* Parse the query and form and try to get it from there. */
	if err := r.ParseForm(); nil != err {
		return "", fmt.Errorf("parsing request: %w", err)
	}
	if p := r.Form.Get(C2Param); "" != p {
		return withPort(p), nil
	}

	/* If it's not there, try to get it as a header. */
	if p := r.Header.Get(C2Param); "" != p {
		return withPort(p), nil
	}

	/* Failing that, try the Host: header. */
	if p, err := idna.ToASCII(r.Host); nil != err {
		return "", fmt.Errorf("punycoding %s: %w", r.Host, err)
	} else if "" != p &&
		!(testing.Testing() && testingIgnoreHostHeader == p) {
		return withPort(p), nil
	}

	/* No Host: header.  Probably HTTP/1.0.  Try the SNI. */
	if nil != r.TLS && "" != r.TLS.ServerName {
		return withPort(r.TLS.ServerName), nil
	}

	/* Out of ideas at this point. */
	return "", errors.New("out of ideas")
}
