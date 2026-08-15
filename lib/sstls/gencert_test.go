package sstls

/*
 * gencert_test.go
 * Tests for gencert.go
 * By J. Stuart McMurray
 * Created 20240323
 * Last Modified 20260111
 */

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
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

	/* Make sure the archive doesn't have too much in it. */
	ar, err := txtar.ParseFile(certFile)
	if nil != err {
		t.Fatalf("Error parsing archive: %s", err)
	}
	if got := len(ar.Files); 2 != got {
		t.Errorf("Got %d files, expected 2", got)
	}
	var gotCertF, gotKeyF bool
	for i, f := range ar.Files {
		i++
		switch n := f.Name; n {
		case txtarCertFile:
			if gotCertF {
				t.Errorf("File %d is another cert file", i)
				break
			}
			gotCertF = true
		case txtarKeyFile:
			if gotKeyF {
				t.Errorf("File %d is another key file", i)
				break
			}
			gotKeyF = true
		default:
			t.Errorf("File %d has unexpected name %s", i, n)
		}
	}
	if !gotCertF {
		t.Errorf("Cert file not found")
	}
	if !gotKeyF {
		t.Errorf("Key file not found")
	}
	if t.Failed() {
		t.FailNow()
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
