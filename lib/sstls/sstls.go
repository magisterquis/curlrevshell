// Package sstls - TLS listener with a self-signed certificate
package sstls

/*
 * sstls.go
 * TLS listener with a self-signed certificate
 * By J. Stuart McMurray
 * Created 20240323
 * Last Modified 20241003
 */

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"time"
)

// Listener listens for TLS connections and handshakes with a self-signed
// certificate.
type Listener struct {
	// Wrapped net.Listener, which returns TLS'd conns.
	net.Listener

	// Base64-encoded SHA256 hash of the generated self-signed
	// certificate's public key, suitable for passing to curl's
	// --pinnedpubkey.
	Fingerprint string
}

// Listen listens on the given network and address using the given cert.  If it
// does not exist it is created with the given  subject and lifespan.  It will
// have no SANs.  The certFile is used to read a previously-generated
// certificate; it may be the empty string to always generate a new
// certificate.
func Listen(
	net string,
	address string,
	subject string,
	lifespan time.Duration,
	certFile string,
) (Listener, error) {
	var l Listener

	/* Get or generate a certificate. */
	cert, err := GetCertificate(subject, nil, nil, lifespan, certFile)
	if nil != err {
		return Listener{}, fmt.Errorf(
			"generating certificate: %w",
			err,
		)
	}

	/* Work out fingerprint. */
	if l.Fingerprint, err = PubkeyFingerprintTLS(cert); nil != err {
		return Listener{}, fmt.Errorf(
			"getting certificate fingerprint: %w",
			err,
		)
	}

	/* Start listening. */
	if l.Listener, err = tls.Listen(net, address, &tls.Config{
		Certificates: []tls.Certificate{cert},
	}); nil != err {
		return Listener{}, fmt.Errorf("starting listener: %w", err)
	}

	return l, nil
}

// PubkeyFingerprint returns the SHA256 hash of the public key fingerprint
// for the cert.  This is used for curl's --pinnedpubkey.
func PubkeyFingerprint(cert *x509.Certificate) (string, error) {
	/* Marshal to nicely-hashable DER. */
	b, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if nil != err {
		return "", fmt.Errorf("marshalling to DER: %w", err)
	}

	/* Hash and encode. */
	h := sha256.Sum256(b)
	return base64.StdEncoding.EncodeToString(h[:]), nil
}

// PubkeyFingerprintTLS is like PubkeyFingerprint, but uses the public key of
// the leaf x509 certificate in cert.
// If the certificate's Leaf isn't set, an error is returned.
func PubkeyFingerprintTLS(cert tls.Certificate) (string, error) {
	/* Make sure we have a parsed cert. */
	if nil == cert.Leaf {
		return "", errors.New("missing leaf x509 certificate")
	}

	return PubkeyFingerprint(cert.Leaf)
}
