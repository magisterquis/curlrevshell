package tlog

/*
 * errors.go
 * Error values and types
 * By J. Stuart McMurray
 * Created 20251215
 * Last Modified 20260321
 */

import (
	"errors"
	"fmt"
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
