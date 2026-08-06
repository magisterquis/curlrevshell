package crstemplate

/*
 * params.go
 * Parameters passed to -template templates
 * By J. Stuart McMurray
 * Created 20241205
 * Last Modified 20260807
 */

import "net/http"

// Params are passed to subtemplates.  All hosts and addresses not in Request
// have port numbers.
type Params struct {
	/* Server configuration. */

	// ListenAddress is the IP:port on which the server is listening.
	ListenAddress string

	// PubkeyFP is the base64-encoded SHA256 hash of the Server's TLS
	// public key.
	PubkeyFP string

	// CallbackAddresses are the host:ports on which the server is expected
	// to be reached: interface addresses as well as addresses set with
	// -callback-address and -icanhazip.
	CallbackAddresses []string

	// URLPaths are usually i, o, etc but may have been changed at
	// compile-time.  They will not include slashes.
	URLPaths URLPaths

	// StaticFilesDir is the directory from which static files are to be
	// served, or empty if no static files are served.
	StaticFilesDir string

	// For the callback and files subtemplates, C2Addr is the address for
	// which to generate a one-liner.
	//
	// For the script (/c) subtemplate, C2Addr contains, in order of
	// priority, the c2 parameter in HTTP form data or URL's query
	// parameters, the HTTP C2: header, the HTTP Host: header, the TLS SNI,
	// or nothing.  If C2Addr is not empty, it will either have the port
	// set in the request or, if no port was set in the request the port on
	// which the request was received.
	C2Addr string

	// Protocol is normally "https", but will be set to "wss" in the script
	// subtemplate (/c) if the connection arrives over a websocket.
	Protocol string

	/* Parameters only set for the script subtemplate (/c). */

	// ID is a random ID string consisting of a base36-encoded uint64.
	// Only set when executing the script subtemplate (/c).
	ID string

	// Request is the HTTP request for which the script template is being
	// executed.
	// Only set when executing the script subtemplate (/c).
	Request *http.Request

	// BasicAuth are the Username and Password set in the request's
	// Authorization header, if set.
	// Only set when executing the script subtemplate (/c).
	BasicAuth BasicAuth

	// LocalAddress is the address on which the request was received.
	// Only set when executing the script subtemplate (/c).
	LocalAddress string
}

// URLPaths contain the parts of the URL paths indicating what an HTTPS
// request is for.
type URLPaths struct {
	In     string /* Default: i */
	InOut  string /* Default: io */
	Out    string /* Default: o */
	Script string /* Default: c */
}

// BasicAuth contains the Basic Auth credentials sent in a request.
type BasicAuth struct {
	Ok       bool /* True if the request contained credentials. */
	Username string
	Password string
}
