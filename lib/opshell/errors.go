package opshell

/*
 * errors.go
 * Error types and values
 * By J. Stuart McMurray
 * Created 20260802
 * Last Modified 20260802
 */

import "errors"

// ErrInputDone is returned by [Shell.Do] to indicate its input closed.
var ErrInputDone = errors.New("input closed")

// ErrOutputClosed is returned by [Shell.Do] when it returns because someone
// closed the output channel.
var ErrOutputClosed = errors.New("output channel closed")
