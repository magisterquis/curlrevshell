// Package crstemplate contains bits and bobs related to curlrevshell's
// template usage.
package crstemplate

/*
 * crstemplate.go
 * Curlrevshell template things
 * By J. Stuart McMurray
 * Created 20241205
 * Last Modified 20241205
 */

import _ "embed"

// Default URL paths for comms with shells.
const (
	DefaultURLPathIn     = "i"  /* Shell input */
	DefaultURLPathInOut  = "io" /* Shell bidirectional stream */
	DefaultURLPathOut    = "o"  /* Shell output */
	DefaultURLPathScript = "c"  /* Script generation. */
)

// DefaultTemplate is the default callback script template.  It can be
// overridden at runtime with -callback-template
//
//go:embed script.tmpl
var DefaultTemplate string

// Params are combined with the callback template to generate the callback
// script.
type Params struct {
	PubkeyFP string /* Base64'd SHA256 hash of the server's TLS pubkey. */
	URL      string /* URL to call back to the server. */
	ID       string /* Random ID string. */

	// URLPaths are usually i, o, etc but may have been changed at
	// compile-time.  They will not include slashes.
	URLPaths URLPaths
}

// URLPaths contain the parts of the URL paths indicating what an HTTPS
// request is for.
type URLPaths struct {
	In     string /* Default: i */
	InOut  string /* Default: io */
	Out    string /* Default: o */
	Script string /* Default: c */
}
