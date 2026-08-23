package sstls

/*
 * archive_test.go
 * Read and Save certs with an archive file
 * By J. Stuart McMurray
 * Created 20240327
 * Last Modified 20260823
 */

import (
	"crypto/x509"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Can we read an RSA certificate?
func TestGetCertificate_RSA(t *testing.T) {
	/* Generated with
	openssl genrsa -out crs_key.pem 2048
	openssl req -new -x509 -key crs_key.pem -days 36500 -nodes \
		-subj /CN=old_crs -out crs_cert.pem
	*/
	oldCert := `-- cert --
-----BEGIN CERTIFICATE-----
MIICojCCAYoCCQDGzTWqvAbw6DANBgkqhkiG9w0BAQsFADASMRAwDgYDVQQDDAdv
bGRfY3JzMCAXDTI2MDExMTEzMzcxMVoYDzIxMjUxMjE4MTMzNzExWjASMRAwDgYD
VQQDDAdvbGRfY3JzMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAzOi6
bG2OM9nGmaE7X6vFPHojZF7N8QJIdcKXp73uEHHzTHqBUzQ++0rgGUmO2yQ/+yII
hX3Rqej79KaKn2CjhdT90n87rgzNzOURw+KoTmh8HmgeT9mLP0Pg8N5MYHkZgBgG
cTyIpToUAHtC+EH+he94Cv78FRFsDpGoZwZA0x3J/bFsuJOWC1+IbDmfWk7NOfxf
qaL+Eo6jFEcVbptSvi9OlpsxrwMLpzXTV8mt/U0W00hpctUivJSk+6fMUgBOh9Al
2n0P1DzZfUP0USnePeKeGeP4ADDmjtte9Eqlg6Pj25mlCU0GuJSZ0IhTfYRZt0XI
Bs/AoblT5mhUIygVWwIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQBf0T8b+tyZeaEz
86pNl0u7phEn5AYhJXnQQw0uvrgkDNGduxrC+2ezX3+yjQF/VhX1bZNMlLOGy+Ei
XISmG9QdyCa33Og++fxx4eOQroUM2F0P2bDPe16YrPxiND6NlU8FQDOf51P7dhwI
n6ZQvX91yTuWD6WPNPbl9qwfcrBF2uhIGtMGDkZy+BjUKTDlqVyVYUYgv9S0OCp2
2wfg06DgJL/cCprHcvkdE1sfXqUosGR+O1lan7kPPvNK06p2bKBLXRJLFAtycVLq
aLfjSWWUJovlEx7VteedDdRzjaIJN3CNxhu0+OnFjFWZarX+Tm1sowK20QTPmsWL
QUJneycb
-----END CERTIFICATE-----
-- key --
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAzOi6bG2OM9nGmaE7X6vFPHojZF7N8QJIdcKXp73uEHHzTHqB
UzQ++0rgGUmO2yQ/+yIIhX3Rqej79KaKn2CjhdT90n87rgzNzOURw+KoTmh8Hmge
T9mLP0Pg8N5MYHkZgBgGcTyIpToUAHtC+EH+he94Cv78FRFsDpGoZwZA0x3J/bFs
uJOWC1+IbDmfWk7NOfxfqaL+Eo6jFEcVbptSvi9OlpsxrwMLpzXTV8mt/U0W00hp
ctUivJSk+6fMUgBOh9Al2n0P1DzZfUP0USnePeKeGeP4ADDmjtte9Eqlg6Pj25ml
CU0GuJSZ0IhTfYRZt0XIBs/AoblT5mhUIygVWwIDAQABAoIBAHTWqx4SZghIwAZv
ugQ2VJPEbRPZPwKSs7B6EbFzCiDUaM+N9tnzq4nsStYAbHWmONlAsa00be29TJVW
tSpllzhDl6uEIwp+gIa5gyS3xBJZX5SS992+BLlBQiz2BITp6FFy4ZGF28Ci2i7g
GfKm5rOGNWPHuwNkWMIB6g08so/toj5KdLkCyFOc4OhlDIcJqoy4v7bokqCXbRva
OOK8frbaHsdKd2PzJDcBoHSxJ3DwBwDiV//k2bFsZ+tOxq4deCXKCBOykbfYPm37
Bp5qiTmBSuvua4YFC3Gd3Q/cRRs2o4vujbNrK8aJHyCOfwGh7hUb5B6RGTWnMBTV
we4QCykCgYEA58a6fkcyn3V0/MthhJ4H9ayoLRCTgcK5qpJi9ErsrWRSEXqi3Z4s
gPalXx+Kl8VW1nas6hK4J+aCgwox/1R7Rg7MNyInDU8p1UvUknS2x7aLkzvZTkT5
ni1sMovtMEOzykelk6g0MF/Ov9BkE4boM2szbMt/UuvOgldxhXvmaVcCgYEA4lMo
JyKe7nHDoWXK1BktJfKZaAAquEtn885T7Rh+GXde8ZCdDZ5D14gnV5M+355ji9ed
R44dsfNpSJbiNieTD7LFgsGpfQa3G1w8CG4uaAerMTjpmVI0wi/CK7C0gj+2rElU
QniKZ7mun5132cdnJgJYgwRPKaiRz4KNSgGmfZ0CgYAh2FEvU3I++sXkjEZnOTRs
WZQNUJhZoHfAQrQUfERnZXjeeIwD1K8m/d1UMKTqWLS/iIDjhWxC11RDkl+Oq2V8
63hCrMgLF35CWVpnMIfoTe2/yEOJPGU/Bd8A2pH+NESyyxeQokVMsxDbzbBvcYnC
yGqv/l9PWoPDYMWA+oDA7QKBgQCbw+HBtYku0Kt0vGsxOLajBGnicyLzviooWVvX
tWCsRETa+s2snr1QbIuvMU83dnpKt7WulrgHTmCqrfW4kdocFszq5kCxJNsHAJ7s
qnBT9tsywFm9xR88esQnb7F8Zz9hKgLM0MtpAhmWDzl6cEuklD64xVF6eWMJL2/w
fFxK3QKBgQCat7rhIlNOs+RNNH2unSj8uWxdaPZBM6MtG9As1qXS0ugxXv7f9zbb
SkvDHB6sUb0zONkKushI25EMXdSN1ZAR7NkTd+rXs4e2IBVhgCvE79IBjLALJd2Z
JG5s0M8Tmb6rrGNjtjllePzYPqYMP0OManuu+ZTWjpiDcO+NyOmO8Q==
-----END RSA PRIVATE KEY-----`

	/* Cert cache file with an unusable cert. */
	fn := filepath.Join(t.TempDir(), "c.txtar")
	if err := os.WriteFile(fn, []byte(oldCert), 0600); nil != err {
		t.Fatalf("Error writing old certificate to %s: %s", fn, err)
	}

	/* Try to read from it. */
	cert, err := GetCertificate(
		SelfSignedSubject,
		nil,
		nil,
		DefaultSelfSignedCertLifespan,
		fn,
	)
	if nil != err {
		t.Fatalf("Error getting/generating certificate: %s", err)
	}

	/* Did the file contents change? */
	if b, err := os.ReadFile(fn); nil != err {
		t.Errorf(
			"Error reading cache file after GetCertificate: %s",
			err,
		)
	} else if got, want := string(b), oldCert; got != want {
		t.Errorf(
			"Cache file changed after GetCertificate\n"+
				"got\n%s\n"+
				"want\n%s",
			got,
			want,
		)
	}

	/* Did we get the right cert? */
	l := cert.Leaf
	if nil == l {
		t.Fatalf("Certificate's leaf unavailable")
	}
	if got, want := l.Subject.CommonName, "old_crs"; got != want {
		t.Errorf(
			"Incorrect common name\n got: %s\nwant: %s",
			got,
			want,
		)
	}
	if got, want := l.SignatureAlgorithm, x509.SHA256WithRSA; got != want {
		t.Errorf(
			"Incorrect signature algorithm\n got: %s\nwant: %s",
			got,
			want,
		)
	}
}

// Does SaveCertificate truncate the file first, to prevent junk at the end?
func TestSaveCertificate_TruncateFile(t *testing.T) {
	/* Make a cert to save. */
	cert, key, _, err := GenerateSelfSignedCertificate(
		SelfSignedSubject,
		nil,
		nil,
		DefaultSelfSignedCertLifespan,
	)
	if nil != err {
		t.Fatalf("Error generating certificate: %s", err)
	}

	/* Make a file larger than SaveCertificate with write. */
	fn := filepath.Join(t.TempDir(), "c.txtar")
	f, err := os.Create(fn)
	if nil != err {
		t.Fatalf("Error creating archive file: %s", err)
	}
	defer f.Close()
	var sz int
	for _, v := range [][]byte{cert, key, cert, key} {
		n, err := f.Write(v)
		if nil != err {
			t.Fatalf("Error writing junk to archive: %s", err)
		}
		sz += n
	}
	f.Close()

	/* Write it as an archive, should truncate. */
	if err := SaveCertificate(fn, cert, key); nil != err {
		t.Fatalf("Error writing archive: %s", err)
	}
	if b, err := os.ReadFile(fn); nil != err {
		t.Fatalf(
			"Error reading archive after saving certificate: %s",
			err,
		)
	} else if got, want := len(b), sz; got >= want {
		t.Errorf(
			"File did not shrink after SaveCertificate\n"+
				"got: %d\n"+
				"want: <%d",
			got,
			want,
		)
	}
}

// Do we get the right errors parsing invalid certificates?
func TestParseCachedCertificate_Errors(t *testing.T) {
	for n, want := range map[string]error{
		"empty_cert": ErrCacheFileEmpty,
		"empty_key":  ErrPrivateKeyPEMEmpty,
	} {
		t.Run(n, func(t *testing.T) {
			if _, got := ParseCachedCertificate(
				[]byte(mustTDFile(t)),
			); !errors.Is(got, want) {
				t.Errorf(
					"Incorrect parse error\n"+
						" got: %v\n"+
						"want: %v",
					got,
					want,
				)
			}
		})
	}
}

// Do we get the right sort of error if we can't parse the cert's PEM?
func TestParseCcahedCertificate_ParseError(t *testing.T) {
	_, err := ParseCachedCertificate([]byte(mustTDFile(t)))
	/* tls library gives us a string :( */
	if got, want := err.Error(), "parsing certificate: tls: failed to "+
		"find any PEM data in certificate input"; got != want {
		t.Errorf(
			"Incorrect parse error\n got: %v\nwant: %v",
			got,
			want,
		)
	}

}
