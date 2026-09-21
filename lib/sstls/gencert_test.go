package sstls

/*
 * gencert_test.go
 * Tests for gencert.go
 * By J. Stuart McMurray
 * Created 20240323
 * Last Modified 20260817
 */

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/tools/txtar"
)

func TestGenerateSelfSignedCertificate(t *testing.T) {
	now := time.Now().Round(time.Second)
	for _, c := range []struct {
		subject     string
		dnsNames    []string
		ipAddresses []net.IP
		start       time.Time
		expiry      time.Time
	}{{
		subject: "kittens",
	}, {
		subject: "",
	}, {
		subject:  "kittens.com",
		dnsNames: []string{"kittens.com", "*.moose.com", "*"},
		ipAddresses: []net.IP{
			net.IPv4(1, 2, 3, 4),
			net.IPv4(0, 0, 0, 0),
			net.ParseIP("::"),
			net.ParseIP("a::b"),
		},
		start:  now,
		expiry: now.Add(time.Minute),
	}} {
		t.Run(c.subject, func(t *testing.T) {
			_, _, g, err := generateSelfSignedCert(
				c.subject,
				c.dnsNames,
				c.ipAddresses,
				c.start,
				c.expiry,
			)
			if nil != err {
				t.Fatalf("Generation failed: %s", err)
			}

			if n := len(g.Certificate); 1 != n {
				t.Errorf("Expected 1 certificate, got %d", n)
			}

			if nil == g.Leaf {
				t.Fatalf("Leaf is nil")
			}

			want := c.subject
			if "" == want {
				want = SelfSignedSubject
			}
			want = "CN=" + want
			if got := g.Leaf.Subject.String(); want != got {
				t.Errorf(
					"Subject incorrect\n"+
						" got: %s\n"+
						"want: %s",
					got,
					want,
				)
			}

			if !slices.Equal(g.Leaf.DNSNames, c.dnsNames) {
				t.Errorf(
					"DNSNames incorrect:\n"+
						" got: %s\n"+
						"want: %s",
					g.Leaf.DNSNames,
					c.dnsNames,
				)
			}

			if !slices.EqualFunc(
				g.Leaf.IPAddresses,
				c.ipAddresses,
				func(a, b net.IP) bool { return a.Equal(b) },
			) {
				t.Errorf(
					"IPAddresses incorrect:\n"+
						" got: %s\n"+
						"want: %s",
					g.Leaf.IPAddresses,
					c.ipAddresses,
				)
			}

			/* Is the notBefore time correct? */
			if got, want := g.Leaf.NotBefore.UTC(),
				c.start.UTC(); !got.Equal(want) {
				t.Errorf(
					"Start time incorrect:\n"+
						" got: %s\n"+
						"want: %s",
					got,
					want,
				)
			}

			/* Is the notAfter time correct? */
			if got, want := g.Leaf.NotAfter.UTC(),
				c.expiry.UTC(); !got.Equal(want) {
				t.Errorf(
					"Expiry incorrect:\n"+
						" got: %s\n"+
						"want: %s",
					got,
					want,
				)
			}
		})
	}
}

func TestGetCertificate(t *testing.T) {
	var (
		certFile = filepath.Join(t.TempDir(), "kittens")
		subject  = "moose"
	)

	/* Generate a new certificate. */
	genC, err := GetCertificate(subject, nil, nil, 0, certFile)
	if nil != err {
		t.Fatalf("Error generating certificate: %s", err)
	}
	if nil == genC.Leaf {
		t.Errorf("Leaf on generated certificate is nil")
	}

	/* Make sure the archive has the files we expect. */
	ar, err := txtar.ParseFile(certFile)
	if nil != err {
		t.Fatalf("Error parsing archive: %s", err)
	}
	var gotNames []string
	for _, f := range ar.Files {
		gotNames = append(gotNames, f.Name)
	}
	if got, want := gotNames, []string{
		txtarFingerprintFile,
		txtarCertFile,
		txtarKeyFile,
	}; !slices.Equal(got, want) {
		t.Fatalf(
			"Archive has incorrect file names\n got: %v\nwant: %v",
			got,
			want,
		)
	}

	/* Re-read the archive. */
	readC, err := GetCertificate("dummy", nil, nil, 0, certFile)
	if nil != err {
		t.Fatalf("Error reading cert file: %s", err)
	}
	if nil == readC.Leaf {
		t.Errorf("Leaf on read certificate is nil")
	}

	/* Make sure it's the same certificate. */
	if !readC.Leaf.Equal(genC.Leaf) {
		t.Errorf("Generated and Read leaves not equal")
	}

	/* Make sure the fingerprint is correct. */
	genFP, err := PubkeyFingerprintTLS(genC)
	if nil != err {
		t.Fatalf(
			"Error calculating generated certificate's "+
				"fingerprint: %v",
			err,
		)
	}
	if got, want := strings.TrimRight(string(ar.Files[0].Data), "\n"),
		genFP; got != want {
		t.Fatalf(
			"Archive had incorrect fingerprint\n"+
				"got: %s\n"+
				"want: %s",
			got,
			want,
		)
	}
}

// If the cert cache file isn't usable, can we overwrite properly?
func TestGetCertificate_OverwriteOldCert(t *testing.T) {
	/* Generate a valid archive file, for size. */
	fn := filepath.Join(t.TempDir(), "c.txtar")
	if _, err := GetCertificate(
		SelfSignedSubject,
		nil,
		nil,
		DefaultSelfSignedCertLifespan,
		fn,
	); nil != err {
		t.Fatalf("Error generating new archive: %s", err)
	}

	/* Turn it into invalid data. */
	b, err := os.ReadFile(fn)
	if nil != err {
		t.Fatalf("Error reading new archive: %s", err)
	} else if 0 == len(b) {
		t.Fatalf("New archive is empty")
	}
	aLen := len(b)
	f, err := os.Create(fn)
	if nil != err {
		t.Fatalf("Error opening new archive for writing")
	}
	defer f.Close()
	nw := 0
	for nw < aLen*2 {
		n, err := fmt.Fprintf(f, "%d\n", nw)
		if nil != err {
			t.Fatalf("Error writing to archive: %s", err)
		}
		nw += n
	}
	f.Close()

	/* Did write enough? */
	if b, err = os.ReadFile(fn); nil != err {
		t.Fatalf("Error reading archive after write: %s", err)
	} else if got, want := len(b), aLen*2; got < want {
		t.Fatalf(
			"Did not write enough junk data\n got: %d\nwant: >=%d",
			got,
			want,
		)
	}

	/* Read/Update again, should overwrite with a valid archive. */
	if _, err = GetCertificate(
		SelfSignedSubject,
		nil,
		nil,
		DefaultSelfSignedCertLifespan,
		fn,
	); nil != err {
		t.Fatalf("Error getting certificate after write: %s", err)
	}

	/* Did it shrink and have a cert? */
	if b, err := os.ReadFile(fn); nil != err {
		t.Fatalf("Error reading archive after rewrite: %s", err)
	} else if got, want := len(b), nw; got >= want {
		t.Fatalf(
			"Archive did not shrink after rewrite\n"+
				" got: %d\n"+
				"want: <%d",
			got,
			want,
		)
	}
}

// Do we barf if we try to read the certificate from a file that's actually a
// directory?
func TestGetCertificate_DirectoryPath(t *testing.T) {
	td := t.TempDir()
	_, err := GetCertificate("", nil, nil, 0, td)
	if got, want := err, syscall.EISDIR; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error passing GetCertificate a directory\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
}

// Do we barf if we try to write to a read-only directory?
func TestGetCertificate_ReadOnlyDirectory(t *testing.T) {
	td := filepath.Join(t.TempDir(), "d")
	if err := os.MkdirAll(td, 0500); nil != err {
		t.Fatalf("Error changing directory permissions: %v", err)
	}
	fn := filepath.Join(td, "f")
	_, err := GetCertificate("", nil, nil, 0, fn)
	if got, want := err, syscall.EACCES; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error saving to a read-only file\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
}
