package sstls

/*
 * verify.go
 * Verify a TLS peer's fingerprint
 * By J. Stuart McMurray
 * Created 20260815
 * Last Modified 20260815
 */

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"strings"
)

// SHA256Prefix is allowed but not required to be at the start of the
// fingerprint passed to TLSFingerprintVerifier.
const SHA256Prefix = "sha256//"

// TLSFingerprintVerifier returns a function which can be used for
// [tls.Config.VerifyConnection].  It ensures the peer presents a certificate
// with the given fingerprint, which must be a base64-encoded sha256 hash as
// used by curl, with or without the leading sha256//.
func TLSFingerprintVerifier(wantFP string) (func(tls.ConnectionState) error, error) {
	/* Don't need the prefix. */
	wantFP = strings.TrimPrefix(wantFP, SHA256Prefix)
	/* Make sure the fingerprint looks correct. */
	if b, err := base64.StdEncoding.DecodeString(wantFP); nil != err {
		return nil, fmt.Errorf("decoding fingerprint: %w", err)
	} else if sha256.Size != len(b) {
		return nil, InvalidFingerprintSizeError(len(b))
	}

	/* Return a function to check if any of the certs in
	cs.PeerCertificates have the right hash. */
	return func(cs tls.ConnectionState) error {
		/* Check ALL the certs. */
		for i, cert := range cs.PeerCertificates {
			/* Hash the server's cert. */
			gotFP, err := PubkeyFingerprint(cert)
			if nil != err {
				return PeerCertificatePubkeyError{
					NCerts: len(cs.PeerCertificates),
					Idx:    i + 1,
					Err:    err,
				}
			}

			/* See if it matches. */
			if gotFP == wantFP {
				return nil
			}
		}
		return ErrNoMatchingCertificate
	}, nil
}
