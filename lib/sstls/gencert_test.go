package sstls

/*
 * gencert_test.go
 * Tests for gencert.go
 * By J. Stuart McMurray
 * Created 20240323
 * Last Modified 20240327
 */

import (
	"net"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"golang.org/x/tools/txtar"
)

func TestGenerateSelfSignedCertificate(t *testing.T) {
	for _, c := range []struct {
		subject     string
		dnsNames    []string
		ipAddresses []net.IP
		expiry      time.Time
	}{{
		subject: "kittens",
	}, {
		subject: "",
	}, {
		subject:  "kittens.com",
		dnsNames: []string{"kittens.com", "*.moose.com", ".", "*"},
		ipAddresses: []net.IP{
			net.IPv4(1, 2, 3, 4),
			net.IPv4(0, 0, 0, 0),
			net.ParseIP("::"),
			net.ParseIP("a::b"),
		},
		expiry: time.Now().Add(time.Minute),
	}} {
		c := c /* :C */
		t.Run(c.subject, func(t *testing.T) {
			_, _, g, err := generateSelfSignedCert(
				c.subject,
				c.dnsNames,
				c.ipAddresses,
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

			gt := g.Leaf.NotAfter.UTC()
			wt := c.expiry.UTC().Truncate(time.Second)
			if !gt.Equal(wt) {
				t.Errorf(
					"Expiry incorrect:\n"+
						"got: %s\n"+
						"want: %s",
					gt,
					wt,
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
