package sstls

/*
 * archive.go
 * Read and Save certs with an archive file
 * By J. Stuart McMurray
 * Created 20240327
 * Last Modified 20240327
 */

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/tools/txtar"
)

// LoadCachedCertificate loads the certificate from the named file, which
// should have been created with SaveCertificate.
func LoadCachedCertificate(certFile string) (tls.Certificate, error) {
	/* Read the saved cert. */
	ta, err := txtar.ParseFile(certFile)
	if nil != err {
		return tls.Certificate{}, fmt.Errorf(
			"reading %s: %w",
			certFile,
			err,
		)
	}

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
		return tls.Certificate{}, fmt.Errorf(
			"PEM-encoded certificate missing",
		)
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

	/* Make sure Leaf is set. */
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if nil != err {
		return tls.Certificate{}, fmt.Errorf(
			"parsing read leaf: %w",
			err,
		)
	}
	cert.Leaf = leaf

	return cert, nil
}

// SaveCertificate saves PEM to the given file.  Directories will be created
// as needed with 0755 permissions.
func SaveCertificate(certFile string, certPEM, keyPEM []byte) error {
	/* Make needed directories. */
	dn := filepath.Dir(certFile)
	if err := os.MkdirAll(dn, 0700); nil != err {
		return fmt.Errorf("making directory %s: %w", dn, err)
	}
	/* Save the cert itself. */
	if err := os.WriteFile(certFile, txtar.Format(&txtar.Archive{
		Comment: []byte(fmt.Sprintf(
			"Generated %s",
			time.Now().Format(time.RFC3339),
		)),
		Files: []txtar.File{{
			Name: txtarCertFile,
			Data: certPEM,
		}, {
			Name: txtarKeyFile,
			Data: keyPEM,
		}},
	}), 0600); nil != err {
		return fmt.Errorf(
			"writing to %s: %w",
			certFile,
			err,
		)
	}

	return nil
}
