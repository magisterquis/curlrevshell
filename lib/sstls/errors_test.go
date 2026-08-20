package sstls

/*
 * errors_test.go
 * Tests for errors.go
 * By J. Stuart McMurray
 * Created 20260815
 * Last Modified 20260815
 */

import (
	"crypto/sha256"
	"strconv"
	"syscall"
	"testing"
)

// Do we whine about an invalid fingerprint size properly?
func TestInvalidFingerprintSizeErrorError(t *testing.T) {
	var (
		have = 123
		want = "decoded fingerprint " +
			strconv.Itoa(have) +
			" bytes, expected " +
			strconv.Itoa(sha256.Size)
	)
	if got := InvalidFingerprintSizeError(have).Error(); got != want {
		t.Errorf(
			"Error message incorrect\n got: %s\nwant: %s",
			got,
			want,
		)
	}
}

// Do we whine about an invalid peer pubkeys properly?
func TestPeerCertificatePubkeyError(t *testing.T) {
	var (
		have = PeerCertificatePubkeyError{
			NCerts: 10,
			Idx:    4,
			Err:    syscall.ENOMEM,
		}
		wantMsg = "getting fingerprint for peer certificate " +
			strconv.Itoa(have.Idx) + "/" +
			strconv.Itoa(have.NCerts) + ": " +
			have.Err.Error()
	)
	if got, want := have.Error(), wantMsg; got != want {
		t.Errorf(
			"Error message incorrect\n got: %s\nwant: %s",
			got,
			want,
		)
	}
	if got, want := have.Unwrap(), have.Err; got != want {
		t.Errorf(
			"Incorrect unwrapped error\n got: %v\nwant: %v",
			got,
			want,
		)
	}
}
