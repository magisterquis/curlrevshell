package adsrv

/*
 * errors.go
 * Error types and values
 * By J. Stuart McMurray
 * Created 20260808
 * Last Modified 20260808
 */

import (
	"fmt"
)

// ErrUnspecifiedConnType indicates an adapter requested a connection without
// a ConnType.
var ErrUnspecifiedConnType = fmt.Errorf("unspecified connection type")

// ErrConnRequestTimeout indicates a connection request took too long.
var ErrConnRequestTimeout = fmt.Errorf("timeout reading conection request")

// UnknownConnTypeError indicates an adapter requested a connection with an
// unknown ConnType.
type UnknownConnTypeError struct {
	ConnType ConnType
}

// Error implements the error interface.
func (err UnknownConnTypeError) Error() string {
	return fmt.Sprintf("unknown connection type: %s", err.ConnType)
}

// UnknownShellStreamDirectionError indicates an adapter requested a shell
// stream with an unknown direction.
type UnknownShellStreamDirectionError struct {
	Direction ShellStreamDirection
}

// Error implements the error interface.
func (err UnknownShellStreamDirectionError) Error() string {
	return fmt.Sprintf("unknown shell stream direction: %s", err.Direction)
}

// ConnResponseError is an error type wrapping a ConnResponse.
type ConnResponseError struct {
	ConnResponse
}

// Error returns err's underlying string, which may be the empty string.
func (err ConnResponseError) Error() string { return err.ConnResponse.Error }

// Is indicates whether err matches target.  If target is nil and err's
// underlying string is nil, Is returns true.  In general, it is preferable to
// use [ConnResponse.ToError] than wrap ConnResponse in a ConnResponseError
// directly.
func (err ConnResponseError) Is(target error) bool {
	if nil == target && "" == err.ConnResponse.Error {
		return true
	}
	cr, ok := target.(ConnResponseError)
	return ok && err.ConnResponse.Error == cr.ConnResponse.Error
}
