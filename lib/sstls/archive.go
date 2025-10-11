package sstls

/*
 * archive.go
 * Read and Save certs with an archive file
 * By J. Stuart McMurray
 * Created 20240327
 * Last Modified 20251011
 */

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/tools/txtar"
)

// ErrCacheFileEmpty indicates the the file passed to LoadCachedCertificate was
// empty.
var ErrCacheFileEmpty = errors.New("cache file empty")

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
	openFile := func() (*os.File, error) {
		return os.OpenFile(certFile, os.O_CREATE|os.O_WRONLY, 0600)
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
	})); nil != err {
		return fmt.Errorf("writing certificate: %w", err)
	}

	return nil
}
