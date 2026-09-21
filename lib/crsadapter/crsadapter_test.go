package crsadapter

/*
 * crsadapter_test.go
 * Tests for crsadapter.go
 * By J. Stuart McMurray
 * Created 20260809
 * Last Modified 20260906
 */

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/bidirpipe"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/tlog"
)

// Can we dial curlrevshell ok?
func TestDial(t *testing.T) {
	var (
		sch  = make(chan *Stream, 1)
		ech  = make(chan error, 1)
		ct   = ConnTypeShellStream
		args = ConnTypeShellStreamArgs{
			Direction: ShellStreamDirectionInOut,
			ID:        tlog.S("id"),
			Tag:       tlog.S("tag"),
			LogInfo:   tlog.S("log-info"),
		}
	)

	/* Listen for a connection */
	l, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: filepath.Join(t.TempDir(), "s"),
		Net:  "unix",
	})
	if nil != err {
		t.Fatalf("Listen error: %v", err)
	}
	defer l.Close()

	/* Dial. */
	go func() {
		defer close(sch)
		defer close(ech)
		st, err := Dial(t.Context(), l.Addr().String(), ct, args)
		sch <- st
		ech <- err
	}()

	/* Playing adsrv now, accept the client. */
	u, err := l.AcceptUnix()
	if nil != err {
		t.Fatalf("Error accepting client: %v", err)
	}
	s := NewStream(u)

	/* Should get the request as sent. */
	cReq := ConnRequest{Args: jsontext.Value{}}
	if err := s.DecodeNext(&cReq); nil != err {
		t.Fatalf("Error decoding ConnRequest: %v", err)
	}
	if got, want := cReq.ConnType, ct; got != want {
		t.Errorf(
			"Got incorrect ConnType\n got: %s\nwant: %s",
			got,
			want,
		)
	}
	var ssa ConnTypeShellStreamArgs
	if err := json.Unmarshal(cReq.Args.(jsontext.Value), &ssa); nil != err {
		t.Fatalf("Error decoding args: %v", err)
	}
	if got, want := ssa, args; got != want {
		t.Errorf("Got incorrect Args\n got: %#v\nwant: %#v", got, want)
	}

	/* Send a happy response. */
	if err := s.Send(ConnResponse{}); nil != err {
		t.Fatalf("Error sending response: %v", err)
	}

	/* Should have got a stream and no error. */
	var c *Stream
	if err := <-ech; nil != err {
		t.Fatalf("Dial returned error: %v", err)
	}
	if c = <-sch; nil == c {
		t.Fatalf("Dial returned nil Stream")
	}

	/* Can we communicate between the two now? */
	var (
		eg, ctx = ctxerrgroup.WithContext(t.Context())
		cMsg    = tlog.S("client-message")
		sMsg    = tlog.S("server-message")
	)
	eg.GoTag(ctx, "client", func(ctx context.Context) error {
		/* Client sends first. */
		if err := c.Send(cMsg); nil != err {
			return fmt.Errorf("sending message: %v", err)
		}
		/* Server sends second. */
		var got string
		if err := c.DecodeNext(&got); nil != err {
			return fmt.Errorf("receiving message: %v", err)
		}
		if want := sMsg; got != want {
			t.Errorf(
				"Client got incorrect message\n"+
					" got: %q\n"+
					"want: %q\n",
				got,
				want,
			)
		}
		return nil
	})
	eg.GoTag(ctx, "server", func(ctx context.Context) error {
		/* Client sends first. */
		var got string
		if err := s.DecodeNext(&got); nil != err {
			return fmt.Errorf("receiving message: %v", err)
		}
		if want := cMsg; got != want {
			t.Errorf(
				"Server got incorrect message\n"+
					" got: %q\n"+
					"want: %q\n",
				got,
				want,
			)
		}
		/* Server sends second. */
		if err := s.Send(sMsg); nil != err {
			return fmt.Errorf("sending message: %v", err)
		}
		return nil
	})
	if err := eg.Wait(); nil != err {
		t.Errorf("Communications error: %v", err)
	}
}

// Do we whine properly when we can't connect to a socket?
func TestDial_DialError(t *testing.T) {
	c, err := Dial(t.Context(), t.TempDir(), ConnType(tlog.S("test")), nil)
	if got, want := err, syscall.ENOTSOCK; !errors.Is(got, want) {
		t.Errorf(
			"Unexpected error connecting to directory\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
	if nil != c {
		t.Errorf("Got non-nil client connecting to directory")
	}
}

// Do we whine properly when the handshake starts with a cancelled context?
func TestDial_ContextCancelledBeforeHandshake(t *testing.T) {
	var (
		p, _        = bidirpipe.New()
		ctx, cancel = context.WithCancel(t.Context())
	)
	cancel()
	s, err := handshakeContext(ctx, p, ConnType(tlog.S("test")), nil)
	if got, want := err, context.Canceled; !errors.Is(got, want) {
		t.Errorf("Unexpected error\n got: %v\nwant: %v", got, want)
	}
	if nil != s {
		t.Errorf("Got non-nil client")
	}
}

// Do we whine properly when handshaking with a closed pipe?
func TestDial_ClosedPipe(t *testing.T) {
	p, _ := bidirpipe.New()
	p.Close()
	s, err := handshakeContext(
		t.Context(),
		p,
		ConnType(tlog.S("test")),
		nil,
	)
	if got, want := err, io.ErrClosedPipe; !errors.Is(got, want) {
		t.Errorf("Unexpected error\n got: %v\nwant: %v", got, want)
	}
	if nil != s {
		t.Errorf("Got non-nil client")
	}
}

// Do we whine properly when the response doesn't happen?
func TestDial_NoResponse(t *testing.T) {
	var (
		p, s = newTestJSONStream(t)
		wg   sync.WaitGroup
	)

	/* Read the request then close the connection.  Jerk. */
	wg.Go(func() {
		var v any
		if err := s.DecodeNext(&v); nil != err {
			t.Fatalf("Error decoding request")
		}
		if err := s.Close(); nil != err {
			t.Fatalf("Error closing stream: %v", err)
		}
	})

	/* Handsake, but we'll never hear back. */
	s, err := handshakeContext(
		t.Context(),
		p,
		ConnType(tlog.S("test")),
		nil,
	)
	if got, want := err, io.EOF; !errors.Is(got, want) {
		t.Errorf("Unexpected error\n got: %v\nwant: %v", got, want)
	}
	if nil != s {
		t.Errorf("Got non-nil client")
	}
}

// Do we whine properly when the server rejects us?
func TestDial_Rejected(t *testing.T) {
	var (
		p, s = newTestJSONStream(t)
		estr = tlog.S("error")
		wg   sync.WaitGroup
	)

	/* Read the request and reject it without even looking at it.
	Tinder, for adapters. */
	wg.Go(func() {
		var v any
		if err := s.DecodeNext(&v); nil != err {
			t.Fatalf("Error decoding request")
		}
		if err := s.Send(ConnResponse{Error: estr}); nil != err {
			t.Fatalf("Error sending response: %v", err)
		}
	})

	/* Handsake, but we'll never hear back. */
	s, err := handshakeContext(
		t.Context(),
		p,
		ConnType(tlog.S("test")),
		nil,
	)
	if cre, ok := errors.AsType[ConnResponseError](err); ok {
		if got, want := cre, (ConnResponseError{
			ConnResponse{estr},
		}); !errors.Is(got, want) {
			t.Errorf(
				"Incorrect ConnResponseError\n"+
					" got: %#v\n"+
					"want: %#v",
				got,
				want,
			)
		}
	} else {
		t.Errorf("Unexpected error\ntype: %T\nerr: %v", err, err)
	}
	if nil != s {
		t.Errorf("Got non-nil client")
	}
}

// Do RandomIDs look like random IDs?
// In theory there could be a collision, but this is unlikely.
func TestRandomID(t *testing.T) {
	var (
		nTry = 10240
		seen = make(map[string]struct{})
	)
	for i := range nTry {
		id := RandomID()
		if _, ok := seen[id]; ok {
			t.Fatalf("Duplicate id found after %d tries", i)
		}
		seen[id] = struct{}{}
	}
}
