// Package crsadapter - Bidirectional stream of JSON objects.
package crsadapter

/*
 * jsonstream.go
 * Bidirectional stream of JSON objects
 * By J. Stuart McMurray
 * Created 20260808
 * Last Modified 20260823
 */

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"

	"github.com/magisterquis/curlrevshell/internal/chanmutex"
)

// ErrDecodeAfterRead is returned by [Stream.DecodeNext] if called after a
// call to [Stream.Read].
var ErrDecodeAfterRead = errors.New(
	"attempt to decode JSON value after reading raw bytes",
)

// Streamer is the interface Stream wraps.  In particular, [net.UnixConn]
// satisfies Streamer.
type Streamer interface {
	io.ReadWriteCloser
	CloseRead() error
	CloseWrite() error
}

// Stream represents a connection to Curlrevshell's adapter interface.
// It provides a bidirectional stream of JSON objects but may also be used for
// Sending and receiving non-JSON data with the caveat that its built-in JSON
// object decoder is no longer usable after the first non-JSON read.
// Stream's methods are generally not safe for concurrent use, with the
// exception that calls to Write/Send may be called concurrently with calls to
// Read/DecodeNext.
// JSON objects are expected to be newline-terminated.
type Stream struct {
	Streamer

	/* Read side. */
	mu   chanmutex.Mutex /* For synctesting. */
	dec  *jsontext.Decoder
	r    io.Reader /* UnixConn plus unread buffer. */
	rErr error     /* When switching to read left us in a funny state. */

	/* Write side. */
	enc *jsontext.Encoder
}

// NewStream returns a new Stream wrapping s.
func NewStream(b Streamer) *Stream {
	return &Stream{
		Streamer: b,
		mu:       chanmutex.New(),
		dec:      jsontext.NewDecoder(b),
		enc: jsontext.NewEncoder(
			b,
			jsontext.ReorderRawObjects(true),
		),
	}
}

// DecodeNext attempts to decode the next JSON message into v.  Do not call
// DecodeNext after the first call to Read.
func (j Stream) DecodeNext(v any) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	/* Make sure nobody called Read. */
	if nil != j.r {
		return ErrDecodeAfterRead
	}

	/* Decode. */
	return json.UnmarshalDecode(j.dec, v)
}

// Read reads plain bytes from j, bypassing JSON decoding.  Do not call
// concurrently with DecodeNext or another call to Read.
// As Stream is indended for newline-separated messages, if a newline was sent
// after the last JSON message, it will not be returned.
func (j *Stream) Read(b []byte) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	/* IF this has already failed, easy day. */
	if nil != j.rErr {
		return 0, j.rErr
	}

	/* If this is our first read, turn off JSON and make the remaining
	unbuffered bytes available. */
	if nil == j.r {
		/* Buffered data, but maybe remove the initial newline. */
		buf := j.dec.UnreadBuffer()
		if 0 != j.dec.InputOffset() && 0 == len(buf) {
			/* We've already read a JSON object but its trailing
			newline hasn't shown up yet so we'll wait for it. */
			b := make([]byte, 1)
			n, err := io.ReadFull(j.Streamer, b)
			if nil != err { /* Probably closed. */
				j.rErr = err
				return n, j.rErr
			} else if '\n' != b[0] {
				j.rErr = ErrNoBufferedNewline
				return 0, j.rErr
			}
		} else if 0 != j.dec.InputOffset() &&
			0 != len(buf) && '\n' == buf[0] {
			/* Buffered after the last object and got a newline. */
			buf = buf[1:]
		}
		/* Read from the buffer and then the stream. */
		if 0 == len(buf) {
			j.r = j.Streamer
		} else {
			j.r = io.MultiReader(
				bytes.NewReader(buf),
				j.Streamer,
			)
		}
		j.dec = nil /* For just in case. */
	}

	/* Actual read itself is relatively easy. */
	return j.r.Read(b)
}

// Send encods and sends v to j.  Do not call concurrently with Write.
func (j Stream) Send(v any) error {
	return json.MarshalEncode(j.enc, v)
}
