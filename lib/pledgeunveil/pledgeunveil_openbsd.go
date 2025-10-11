// Package pledgeunveil - pledge(2) and unveil(2) on OpenBSD, no-ops elsewhere
package pledgeunveil

/*
 * pledgeunveil_openbsd.go
 * pledge(2) and unveil(2) on OpenBSD, no-ops elsewhere
 * By J. Stuart McMurray
 * Created 20251010
 * Last Modified 20251010
 */

import "golang.org/x/sys/unix"

// Pledge is a thin wrapper around [unix.Pledge]
// on OpenBSD and a no-op elsewhere.
func Pledge(promises, execpromises string) error {
	return unix.Pledge(promises, execpromises)
}

// PledgeExecpromises is a thin wrapper around [unix.PledgeExecpromises]
// on OpenBSD and a no-op elsewhere.
func PledgeExecpromises(execpromises string) error {
	return unix.PledgeExecpromises(execpromises)
}

// PledgePromises is a thin wrapper around [unix.PledgePromises]
// on OpenBSD and a no-op elsewhere.
func PledgePromises(promises string) error {
	return unix.PledgePromises(promises)
}

// Unveil is a thin wrapper around [unix.Unveil]
// on OpenBSD and a no-op elsewhere.
func Unveil(path string, flags string) error {
	return unix.Unveil(path, flags)
}

// UnveilBlock is a thin wrapper around [unix.UnveilBlock]
// on OpenBSD and a no-op elsewhere.
func UnveilBlock() error {
	return unix.UnveilBlock()
}

//go:generate ./gen_other.pl $GOFILE
//go:generate gofmt -w .
