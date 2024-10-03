package shellfuncsfile

/*
 * filter_shell.go
 * Fake filter for shell scripts
 * By J. Stuart McMurray
 * Created 20240706
 * Last Modified 20240707
 */

import (
	"io"
)

// FromShell returns the bytes read from r, under the assumption that r already
// has shell functions.
func FromShell(_ string, r io.Reader) ([]byte, error) { return io.ReadAll(r) }
