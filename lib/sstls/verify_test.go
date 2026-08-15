package sstls

/*
 * verify.go
 * Verify a TLS peer's fingerprint
 * By J. Stuart McMurray
 * Created 20260815
 * Last Modified 20260815
 */

import (
	"crypto/tls"
	"crypto/x509"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/internal/bufferedconn"
	"github.com/magisterquis/curlrevshell/internal/testdatareader"
	"github.com/magisterquis/curlrevshell/lib/tlog"
)

// testdataFS contains the testdata directory.
//
//go:embed testdata
var testdataFS embed.FS

// mustTDFile gets a test-specific file from testdataFS.
var mustTDFile = testdatareader.Reader{FS: testdataFS}.MustReadFile

// testCertAndFP returns an embedded test TLS certificate and its fingerprint.
var newTestCertAndFP = sync.OnceValues(func() (tls.Certificate, string) {
	/* Load the certificate. */
	fn := "test_cert.txtar"
	cert, err := loadCachedCertificate(fn, []byte(mustTDFile(nil, fn)))
	if nil != err {
		panic(fmt.Errorf("loading test certificate: %w", err))
	}

	/* Work out the fingerprint. */
	fp, err := PubkeyFingerprintTLS(cert)
	if nil != err {
		panic(fmt.Errorf(
			"calculating test certificate's fingerprint: %v",
			err,
		))
	}

	return cert, fp
})

// Does the verifier verify certificates?
func TestTLSCertificateVerifier(t *testing.T) {
	embeddedCert, embeddedFP := newTestCertAndFP()

	/* try tries a connection between a TLS server using embeddedCert and a
	TLS client expecting the fingerprint fp.  It checks if the client's
	error is wantCErr and the server's error is wantSErr, */
	try := func(t *testing.T, fp string, wantCErr, wantSErr error) {
		/* Verifier we're testing. */
		tv, err := TLSFingerprintVerifier(fp)
		if nil != err {
			t.Fatalf("Error generating verifier: %v", err)
		}

		/* Make some TLS comms happen to verify the verifier. */
		var (
			wg     sync.WaitGroup
			sc, cc = bufferedconn.NewPair()
		)
		wg.Go(func() {
			/* Send a byte to the server. */
			_, err := tls.Client(cc, &tls.Config{
				InsecureSkipVerify: true,
				VerifyConnection:   tv,
			}).Write(make([]byte, 1))
			/* Did it work, or at least not work correctly? */
			if got, want := err, wantCErr; !errors.Is(got, want) {
				t.Errorf(
					"Incorrect client error\n"+
						" got: %v\n"+
						"want: %v",
					got,
					want,
				)
			}
		})
		wg.Go(func() {
			/* Read a byte from the client. */
			_, err := tls.Server(sc, &tls.Config{
				Certificates: []tls.Certificate{embeddedCert},
			}).Read(make([]byte, 1))
			/* Did it work, or at least not work correctly? */
			if got, want := err, wantSErr; !errors.Is(got, want) &&
				got.Error() != want.Error() {
				t.Errorf(
					"Incorrect server error\n"+
						" got: %v\n"+
						"want: %v",
					got,
					want,
				)
			}
		})
		wg.Wait()
	}

	t.Run("correct_fingerprint", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			try(t, embeddedFP, nil, nil)
		})
	})

	t.Run("incorrect_fingerprint", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			/* Make an invalid fingerprint. */
			b := []byte(embeddedFP)
			copy(b, "invalid+fingerprint+")
			/* Did it not work? */
			try(
				t,
				string(b),
				ErrNoMatchingCertificate,
				errors.New(
					"remote error: tls: bad certificate",
				),
			)
		})
	})
}

// Do we get the right sort of errors with invalid fingerprints?
func TestTLSFingerprintVerifier_InvalidFingerprint(t *testing.T) {
	/* Do we whine if the fingerprint isn't the right size? */
	t.Run("invalid_size", func(t *testing.T) {
		var (
			have = "abcd"

			_, got = TLSFingerprintVerifier(have)
			want   = InvalidFingerprintSizeError(
				base64.StdEncoding.DecodedLen(len(have)),
			)
		)
		if !errors.Is(got, want) {
			t.Errorf(
				"Incorrect error\n got: %v\nwant: %v",
				got,
				want,
			)
		}
	})

	/* Do we whine if the fingerprint isn't base64? */
	t.Run("invalid_base64", func(t *testing.T) {
		var (
			have = "!" + tlog.S("invalid-base64")
			want = base64.CorruptInputError(0)

			_, got = TLSFingerprintVerifier(have)
		)
		if !errors.Is(got, want) {
			t.Errorf(
				"Incorrect error\n got: %v\nwant: %v",
				got,
				want,
			)
		}
	})
}

// Do we get the right sort of errors when verifying a cert with an invalid
// pubkey?
func TestTLSFingerprintVerifier_VerificationPubkeyError(t *testing.T) {
	/* Roll a verifier, such as it is. */
	tv, err := TLSFingerprintVerifier(
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	)
	if nil != err {
		t.Fatalf("Error rolling fingerprint verifier: %v", err)
	}
	/* Verify a cert with an invalid pubkey. */
	var (
		have = 123
		want = PeerCertificatePubkeyError{
			NCerts: 1,
			Idx:    1,
			Err: fmt.Errorf(
				"marshalling to DER: x509: "+
					"unsupported public key type: %T",
				have,
			),
		}
	)
	if got := tv(tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{{
			PublicKey: 123,
		}},
	}); got.Error() != want.Error() {
		t.Errorf("Incorrect error\n got: %v\nwant: %v", got, want)
	}

}
