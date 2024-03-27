package sstls

/*
 * gencert.go
 * Generate a self-signed certificate
 * By J. Stuart McMurray
 * Created 20240323
 * Last Modified 20240327
 */

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"time"
)

var (
	// DefaultSelfSignedCertLifespan is the amount of time self-signed
	// certificates.
	DefaultSelfSignedCertLifespan = time.Until(time.Now().AddDate(10, 0, 0))
	// SelfSignedSubject is the subject name we use for self-signed
	// certificates.
	SelfSignedSubject = "sstls"
)

// Names of files in a txtar archive for the PEM-encoded cert and key.
const (
	txtarCertFile = "cert"
	txtarKeyFile  = "key"
)

// GetCertificate gets a cert from the given file or generates if it doesn't
// exist.  If certFile is the empty string, a certificate will be generated and
// not stored.  The other arguments are the same as for
// GenerateSelfSignedCertificate.
func GetCertificate(
	subject string,
	dnsNames []string,
	ipAddresses []net.IP,
	lifespan time.Duration,
	certFile string,
) (tls.Certificate, error) {
	/* Try reading the cert from the file. */
	if "" != certFile {
		cert, err := LoadCachedCertificate(certFile)
		if nil == err {
			return cert, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return tls.Certificate{}, fmt.Errorf(
				"loading cached certificate: %w",
				err,
			)
		}
	}

	/* Don't have one, generate it. */
	certPEM, keyPEM, cert, err := GenerateSelfSignedCertificate(
		subject,
		dnsNames,
		ipAddresses,
		lifespan,
	)
	if nil != err {
		return tls.Certificate{}, fmt.Errorf(
			"generating certificate: %w",
			err,
		)
	}

	/* Save it for next time. */
	if "" != certFile {
		if err := SaveCertificate(certFile, certPEM, keyPEM); nil != err {
			return tls.Certificate{}, fmt.Errorf(
				"saving certificate to %s: %w",
				certFile,
				err,
			)
		}
	}

	return cert, err
}

// GenerateSelfSignedCertificate generates a bare-bones self-signed certificate
// with the given subject, DNS and IP Address SANs, and lifespan.  The
// certificate's Leaf will be set.  The certificate is also returned in PEM
// form.
func GenerateSelfSignedCertificate(subject string, dnsNames []string, ipAddresses []net.IP, lifespan time.Duration) (certPEM, keyPEM []byte, cert tls.Certificate, err error) {
	/* Make sure the cert will stay valid. */
	if 0 == lifespan {
		lifespan = DefaultSelfSignedCertLifespan
	}
	/* Generate it. */
	return generateSelfSignedCert(
		subject,
		dnsNames,
		ipAddresses,
		time.Now().Add(lifespan),
	)
}

// generateSelfSignedCert is like GenerateSelfSignedCert, but allows for an
// explicit expiry time, useful for testing.
func generateSelfSignedCert(
	subject string,
	dnsNames []string,
	ipAddresses []net.IP,
	notAfter time.Time,
) ([]byte, []byte, tls.Certificate, error) {
	/*
		Most of this inspired by
		https://github.com/golang/go/blob/46ea4ab5cb87e9e5d443029f5f1a4bba012804d3/src/crypto/tls/generate_cert.go#L7
	*/

	/* Make sure we have a subject. */
	if "" == subject {
		subject = SelfSignedSubject
	}

	/* Generate our private key. */
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if nil != err {
		return nil, nil, tls.Certificate{}, fmt.Errorf("generating key: %w", err)
	}

	/* Gather all the important data for the cert. */
	keyUsage := x509.KeyUsageDigitalSignature
	notBefore := time.Now()
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, tls.Certificate{}, fmt.Errorf(
			"generating serial number: %w",
			err,
		)
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{CommonName: subject},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     keyUsage,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ipAddresses,
	}

	/* Turn the certtificate into something the tls library can parse. */
	derBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&priv.PublicKey,
		priv,
	)
	if err != nil {
		return nil, nil, tls.Certificate{}, fmt.Errorf(
			"creating certificate: %w",
			err,
		)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	/* Key, as well. */
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, tls.Certificate{}, fmt.Errorf(
			"marshalling key: %w",
			err,
		)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	})

	/* Finally, parse back into a tls.Certificate. */
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if nil != err {
		return nil, nil, tls.Certificate{}, fmt.Errorf(
			"parsing PEM blocks into certificate: %w",
			err,
		)
	}

	/* Make sure Leaf is set. */
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if nil != err {
		return nil, nil, tls.Certificate{}, fmt.Errorf("parsing leaf: %w", err)
	}
	cert.Leaf = leaf

	return certPEM, keyPEM, cert, nil
}
