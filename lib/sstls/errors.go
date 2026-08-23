package sstls

/*
 * errors.go
 * Error types and values
 * By J. Stuart McMurray
 * Created 20260815
 * Last Modified 20260823
 */

import (
	"crypto/sha256"
	"errors"
	"fmt"
)

// ErrCacheFileEmpty indicates the the file passed to [LoadCachedCertificate]
// or the slice passed to [ParseCachedCertificate] was missing the certificate.
var ErrCacheFileEmpty = errors.New("cache file empty")

// ErrPrivateKeyPEMEmpty indicates the the file passed to
// [LoadCachedCertificate] or the slice passed to [ParseCachedCertificate] was
// missing the private key or the private key was empty.
var ErrPrivateKeyPEMEmpty = errors.New("PEM-encoded key missing or empty")

// ErrLeafCertificateNotSet is returned by [LoadCachedCertificate] if the
// [tls.Certificate] it would have returned has a nil
// [LoadCachedCertificate.Leaf], which should only happen if
// "x509keypairleaf=0" is set in GODEBUG.
var ErrLeafCertificateNotSet = errors.New("unset leaf certificate")

// ErrNoMatchingCertificate indicates a TLS connection's peer did not present
// a certificate matching a configured fingerprint.
var ErrNoMatchingCertificate = errors.New(
	"no certificate with correct fingerprint found",
)

// ErrMissingLeafCertificate indicates a leaf certificate was expected but not
// present.
var ErrMissingLeafCertificate = errors.New("missing leaf x509 certificate")

// InvalidFingerprintSizeError indicates that a decoded fingerprint was an
// invalid size.
type InvalidFingerprintSizeError int

// Error implements the error interface.
func (err InvalidFingerprintSizeError) Error() string {
	return fmt.Sprintf(
		"decoded fingerprint %d bytes, expected %d",
		int(err),
		sha256.Size,
	)
}

// PeerCertificatePubkeyError is returned as part of TLS connection
// verification if an error is encountered verifying a PubKey in one of the
// peer's certificates.
type PeerCertificatePubkeyError struct {
	Idx    int /* 1-indexed index into presented cert slice. */
	NCerts int /* Number of certificates presented. */
	Err    error
}

// Error implements the error interface.
func (err PeerCertificatePubkeyError) Error() string {
	return fmt.Sprintf(
		"getting fingerprint for peer certificate %d/%d: %v",
		err.Idx, err.NCerts,
		err.Err,
	)
}

// Unwrap returns err.Err.
func (err PeerCertificatePubkeyError) Unwrap() error { return err.Err }
