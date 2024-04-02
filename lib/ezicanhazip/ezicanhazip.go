// Package ezicanhazip - Get IP addresses from icanhazip.com
package ezicanhazip

/*
 * ezicanhazip.go
 * Get IP addresses from icanhazip.com
 * By J. Stuart McMurray
 * Created 20240402
 * Last Modified 20240402
 */

import (
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
)

// BaseDomain is icanhazip's domain.
const BaseDomain = "icanhazip.com"

// IPv4 returns the IP address returned by icanhazip.com.
func IPv4() (netip.Addr, error) {
	/* Ask for our address. */
	res, err := http.Get("https://ipv4." + BaseDomain)
	if nil != err {
		return netip.Addr{}, fmt.Errorf("GET: %w", err)
	} else if http.StatusOK != res.StatusCode {
		return netip.Addr{}, fmt.Errorf(
			"non-OK status: %s",
			res.Status,
		)
	}
	defer res.Body.Close()

	/* Parse the returned address. */
	b, err := io.ReadAll(res.Body)
	if nil != err {
		return netip.Addr{}, fmt.Errorf(
			"reading HTTP response: %w",
			err,
		)
	}
	a, err := netip.ParseAddr(strings.TrimSpace(string(b)))
	if nil != err {
		return netip.Addr{}, fmt.Errorf(
			"parsing response %q: %w",
			b,
			err,
		)
	}

	return a, nil
}
