package crstemplate

/*
 * params.go
 * Parameters passed to -template templates
 * By J. Stuart McMurray
 * Created 20241205
 * Last Modified 20241222
 */

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
