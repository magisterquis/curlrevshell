package hsrv

/*
 * errors.go
 * Error types and values
 * By J. Stuart McMurray
 * Created 20260801
 * Last Modified 20260801
 */

import (
	"errors"
	"fmt"
)

// ErrOneShellClosed indicates that the listener was closed as expected after
// receiving a single shell.
var ErrOneShellClosed = errors.New("closed after shell received")

// errNoCallbackAddresses indicates that New could not work out any callback
// addresses.
var errNoCallbackAddresses = errors.New("no callback addresses")

// listenError indicates New couldn't listen.
type listenError struct {
	Addr string
	Err  error
}

// Error satisfies the error interface.
func (err listenError) Error() string {
	return fmt.Sprintf("listening on %s: %v", err.Addr, err.Err)
}

// Unwrap returns err.Error.
func (err listenError) Unwrap() error { return err.Err }
