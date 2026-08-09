// Package jsonstream - Bidirectional stream of JSON objects.
package jsonstream

/*
 * jsonstream.go
 * Bidirectional stream of JSON objects
 * By J. Stuart McMurray
 * Created 20260808
 * Last Modified 20260809
 */

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"
)

// ErrDecodeAfterRead is returned by [Stream.DecodeNext] if called after a
// call to [Stream.Read].
var ErrDecodeAfterRead = errors.New(
	"attempt to decode JSON value after reading raw bytes",
)

// Streamer is the interface Stream wraps.
type Streamer interface {
	io.ReadWriteCloser
	CloseRead() error
	CloseWrite() error
}

// Stream is a bidirectional stream of JSON objects.  Its methods are
// generally not safe for concurrent use, with the exception that calls to
// Write/Send may be called concurrently with calls to Read/DecodeNext.
type Stream struct {
	Streamer

	/* Read side. */
	r     io.Reader                /* UnixConn plus unread buffer. */
	decCh chan (*jsontext.Decoder) /* Mutex and synctest don't get along. */

	/* Write side. */
	enc *jsontext.Encoder
}

// New returns a new jsonStream wrapping c.
func New(b Streamer) *Stream {
	decCh := make(chan *jsontext.Decoder, 1)
	decCh <- jsontext.NewDecoder(b)
	return &Stream{
		Streamer: b,
		decCh:    decCh,
		enc: jsontext.NewEncoder(
			b,
			jsontext.ReorderRawObjects(true),
		),
	}
}

// DecodeNext attempts to decode the next JSON message into v.  Do not call
// DecodeNext after the first call to Read.
func (j Stream) DecodeNext(v any) error {
	/* Get the decoder and make sure to put it back. */
	dec := <-j.decCh
	defer func() { j.decCh <- dec }()
	/* Make sure nobody called Read. */
	if nil == dec {
		return ErrDecodeAfterRead
	}
	/* Decode. */
	return json.UnmarshalDecode(dec, v)
}

// Read reads plain bytes from j, bypassing JSON decoding.  Do not call
// concurrently with DecodeNext or another call to Read.
// As Stream is indended for newline-separated messages, if a newline was sent
// after the last JSON message, it will not be returned.
func (j *Stream) Read(b []byte) (int, error) {
	/* Get the decoder. */
	dec := <-j.decCh
	defer func() { j.decCh <- dec }()
	/* If this is our first read, turn off JSON and make the reamining
	unbuffered bytes available. */
	if nil != dec {
		/* Buffered data, but maybe remove the initial newline. */
		buf := dec.UnreadBuffer()
		if 0 != len(buf) && '\n' == buf[0] {
			buf = buf[1:]
		}
		/* Read from the buffer and then the stream. */
		j.r = io.MultiReader(
			bytes.NewReader(buf),
			j.Streamer,
		)
		dec = nil
	}

	return j.r.Read(b)
}

// Send encods and sends v to j.  Do not call concurrently with Write.
func (j Stream) Send(v any) error {
	return json.MarshalEncode(j.enc, v)
}
