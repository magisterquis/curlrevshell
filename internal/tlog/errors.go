package tlog

/*
 * errors.go
 * Error values and types
 * By J. Stuart McMurray
 * Created 20251215
 * Last Modified 20251215
 */

import "errors"

// ErrBufferClosed is returned by [LogBuffer.Write] if the buffer was closed
// before saving a message.
var ErrBufferClosed = errors.New("buffer closed")

// ErrBufferEmpty indicates we were non-blocking reading from the buffer
// and it ran out of messages.
var ErrBufferEmpty = errors.New("buffer empty")

// ErrBufferFull is returned by [LogBuffer.Write] when its internal buffer is
// full.
var ErrBufferFull = errors.New("buffer full")
