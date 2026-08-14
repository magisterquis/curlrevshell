package tlog

/*
 * errors.go
 * Error values and types
 * By J. Stuart McMurray
 * Created 20251215
 * Last Modified 20260406
 */

import (
	"errors"
	"fmt"
	"slices"
)

// ErrBufferClosed is returned by [Buffer.Write] if the buffer was closed
// before saving a message.
var ErrBufferClosed = errors.New("buffer closed")

// ErrBufferEmpty indicates we were non-blocking reading from the buffer
// and it ran out of messages.
var ErrBufferEmpty = errors.New("buffer empty")

// ErrBufferFull is returned by [Buffer.Write] when its internal buffer is
// full.
var ErrBufferFull = errors.New("buffer full")

// leftoverLogMessageMessage notes a log message was left over when the buffer
// should have been empty.
type leftoverLogMessageMessage struct{ Msg string }

// String returns a string suitable for passing to t.Error.
func (l leftoverLogMessageMessage) String() string {
	return fmt.Sprintf("Leftover log message: %s", l.Msg)
}

// incorrectLogMessageMessage notes a log message was incorrect.
type incorrectLogMessageMessage struct {
	Idx  int /* 1-based index in the message list. */
	Len  int /* Length of message list. */
	Got  Msg
	Want Msg
}

// String returns a string suitable for passing to t.Error.
func (i incorrectLogMessageMessage) String() string {
	return fmt.Sprintf(
		"Incorrect log message %d/%d\n"+
			" got:\n%s\n"+
			"want:\n%s",
		i.Idx, i.Len,
		i.Got,
		i.Want,
	)
}

// messageSentAfterCloseMessage notes a log message which was sent after
// [Buffer.Close] was called.
type messageSentAfterCloseMessage struct{ Msg string }

// String returns a string suitable for passing to t.Error.
func (m messageSentAfterCloseMessage) String() string {
	return fmt.Sprintf("Log message sent after close:\n%s", m.Msg)
}

// unexpectedLogMessageMessage notes an unexpected log message.
type unexpectedLogMessageMessage struct{ Msg Msg }

// String returns a string suitable for passing to t.Error.
func (u unexpectedLogMessageMessage) String() string {
	return fmt.Sprintf("Unexpected log message:\n%s", u.Msg)
}

// unfoundLogMessageMessage notes we did not find a log message we expected.
type unfoundLogMessageMessage struct {
	Idx  int /* 1-based index in the message list. */
	Len  int /* Length of message list. */
	Want Msg
}

// String returns a string suitable for passing to t.Error.
func (u unfoundLogMessageMessage) String() string {
	return fmt.Sprintf(
		"Did not find log message %d/%d:\n%s",
		u.Idx, u.Len,
		u.Want,
	)
}

// errorWaitingForMessageMessage notes we encountered an error while waiting
// for a message.
type errorWaitingForMessageMessage struct {
	Idx int   /* 1-based index in the message list. */
	Len int   /* Length of message list. */
	Err error /* Underlying error. */
}

// String returns a string suitable for passing to t.Error.
func (e errorWaitingForMessageMessage) String() string {
	return fmt.Sprintf(
		"Error waiting for message %d/%d: %s",
		e.Idx, e.Len,
		e.Err,
	)
}

// groupPathMissingError is returned from Msg.UnmarshalJSON to indicate a group
// from msg.Path was missing.
type groupPathMissingError struct{ Path []string }

// Error implements the error interface.
func (err groupPathMissingError) Error() string {
	return fmt.Sprintf("group path %q missing", err.Path)
}

// Is indicates if target is a groupPathMissingError with the same slice
// contents.
func (err groupPathMissingError) Is(target error) bool {
	t, ok := errors.AsType[groupPathMissingError](target)
	if !ok {
		return false
	}
	return slices.Equal(err.Path, t.Path)
}

// groupPathLeadsToNotGroupError is returned from Msg.UnmarshalJSON to
// indicate a group from msg.Path pointed to a value, not a group.
type groupPathLeadsToNotGroupError struct {
	Path []string
	Type string
}

// newGroupPathLeadsToNotGroupError returns a new
// groupPathLeadsToNotGroupError for a path leading to a value.
// path is not retained.
func newGroupPathLeadsToNotGroupError(
	path []string,
	val any,
) groupPathLeadsToNotGroupError {
	return groupPathLeadsToNotGroupError{
		Path: slices.Clone(path),
		Type: fmt.Sprintf("%T", val),
	}
}

// Error implements the error interface.
func (err groupPathLeadsToNotGroupError) Error() string {
	return fmt.Sprintf(
		"group path %q leads to a value of type %s, not a group",
		err.Path,
		err.Type,
	)
}

// Is indicates if target is a groupPathMissingError with the same slice
// contents.
func (err groupPathLeadsToNotGroupError) Is(target error) bool {
	t, ok := errors.AsType[groupPathLeadsToNotGroupError](target)
	if !ok {
		return false
	}
	return err.Type == t.Type && slices.Equal(err.Path, t.Path)
}
