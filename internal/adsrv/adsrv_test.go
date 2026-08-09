package adsrv

/*
 * adsrv_test.go
 * Tests for adsrv.go
 * By J. Stuart McMurray
 * Created 20260803
 * Last Modified 20260809
 */

import (
	"cmp"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/internal/bidirpipe"
	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

const (
	// chBufLen is the size of the i/o broker channels. */
	chBufLen = 10240

	// firstAdapterName is the name we expect to be given to the first
	// adapter connection, i.e. the one returned from newTestServer.
	firstAdapterName = adapterConnPrefix + "1"
)

// testServerConfig is passed to newTestServer.
type testServerConfig struct {
	ctx context.Context
}

// newTestServer returns a new test server and connected client.
// Meant to be called like
//
//	m, tb, ich, och, c, s := newTestServer(t, nil)
func newTestServer(t *testing.T, conf *testServerConfig) (
	tlog.Msg, /* m */
	*tlog.Buffer, /* tb */
	chan<- string, /* ich */
	<-chan opshell.CLine, /* och */
	bidirpipe.Pipe, /* c */
	*Server, /* s */
) {
	/* Do actually need a config. */
	if nil == conf {
		conf = new(testServerConfig)
	}

	var (
		ctx    = cmp.Or(conf.ctx, t.Context())
		ich    = make(chan string, chBufLen)
		lPath  = filepath.Join(t.TempDir(), "l")
		och    = make(chan opshell.CLine, chBufLen)
		pch    chan bidirpipe.Pipe
		tb, sl = tlog.NewBuffer()

		iob = iobroker.New(och)
	)

	/* Make sure we have a pipe channel in the context. */
	var ok bool
	if pch, ok = ctx.Value(
		testClientsChannelContextKey{},
	).(chan bidirpipe.Pipe); !ok {
		pch = make(chan bidirpipe.Pipe)
		ctx = context.WithValue(
			ctx,
			testClientsChannelContextKey{},
			pch,
		)
	}

	/* Create and start a server. */
	eg, ctx := ctxerrgroup.WithContext(ctx)
	s, err := New(sl, lPath, iob)
	if nil != err {
		t.Fatalf("Error creating Server: %v", err)
	}
	tb.Expect(t.Context(), t,
		tlog.M.
			With(LKSocketPath, s.Addr().String()).
			Info(LMAdapterListenerStarted),
	)
	eg.GoTag(ctx, "server", func(ctx context.Context) error {
		return s.Run(ctx)
	})

	/* Start the I/O Broker. */
	eg.GoTag(ctx, "iobroker", func(ctx context.Context) error {
		return iob.Run(ctx, ich, false)
	})

	/* Check for a happy ending. */
	t.Cleanup(func() {
		if err := eg.Wait(); nil != err {
			t.Errorf("Error: %v", err)
		}
		/* Shouldn't have any more logs. */
		tb.CloseExpectEmpty(ctx, t)
		/* Shouldn't have any shell output. */
		close(och)
		opshell.ExpectNoShellMessages(t, och)
	})

	/* Hook up a client. */
	pc, ps := bidirpipe.New()
	pch <- ps
	m := tlog.M.With(LKConnectionName, firstAdapterName)
	tb.Expect(t.Context(), t,
		m.
			Debug(LMNewAdapterConn),
	)

	return m, tb, ich, och, pc, s
}

// Can we start and shut down a server?
func TestServer_Smoketest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) { newTestServer(t, nil) })
}

// Do we get the right sort of error when we can't remove what we expect
// is a stale unix socket?
func TestNew_RemovePathError(t *testing.T) {
	/* Make a directory with a file in it, but not enough perms to remove
	the file. */
	td := t.TempDir()
	f, err := os.Create(filepath.Join(td, "f"))
	if nil != err {
		t.Fatalf("Error creating file: %v", err)
	}
	defer f.Close()
	if err := os.Chmod(td, 0000); nil != err {
		t.Fatalf("Error setting no permissions: %v", err)
	}
	defer os.Chmod(td, 0770)

	/* Shouldn't be able to start a server listening on that path. */
	_, err = New(nil, td, nil)
	if got, want := err, syscall.EACCES; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error trying to remove "+
				"non-empty directory\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
}

// Do we get the right sort of error when we can't remove what we expect
// is a stale unix socket?
func TestNew_ListenError(t *testing.T) {
	_, err := New(
		nil,
		filepath.Join(t.TempDir(), "does", "not", "exist"),
		nil,
	)
	if got, want := err, syscall.ENOENT; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error trying to listen on non-existent "+
				"path\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
}

// Do we properly report errors when accept fails?
func TestServerRun_AcceptError(t *testing.T) {
	/* Start the server with listener closed. */
	tb, sl := tlog.NewBuffer()
	s, err := New(sl, filepath.Join(t.TempDir(), "l"), nil)
	if nil != err {
		t.Fatalf("Error making Server: %v", err)
	}
	if err := s.l.Close(); nil != err {
		t.Fatalf("Error closing listener: %v", err)
	}

	/* Should get an error. */
	if got, want := s.Run(t.Context()), net.ErrClosed; nil != err {
		t.Errorf(
			"Incorrect error after accept fail\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}

	/* Should only get the log that we started listening. */
	tb.WithExpectEmpty().Expect(t.Context(), t,
		tlog.M.
			With(LKSocketPath, s.Addr().String()).
			Info(LMAdapterListenerStarted),
	)
}

// Can we get a client's unix socket path?
func TestServerRun_AcceptConnectionWithName(t *testing.T) {
	/* Test server. */
	var (
		ctx, cancel = context.WithCancel(t.Context())
		ech         = make(chan error, 1)
		tb, sl      = tlog.NewBuffer()
	)
	s, err := New(sl, filepath.Join(t.TempDir(), "s"), nil)
	if nil != err {
		t.Fatalf("Error making Server: %v", err)
	}
	defer s.l.Close()

	/* Start it running. */
	go func() { ech <- s.Run(ctx) }()

	/* Unix banner-grabbing. */
	ta := filepath.Join(t.TempDir(), "c")
	c, err := net.DialUnix("unix", &net.UnixAddr{
		Name: ta,
		Net:  "unix",
	}, s.l.Addr().(*net.UnixAddr))
	if nil != err {
		t.Fatalf("Error connecting to server: %v", err)
	}
	defer c.Close()

	/* Should have logged.  We check before closing the connection to avoid
	closing the connection before the server gets the name. */
	m := tlog.M.
		With(LKConnectionName, c.LocalAddr().String())
	tb.WithExpectEmpty().Expect(t.Context(), t,
		tlog.M.
			With(LKSocketPath, s.Addr().String()).
			Info(LMAdapterListenerStarted),
		m.
			Debug(LMNewAdapterConn),
	)

	/* Close the connection, should get an error. */
	if err := c.Close(); nil != err {
		t.Errorf("Error closing client connection: %v", err)

	}
	tb.WithExpectEmpty().Expect(t.Context(), t,
		m.
			With(LKError, io.EOF).
			Warn(LMConnRequestError),
	)

	/* Done with the server. */
	cancel()
	if err := <-ech; nil != err {
		t.Errorf("Server exited with error: %v", err)
	}
	tb.CloseExpectEmpty(ctx, t)
}

// Do we report our own address properly?
func TestServerAddr(t *testing.T) {
	_, _, _, _, _, s := newTestServer(t, nil)
	if got, want := s.Addr().String(), s.l.Addr().String(); got != want {
		t.Errorf(
			"Incorrect listen address\n got: %v\nwant: %v",
			got,
			want,
		)
	}
}
