package crsadapter

/*
 * jsonstream_test.go
 * Tests for jsonstream.go
 * By J. Stuart McMurray
 * Created 20260808
 * Last Modified 20260823
 */

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/internal/bidirpipe"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/tlog"
)

// newTestJSONStreamPair returns two connected jsonStreams.
func newTestJSONStreamPair(t *testing.T) (*Stream, *Stream) {
	l, r := bidirpipe.New()
	t.Cleanup(func() { l.Close(); r.Close() })
	return NewStream(l), NewStream(r)
}

// newTestJSONStream returns a jsonStream and the pipe connected to the other
// end.
func newTestJSONStream(t *testing.T) (bidirpipe.Pipe, *Stream) {
	pr, pj := bidirpipe.New()
	t.Cleanup(func() { pr.Close(); pj.Close() })
	return pr, NewStream(pj)
}

// Can we make a new stream?
func TestJSONStream_Smoketest(t *testing.T) {
	t.Run("connected_pair", func(t *testing.T) { newTestJSONStreamPair(t) })
	t.Run("bidirpipe", func(t *testing.T) { newTestJSONStream(t) })
	t.Run("unix", func(t *testing.T) {
		c, _ := testUnixPair(t)
		NewStream(c)
	})
}

// Can we send and receive JSON and then bytes?
func TestJSONStream(t *testing.T) {
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
		t.Fatalf("JSON tx/rx error: %v", err)
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

	/* Can we send bytes? */
	var (
		plainBytes = tlog.S("plain-bytes")
	)
	eg, ctx = ctxerrgroup.WithContext(t.Context())
	eg.GoTag(ctx, "write", func(ctx context.Context) error {
		_, err := io.WriteString(src, plainBytes)
		return err
	})
	eg.GoTag(ctx, "read", func(ctx context.Context) error {
		buf := make([]byte, len(plainBytes))
		n, err := io.ReadFull(dst, buf)
		if got, want := string(buf[:n]), plainBytes; got != want {
			t.Errorf(
				"Read incorrect plain bytes\n"+
					" got: %q\n"+
					"want: %q",
				got,
				want,
			)
		}
		return err
	})
	if err := eg.Wait(); nil != err {
		t.Errorf("Plain bytes tx/rx error: %v", err)
	}
}

// Can we read from the stream after decoding?
func TestJSONStreamRead(t *testing.T) {
	synctest.Test(t, testJSONStreamRead)
}
func testJSONStreamRead(t *testing.T) {
	p, js := newTestJSONStream(t)

	/* Send/Receive a JSON value. */
	var (
		txJSON = tlog.S("json")
		rxJSON string
		wg     sync.WaitGroup
	)
	wg.Go(func() {
		b, err := json.Marshal(txJSON)
		if nil != err {
			t.Errorf("Error marshalling JSON string: %v", err)
		}
		b = append(b, '\n')
		if _, err := p.Write(b); nil != err {
			t.Errorf("Error writing JSON string: %v", err)
		}
	})
	wg.Go(func() {
		if err := js.DecodeNext(&rxJSON); nil != err {
			t.Errorf("Error decoding JSON string: %v", err)
		}
	})
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}

	/* Send/Receive non-JSON bytes. */
	var (
		txBytes = tlog.S("bytes")
		rxBytes = make([]byte, len(txBytes))
	)
	wg.Go(func() {
		if _, err := p.Write([]byte(txBytes)); nil != err {
			t.Errorf("Error writing non-JSON bytes: %v", err)
		}
	})
	wg.Go(func() {
		n, err := io.ReadFull(js, rxBytes)
		rxBytes = rxBytes[:n]
		if nil != err {
			t.Errorf(
				"Error reading JSON bytes\n"+
					" got: %q\n"+
					"want: %q\n"+
					" err: %v",
				rxBytes,
				txBytes,
				err,
			)
		}
	})
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}

	/* Trying to decode JSON again should fail. */
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
}

// testUnixPair returns a pair of connected Unix sockets.
func testUnixPair(t *testing.T) (c, s *net.UnixConn) {
	t.Helper()

	var (
		eg, ctx = ctxerrgroup.WithContext(t.Context())
		td      = t.TempDir()
	)

	/* Unix listener. */
	l, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: filepath.Join(td, "s"),
		Net:  "unix",
	})
	if nil != err {
		t.Fatalf("Listen error: %v", err)
	}
	defer l.Close()

	/* Connect as a client. */
	eg.GoTag(ctx, "client", func(ctx context.Context) error {
		var err error
		if c, err = (&net.Dialer{}).DialUnix(
			ctx,
			l.Addr().Network(),
			&net.UnixAddr{
				Name: filepath.Join(td, "c"),
				Net:  "unix",
			},
			l.Addr().(*net.UnixAddr),
		); nil != err {
			t.Errorf("Dial error: %v", err)
		}
		return err
	})
	/* Accept as the server. */
	eg.GoTag(ctx, "server", func(ctx context.Context) error {
		var err error
		if s, err = l.AcceptUnix(); nil != err {
			t.Errorf("Accept error: %v", err)
		}
		return err
	})
	if err := eg.Wait(); nil != err {
		t.Errorf("Errar making pair of sockets: %v", err)
	}
	/* Don't keep going if something went wrong. */
	if t.Failed() {
		if nil != c {
			c.Close()
		}
		if nil != s {
			s.Close()
		}
		t.FailNow()
	}

	/* Don't leak file descriptors. */
	t.Cleanup(func() { c.Close(); s.Close() })

	return c, s
}

// Can we handle a read that happens after a JSON object is written but
// before it's trailing newline is written?
func TestStreamRead_ReadBeforeJSONNewline(t *testing.T) {
	t.Run("newline_present", func(t *testing.T) {
		msg := tlog.S("msg")
		synctest.Test(t, func(t *testing.T) {
			testStreamReadReadBeforeJSONNewline(
				t,
				"\n"+msg,
				msg,
				nil,
			)
		})
	})
	t.Run("newline_absent", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			testStreamReadReadBeforeJSONNewline(
				t,
				tlog.S("msg"),
				"",
				ErrNoBufferedNewline,
			)
		})
	})
}
func testStreamReadReadBeforeJSONNewline(
	t *testing.T,
	msg string,
	wantMsg string,
	wantErr error,
) {
	var (
		p, js = newTestJSONStream(t)
		wg    sync.WaitGroup
	)

	/* Write a JSON object, but no newline. */
	wg.Go(func() {
		if _, err := io.WriteString(p, "{}"); nil != err {
			t.Errorf("Error writing empty object: %v", err)
		}
	})
	wg.Go(func() {
		var v any
		if err := js.DecodeNext(&v); nil != err {
			t.Errorf("Error decoding empty object: %v", err)
		}
	})
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}

	/* Start a Read, let it settle so it's blocking on read. */
	wg.Go(func() {
		b := make([]byte, len(msg))
		n, err := js.Read(b)
		if got, want := err, wantErr; !errors.Is(got, want) {
			t.Errorf(
				"Read returned incorrect error\n"+
					" got: %v\n"+
					"want: %v",
				got,
				want,
			)
		}
		if got, want := string(b[:n]), wantMsg; got != want {
			t.Errorf(
				"Read incorrect\n got: %q\nwant: %q",
				got,
				want,
			)
		}
		if nil != err {
			js.Close()
		}
	})
	synctest.Wait()

	/* Should be blocking, send some data. */
	wg.Go(func() {
		_, err := io.WriteString(p, msg)
		if (nil == wantErr && nil != err) ||
			(nil != wantErr && !errors.Is(err, io.ErrClosedPipe)) {
			t.Errorf("Error writing non-JSON: %v", err)
		}
	})
	wg.Wait()
}

// Do we get an error when we're switching to reading bytes but the stream is
// closed before the newline after the final object, and do we get the error
// on subsequent reads?
func TestStreamRead_ErrorbeforeJSONNewline(t *testing.T) {
	var (
		p, js = newTestJSONStream(t)
		nTry  = 10
		wg    sync.WaitGroup
	)

	/* Write a JSON object, but no newline. */
	wg.Go(func() {
		if _, err := io.WriteString(p, "{}"); nil != err {
			t.Errorf("Error writing empty object: %v", err)
		}
	})
	wg.Go(func() {
		var v any
		if err := js.DecodeNext(&v); nil != err {
			t.Errorf("Error decoding empty object: %v", err)
		}
	})
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}

	/* Close the stream, so reading the final newline will fail. */
	if err := p.Close(); nil != err {
		t.Fatalf("Error closing write side of pipe: %v", err)
	}

	/* Read should fail as many times as we try. */
	b := make([]byte, 1)
	for n := range nTry {
		_, err := js.Read(b)
		if got, want := err, io.EOF; !errors.Is(got, want) {
			t.Errorf(
				"Post-close read %d/%d returned "+
					"incorrect error\n"+
					" got: %v\n"+
					"want: %v",
				n+1, nTry,
				got,
				want,
			)
		}
	}
}

// Can we read buffered bytes after JSON as well as non-buffered bytes?
// This is a bit fragile, perhaps.
func TestStreamRead_BufferedData(t *testing.T) {
	var (
		p, js = newTestJSONStream(t)
		bMsg  = tlog.S("buffered-msg")
		nMsg  = tlog.S("streamed-msg")
		wg    sync.WaitGroup
	)

	/* Write a JSON object and some raw data and hope it's buffered. */
	wg.Go(func() {
		if _, err := fmt.Fprintf(p, "{}\n%s", bMsg); nil != err {
			t.Errorf("Error writing empty object: %v", err)
		}
	})
	wg.Go(func() {
		var v any
		if err := js.DecodeNext(&v); nil != err {
			t.Errorf("Error decoding empty object: %v", err)
		}
	})
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}

	/* Make sure it's buffered.  Hack. */
	if 0 == len(js.dec.UnreadBuffer()) {
		t.Fatalf("BROKEN TEST: No data buffered :(")
	}

	/* Read from the buffer. */
	b := make([]byte, len(bMsg))
	n, err := js.Read(b)
	if nil != err {
		t.Fatalf("Error reading buffered message: %v", err)
	}
	if got, want := string(b[:n]), bMsg; got != want {
		t.Fatalf(
			"Read incorrect buffered message\n got: %q\nwant: %q",
			got,
			want,
		)
	}

	/* And a non-buffered message. */
	wg.Go(func() {
		b := make([]byte, len(nMsg))
		n, err := js.Read(b)
		if nil != err {
			t.Errorf("Error reading non-buffered message: %v", err)
		}
		if got, want := string(b[:n]), nMsg; got != want {
			t.Errorf(
				"Non-buffered read incorrect\n"+
					" got: %q\n"+
					"want: %q",
				got,
				want,
			)
		}
	})
	wg.Go(func() {
		_, err := io.WriteString(p, nMsg)
		if nil != err {
			t.Errorf("Error writing non-buffered message: %v", err)
		}
	})
	wg.Wait()
}
