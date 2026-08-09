package jsonstream

/*
 * jsonstream_test.go
 * Tests for jsonstream.go
 * By J. Stuart McMurray
 * Created 20260808
 * Last Modified 20260809
 */

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"path/filepath"
	"sync"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/bidirpipe"
	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
)

// newTestJSONStreamPair returns two connected jsonStreams.
func newTestJSONStreamPair(t *testing.T) (*Stream, *Stream) {
	l, r := bidirpipe.New()
	t.Cleanup(func() { l.Close(); r.Close() })
	return New(l), New(r)
}

// newTestJSONStream returns a jsonStream and the pipe connected to the other
// end.
func newTestJSONStream(t *testing.T) (bidirpipe.Pipe, *Stream) {
	pr, pj := bidirpipe.New()
	t.Cleanup(func() { pr.Close(); pj.Close() })
	return pr, New(pj)
}

// Can we make a new stream?
func TestJSONStream_Smoketest(t *testing.T) {
	t.Run("connected_pair", func(t *testing.T) { newTestJSONStreamPair(t) })
	t.Run("bidirpipe", func(t *testing.T) { newTestJSONStream(t) })
	t.Run("unix", func(t *testing.T) {
		/* Unix listener. */
		ua := &net.UnixAddr{
			Name: filepath.Join(t.TempDir(), "l"),
			Net:  "unix",
		}
		l, err := net.ListenUnix("unix", ua)
		if nil != err {
			t.Fatalf("Listen error: %v", err)
		}
		defer l.Close()
		/* Unix connection. */
		c, err := net.DialUnix("unix", nil, ua)
		if nil != err {
			t.Fatalf("Dial error: %v", err)
		}
		defer c.Close()
		/* Smoketest? */
		New(c)
	})
}

// Can we send and receive JSON?
func TestJSONStream_JSON(t *testing.T) {
	/* Message type we'll send via JSON. */
	type msgT struct {
		S string
		N int64
	}

	var (
		eg, ctx  = ctxerrgroup.WithContext(t.Context())
		nTry     = 100
		src, dst = newTestJSONStreamPair(t)

		sent     = make([]msgT, nTry)
		received = make([]msgT, nTry)
	)

	/* Send a bunch of messages. */
	eg.GoTag(ctx, "send", func(ctx context.Context) error {
		for i := range nTry {
			if nil != ctx.Err() {
				return nil
			}
			sent[i] = msgT{S: tlog.S("msg"), N: rand.Int64()}
			if err := src.Send(sent[i]); nil != err {
				return fmt.Errorf(
					"sending %d/%d: %w",
					i+1, nTry,
					err,
				)
			}
		}
		return nil
	})

	/* Receive them, hopefully. */
	eg.GoTag(ctx, "decode", func(ctx context.Context) error {
		for i := range nTry {
			if nil != ctx.Err() {
				return nil
			}
			if err := dst.DecodeNext(&received[i]); nil != err {
				return fmt.Errorf(
					"decoding %d/%d: %w",
					i+1, nTry,
					err,
				)
			}
		}
		return nil
	})

	/* Wait for it to all be done. */
	if err := eg.Wait(); nil != err {
		t.Fatalf("Error: %v", err)
	}

	/* Did we get them all? */
	for i := range nTry {
		if got, want := received[i], sent[i]; got != want {
			t.Errorf(
				"Message %d/%d incorrect\n"+
					" got: %#v\n"+
					"want: %v",
				i+1, nTry,
				got,
				want,
			)
		}
	}
}

// Can we read from the stream after decoding?
func TestJSONStreamRead(t *testing.T) {
	p, js := newTestJSONStream(t)

	/* Send a JSON value. */
	t.Run("json/pre-read", func(t *testing.T) {
		var (
			sent     = tlog.S("json")
			received string
			wg       sync.WaitGroup
		)
		wg.Go(func() {
			if err := json.MarshalWrite(p, sent); nil != err {
				t.Errorf("Error sending JSON string: %v", err)
			}
		})
		wg.Go(func() {
			if err := js.DecodeNext(&received); nil != err {
				t.Errorf("Error decoding JSON string: %v", err)
			}
		})
		wg.Wait()
	})
	if t.Failed() {
		t.FailNow()
	}

	/* Read non-JSON bytes. */
	t.Run("bytes", func(t *testing.T) {
		var (
			wrote    = tlog.S("bytes")
			received = make([]byte, len(wrote))
			wg       sync.WaitGroup
		)
		wg.Go(func() {
			if _, err := p.Write([]byte(wrote)); nil != err {
				t.Errorf("Error writing bytes: %v", err)
			}
		})
		wg.Go(func() {
			n, err := js.Read(received)
			received = received[:n]
			if nil != err {
				t.Errorf(
					"Error reading bytes\n"+
						" got: %q\n"+
						"want: %q\n"+
						" err: %v",
					received,
					wrote,
					err,
				)
			}
		})
		wg.Wait()
	})
	if t.Failed() {
		t.FailNow()
	}

	/* Trying to decode JSON again should fail. */
	t.Run("json/post-read", func(t *testing.T) {
		var v any
		if got, want := js.DecodeNext(&v),
			ErrDecodeAfterRead; !errors.Is(got, want) {
			t.Errorf(
				"Incorrect error decoding after read\n"+
					" got: %v\n"+
					"want: %v",
				got,
				want,
			)
		}
	})
}
