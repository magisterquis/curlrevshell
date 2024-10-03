package uu

/*
 * errors.go
 * Errors we might return
 * By J. Stuart McMurray
 * Created 20240928
 * Last Modified 20240928
 */

import (
	"errors"
	"fmt"
)

// ErrInvalidDataLen indicates the data part of the line was not a multiple of
// four characters.
var ErrInvalidDataLen = errors.New(
	"encoded data not a multiple of four characters",
)

// DecodeError indicates where in the encoded data decoding failed.
type DecodeError struct {
	Line, Offset int   /* Where the error occured. */
	Err          error /* What happened. */
}

// Error implements the error interface.
func (err DecodeError) Error() string {
	return fmt.Sprintf(
		"decoding failed at line %d, offset %d: %s",
		err.Line,
		err.Offset,
		err.Err,
	)
}

// Unwrap returns err.Err.
func (err DecodeError) Unwrap() error { return err.Err }

// InvalidLengthCharacterError indicates a line's length (first) character
// indicated a length less than 0.
type InvalidLengthCharacterError rune

// Error implements the error interface.
func (err InvalidLengthCharacterError) Error() string {
	return fmt.Sprintf("invalid length character %q", rune(err))
}

// InvalidEncodedCharacterError indicates a line contained an invalid
// character.
type InvalidEncodedCharacterError rune

// Error implements the error interface.
func (err InvalidEncodedCharacterError) Error() string {
	return fmt.Sprintf("invalid encoded character %q", rune(err))
}

// IncorrectDataLenError indicates the data part of the line was inconsistent
// with the length character.
type IncorrectDataLenError struct {
	Expected int
	Actual   int
}

// Error implements the error interface.
func (err IncorrectDataLenError) Error() string {
	return fmt.Sprintf(
		"incorrect encoded data length: expected %d, actual %d",
		err.Expected,
		err.Actual,
	)
}
