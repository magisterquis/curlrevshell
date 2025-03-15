package crstemplate

/*
 * template_funcs.go
 * Functions available to templates
 * By J. Stuart McMurray
 * Created 20250205
 * Last Modified 20250205
 */

import (
	"cmp"
	"errors"
	"fmt"
	"net"
	"text/template"
)

// DefaultPort is used when we need a port and have no better options.
const DefaultPort = "443"

// TemplateFuncs are functions added to templates' function map.
var TemplateFuncs = template.FuncMap{
	"ensureport":    EnsurePort,
	"host":          Host,
	"map":           Map,
	"nodefaultport": NoDefaultPort,
	"port":          Port,
}

// Host takes an IP:port or host:port and returns just the IP or host.  It is
// a wrapper around net.SplitHostPort.
func Host(addr string) (string, error) {
	h, _, err := net.SplitHostPort(addr)
	return h, err
}

// Port takes an IP:port or host:port and returns just the port.  If there
// was no port, Port returns DefaultPort.
func Port(addr string) (string, error) {
	if _, p, err := net.SplitHostPort(addr); nil == err {
		/* All is good. */
		return p, nil
	} else if hasNoPort(err) {
		/* No port to begin with. */
		return DefaultPort, nil
	} else {
		/* Some other error. */
		return "", err
	}
}

// EnsurePort takes an address and ensures it contains a port.  If addr is
// the empty string, the empty string will be returned.  If port is the empty
// string DefaultPort is used.
func EnsurePort(port, addr string) string {
	/* Shouldn't happen, but for just in case. */
	if "" == addr {
		return ""
	}
	if _, p, err := net.SplitHostPort(addr); nil == err && "" != p {
		return addr
	}
	return net.JoinHostPort(addr, cmp.Or(port, DefaultPort))
}

// NoDefaultPort takes a host:port or IP:port and returns it without the port
// if the port was DefaultPort.  If addr has no port to begin with or can't be
// parsed into an address and port, it is returned as-is.
func NoDefaultPort(addr string) string {
	if h, p, err := net.SplitHostPort(addr); nil == err &&
		p == DefaultPort {
		return h
	}
	return addr
}

// hasNoPort returns true if err, which should come from net.SplitHostPort,
// indicates that the split address had no port.
func hasNoPort(err error) bool {
	var ae *net.AddrError
	return errors.As(err, &ae) && "missing port in address" == ae.Err
}

// Map assembles its arguments, which must be key/value pairs into a map,
// for easier passing multiple values to subtemplates.  Totally not a knockoff
// of Sprig's dict...
func Map(kvs ...any) (map[string]any, error) {
	/* Make sure we don't have a half-pair. */
	if 0 != len(kvs)&0x1 {
		return nil, fmt.Errorf("need a value for each key")
	}
	/* Assemble the map. */
	ret := make(map[string]any)
	for i := 0; i < len(kvs); i += 2 {
		/* Key should be a string. */
		s, ok := kvs[i].(string)
		if !ok {
			return nil, fmt.Errorf(
				"key %v is a %T, not a string",
				kvs[i],
				kvs[i],
			)
		}
		/* Save this pair. */
		ret[s] = kvs[i+1]
	}

	return ret, nil
}
