package testdatareader

/*
 * errors.go
 * Error types and values
 * By J. Stuart McMurray
 * Created 20260815
 * Last Modified 20260815
 */

import "fmt"

// ReadError is panic'd by [Reader.MustReadFile] on error.
type ReadError struct {
	Path string
	Err  error
}

// Error implements the error interface.
func (err ReadError) Error() string {
	return fmt.Sprintf("reading %s: %v", err.Path, err.Err)
}

// Unwrap returns err.Err.
func (err ReadError) Unwrap() error { return err.Err }
