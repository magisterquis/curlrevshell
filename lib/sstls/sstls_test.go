package sstls

/*
 * sstls_test.go
 * Tests for sstls.go
 * By J. Stuart McMurray
 * Created 20241003
 * Last Modified 20241003
 */

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPubkeyFingerprintTLS(t *testing.T) {
	have := `Generated 2024-10-03T19:50:58+02:00
-- cert --
-----BEGIN CERTIFICATE-----
MIIBUjCB+aADAgECAhBmy03RSxd0caoLCYIzVljhMAoGCCqGSM49BAMCMBAxDjAM
BgNVBAMTBXNzdGxzMB4XDTI0MTAwMzE3NTA1OFoXDTM0MTAwMzE3NTA1OFowEDEO
MAwGA1UEAxMFc3N0bHMwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAR4JbcAP7Ha
+zMSkfk42HCKMsmm8Qg6/9ZNGfza/NK+wy1yx0GwTwpA5bBOMZGvM27lVB+kDV1f
gupC97Ek3V6tozUwMzAOBgNVHQ8BAf8EBAMCB4AwEwYDVR0lBAwwCgYIKwYBBQUH
AwEwDAYDVR0TAQH/BAIwADAKBggqhkjOPQQDAgNIADBFAiEA9MJjfcQjZ2UmAjb6
AF7HJMmXWJ3OktK38HebcpXYXI0CIHY6ehcD2lEcOMQ5H2h67OlHmz5atnUkRS9w
rC3ITJ+P
-----END CERTIFICATE-----
-- key --
-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgE+sExMWiQBCaNzhV
OZbzJTplSR7Z0Usht5JCXIJ+T7ChRANCAAR4JbcAP7Ha+zMSkfk42HCKMsmm8Qg6
/9ZNGfza/NK+wy1yx0GwTwpA5bBOMZGvM27lVB+kDV1fgupC97Ek3V6t
-----END PRIVATE KEY-----
`
	want := "mHnXq08GRE7Iqv/CGMAvD24tXU2URsoio8mpQwht2Og="

	/* Write the txtar to a file. */
	fn := filepath.Join(t.TempDir(), "cert.txtar")
	if err := os.WriteFile(fn, []byte(have), 0600); nil != err {
		t.Fatalf("Error writing certificate to %s: %s", fn, err)
	}

	/* Load it as a cert. */
	cert, err := LoadCachedCertificate(fn)
	if nil != err {
		t.Fatalf("Error loading certificate from %s: %s", fn, err)
	}

	/* Make sure it hashes nicely. */
	if got, err := PubkeyFingerprintTLS(cert); nil != err {
		t.Fatalf("Error getting certificate fingerprint: %s", err)
	} else if got != want {
		t.Errorf("Incorrect hash\n got: %s\nwant: %s", got, want)
	}

}
