package sstls

/*
 * archive.go
 * Read and Save certs with an archive file
 * By J. Stuart McMurray
 * Created 20240327
 * Last Modified 20260817
 */

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/tools/txtar"
)

// Names of files in a txtar archive for the PEM-encoded cert and key.
const (
	txtarCertFile        = "cert"
	txtarFingerprintFile = "fingerprint"
	txtarKeyFile         = "key"
)

// LoadCachedCertificate loads the certificate from the named file, which
// should have been created with SaveCertificate.
func LoadCachedCertificate(certFile string) (tls.Certificate, error) {
	b, err := os.ReadFile(certFile)
	if nil != err {
		return tls.Certificate{}, fmt.Errorf(
			"reading %s: %w",
			certFile,
			err,
		)
	}
	return loadCachedCertificate(certFile, b)
}

// loadCachedCertificate does what LoadCachedCertificate says it does but
// from a pre-loaded file.  certFile is only used in error messages.
func loadCachedCertificate(certFile string, b []byte) (tls.Certificate, error) {
	/* Read the saved cert. */
	ta := txtar.Parse(b)

	/* Grab the important files. */
	var certB, keyB []byte
	for _, f := range ta.Files {
		switch f.Name {
		case txtarCertFile:
			certB = f.Data
		case txtarKeyFile:
			keyB = f.Data
		}
	}

	/* Try to use it. */
	if 0 == len(certB) {
		return tls.Certificate{}, ErrCacheFileEmpty
	} else if 0 == len(keyB) {
		return tls.Certificate{}, fmt.Errorf(
			"PEM-encoded key missing",
		)
	}
	cert, err := tls.X509KeyPair(certB, keyB)
	if nil != err {
		return tls.Certificate{}, fmt.Errorf(
			"loading certificate from %s: %w",
			certFile,
			err,
		)
	}

	/* Make sure Leaf is set.  At one point, tls.X509KeyPair didn't
	always do this, but it should now unless someone set
	GODEBUG=x509keypairleaf=0. */
	if nil == cert.Leaf {
		return tls.Certificate{}, ErrLeafCertificateNotSet
	}

	return cert, nil
}

// SaveCertificate saves PEM to the given file.  Directories will be created
// as needed with 0755 permissions.
func SaveCertificate(certFile string, certPEM, keyPEM []byte) error {
	/* Work out the fingerprint.  Also validates the cert for us. */
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if nil != err {
		return fmt.Errorf("parsing certificate: %w", err)
	}
	fp, err := PubkeyFingerprintTLS(cert)
	if nil != err {
		return fmt.Errorf("calculating fingerprint: %w", err)
	}

	/* Do what we said we'd do in the first place. */
	return saveCertificateFingerprint(certFile, fp, certPEM, keyPEM)
}

// saveCertificateFingerprint is like SaveCertificate, but with a
// pre-calculated fingerprint to avoid having to parse fresh PEM.
func saveCertificateFingerprint(
	certFile string,
	fingerprint string,
	certPEM []byte,
	keyPEM []byte,
) error {
	/* openFile tries to open certFile. */
	openFile := func() (*os.File, error) {
		return os.OpenFile(
			certFile,
			os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
			0600,
		)
	}
	/* Try opening the file.  If we don't have enough directories it'll
	fail and we'll try again. */
	f, err := openFile()
	if errors.Is(err, syscall.ENOENT) {
		/* Don't have all the directories. */
		dn := filepath.Dir(certFile)
		if err := os.MkdirAll(dn, 0700); nil != err {
			return fmt.Errorf("making directory %s: %w", dn, err)
		}
		/* Try again. */
		f, err = openFile()
	}
	if nil != err {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	/* Save the cert itself. */
	if _, err := f.Write(txtar.Format(&txtar.Archive{
		Comment: fmt.Appendf(nil,
			"Generated %s",
			time.Now().Format(time.RFC3339),
		),
		Files: []txtar.File{{
			Name: txtarFingerprintFile,
			Data: []byte(fingerprint),
		}, {
			Name: txtarCertFile,
			Data: certPEM,
		}, {
			Name: txtarKeyFile,
			Data: keyPEM,
		}},
	})); nil != err {
		return fmt.Errorf("writing certificate: %w", err)
	}

	return nil
}
