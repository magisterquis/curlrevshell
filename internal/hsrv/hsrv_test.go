package hsrv

/*
 * hsrv_test.go
 * Tests for hserv.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20260815
 */

import (
	"cmp"
	"context"
	"crypto/tls"
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
	"testing"
	"time"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/lib/crstemplate"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
	"github.com/magisterquis/curlrevshell/lib/sstls"
	"github.com/magisterquis/curlrevshell/lib/tlog"
)

// bufLen is used for the buffer length of our iobroker channels.
var bufLen = 512

// testServerConfig configures newTestServer.
type testServerConfig struct {
	debug      bool   /* Log/Print debug messages. */
	makeFDir   bool   /* Make directory from which to serve files. */
	noLogHI    bool   /* Don't log HTTP info. */
	oneShell   bool   /* Exit after one shell. */
	wantErr    error  /* Error we expect. */
	listenAddr string /* Listen address. */
	tmplf      string /* Template file. */
	registerHM bool   /* Register help messages. */
	urlPaths   *crstemplate.URLPaths
}

// newTestServer returns a new server, suitable for testing, as well as a
// client configured for the server's TLS.
// A nil config is equivalent to an empty config.
//
// Run like
//
//	tb, ich, och, done, c, s := newTestServer(t.Context(), t, nil)
func newTestServer(ctx context.Context, t *testing.T, conf *testServerConfig) (
	*tlog.Buffer, /* tb */
	chan<- string, /* ich */
	<-chan opshell.CLine, /* och */
	<-chan struct{}, /* done. */
	*http.Client, /* c */
	*Server, /* s */
) {
	t.Helper()
	var (
		addr    = "127.0.0.1"
		cbAddrs = []string{"kittens.com:8888", "moose.com"}
		done    = make(chan struct{})
		ich     = make(chan string, bufLen)
		och     = make(chan opshell.CLine, bufLen)
		tb, sl  = tlog.NewBuffer()
		td      string

		iob = iobroker.New(och)
	)

	/* Need a config. */
	if nil == conf {
		conf = new(testServerConfig)
	}
	/* Work out if we're serving files. */
	if conf.makeFDir {
		td = t.TempDir()
	}
	/* Work out our parameters. */
	params := crstemplate.Params{StaticFilesDir: td}
	if nil != conf.urlPaths {
		params.URLPaths = *conf.urlPaths
	}
	/* Set up a new server. */
	s, err := New(
		sl,
		cmp.Or(conf.listenAddr, addr),
		conf.tmplf,
		iob,
		"", /* certFile */
		cbAddrs,
		false, /* printIPv6 */
		conf.debug,
		params,
	)
	if nil != err {
		t.Fatalf("Error creating server: %s", err)
	}

	/* Register help functions, maybe. */
	if conf.registerHM {
		s.RegisterOneLiners()
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

	/* Work out the context, including optional test things. */
	ctx, cancel := context.WithCancel(ctx)
	if conf.noLogHI {
		ctx = context.WithValue(ctx, testNoLogHIKey{}, true)
	}

	/* Start the server going. */
	eg, ctx := ctxerrgroup.WithContext(ctx)
	eg.GoTag(ctx, "Server", func(ctx context.Context) error {
		defer close(done)
		return s.Do(ctx)
	})
	eg.GoTag(ctx, "i/o broker", func(ctx context.Context) error {
		return iob.Run(ctx, ich, conf.oneShell)
	})
	t.Cleanup(func() {
		t.Helper()
		cancel()
		/* Everything should end happily. */
		if got, want := eg.Wait(), conf.wantErr; !errors.Is(got, want) {
			t.Errorf(
				"Unexpected error after server shutdown\n"+
					"got: %v\n"+
					"want: %v",
				got,
				want,
			)
		}
		/* Make sure we have no leftover opshell or log lines. */
		close(och)
		opshell.ExpectNoShellMessages(t, och)
		tb.CloseExpectEmpty(context.Background(), t)
	})

	/* Server started? */
	tb.Expect(t.Context(), t,
		tlog.M.
			With(LKFingerprint, s.l.Fingerprint).
			With(LKListenAddr, s.l.Addr().String()).
			Info(LMListenerStarted),
	)

	/* And a client pre-configured for the server's TLS fingerprint. */
	fpv, err := sstls.TLSFingerprintVerifier(s.l.Fingerprint)
	if nil != err {
		t.Fatalf("Error setting up client TLS verification: %v", err)
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
		VerifyConnection:   fpv,
	}
	c := &http.Client{Transport: tr}

	/* Don't keep going if we have an error. */
	if t.Failed() {
		t.FailNow()
	}

	return tb, ich, och, done, c, s
}

func TestServer_Smoketest(t *testing.T) {
	newTestServer(t.Context(), t, nil)
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

// Make sure the returned client can connect to the server.
func TestServer_Client(t *testing.T) {
	_, _, _, _, c, s := newTestServer(t.Context(), t, nil)
	res, err := c.Get("https://" + s.l.Addr().String())
	if nil != err {
		t.Fatalf("GET error: %v", err)
	}
	defer res.Body.Close()
	if http.StatusNotFound != res.StatusCode {
		t.Errorf("Unexpected HTTP status: %s", res.Status)
	}
}

// Make sure the server returns after a single shell, if oneShell is set.
func TestServer_OneShell(t *testing.T) {
	var (
		eg, ctx                = ctxerrgroup.WithContext(t.Context())
		id                     = tlog.S("id")
		msg                    = tlog.S("output")
		pr, pw                 = io.Pipe()
		tb, _, och, done, c, s = newTestServer(
			t.Context(),
			t,
			&testServerConfig{
				noLogHI:  true,
				oneShell: true,
				wantErr:  iobroker.ErrOneShell,
			},
		)
	)
	defer pr.Close()
	defer pw.Close()

	/* Connection to the shell. */
	eg.GoTag(ctx, "connection", func(ctx context.Context) error {
		defer pr.Close()
		/* Connect to the server. */
		res, err := c.Post(
			"https://"+s.l.Addr().String()+"/o/"+id,
			"",
			pr,
		)
		if nil != err {
			return fmt.Errorf("POST request: %w", err)
		}
		defer res.Body.Close()
		if http.StatusOK != res.StatusCode {
			return fmt.Errorf("non-OK status: %s", res.Status)
		}
		/* Work out what our logs should have in them. */
		return nil
	})

	/* Send it some output. */
	eg.GoTag(ctx, "output", func(ctx context.Context) error {
		defer pw.Close()
		_, err := pw.Write([]byte(msg))
		return err
	})

	/* Did it work? */
	if err := eg.Wait(); nil != err {
		t.Errorf("Error: %v", err)
	}

	/* Shell messages look ok? */
	opshell.ExpectShellMessages(t, och,
		opshell.CLine{
			Color: connectedColor,
			Line: fmt.Sprintf(
				"[127.0.0.1] Output connected: ID %s",
				id,
			),
		},
		opshell.CLine{
			Line:  msg,
			Plain: true,
		},
		opshell.CLine{
			Color: errorColor,
			Line:  "[127.0.0.1] Output connection closed",
		},
	)
	m := tlog.M.
		With(iobroker.LKDirection, iobroker.LVOutput).
		With(iobroker.LKID, id)
	tb.Expect(t.Context(), t,
		m.
			Info(iobroker.LMNewConnection),
		m.
			With(iobroker.LKData, msg).
			Info(iobroker.LMShellIO),
		m.
			Info(iobroker.LMConnectionClosed),
	)

	/* Server actually done? */
	<-done
}

// Can we switch on and off debug messages?
func TestServer_Debug(t *testing.T) {
	/* try spawns a server, banner-grabs it, and checks for expected
	messages. */
	try := func(t *testing.T, printDebug bool) {
		_, _, och, _, _, s := newTestServer(
			t.Context(),
			t,
			&testServerConfig{debug: printDebug},
		)

		/* Banner-grab it. */
		c, err := net.DialTimeout(
			"tcp",
			s.l.Addr().String(),
			time.Second,
		)
		if nil != err {
			t.Fatalf("Error connecting to server: %v", err)
		}
		if err := c.Close(); nil != err {
			t.Fatalf("Error closing connection to server: %v", err)
		}

		/* Did we get a debug message if we should have? */
		if printDebug {
			opshell.ExpectShellMessages(t, och, opshell.CLine{
				Color: opshell.ColorMagenta,
				Line: fmt.Sprintf(
					"Server error: http: "+
						"TLS handshake error from %s: "+
						"%s\n",
					c.LocalAddr(),
					io.EOF,
				),
			})
		}

	}

	/* Try with debug messages. */
	t.Run("with_debug", func(t *testing.T) { try(t, true) })

	/* And try without debug messages. */
	t.Run("no_debug", func(t *testing.T) { try(t, false) })
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

// Make sure Server.allCallbackAddresses adds port number to everything.
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
	gotAddrs, err := (&Server{l: l}).allCallbackAddresses(have)
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

// Do we handle failures to listen nicely?
func TestNew_ListenError(t *testing.T) {
	var (
		addr    = tlog.S("({impossibleAddr!})")
		lb, sl  = tlog.NewBuffer()
		wantErr = "no such host"
	)
	_, err := New(
		sl,
		addr,
		"",
		nil,
		"",
		nil,
		false,
		true, /* Shouldn't matter. */
		crstemplate.Params{},
	)

	/* Error look ok? */
	de, ok := errors.AsType[*net.DNSError](err)
	if !ok {
		t.Fatalf(
			"Error has incorrect type\n"+
				" err: %v\n"+
				" got: %T\n"+
				"want: %T",
			err,
			err,
			de,
		)
	}
	if got, want := de.Err, wantErr; got != want {
		t.Errorf(
			"Incorrect error string\n got: %s\nwant: %s",
			got,
			want,
		)
	}
	if !de.IsNotFound {
		t.Errorf("Error not an NXDOMAIN: %v", err)
	}

	/* Should get no logs. */
	lb.CloseExpectEmpty(t.Context(), t)
}

// Do we handle wonky callback addresses?
func TestNew_AllCallbackAddressesError(t *testing.T) {
	var (
		_, sl = tlog.NewBuffer()
		och   = make(chan opshell.CLine, bufLen)
		iob   = iobroker.New(och)
	)

	_, err := New(
		sl,
		"",
		"",
		iob,
		"",
		[]string{testErrorCBAddr},
		false,
		true, /* Shouldn't matter. */
		crstemplate.Params{},
	)
	if got, want := err, errAllCallbackAddresses; !errors.Is(got, want) {
		t.Errorf("Incorrect error\n got: %v\nwant: %v", got, want)
	}
}

// Do we handle no callback addresses?
func TestNew_AllCallbackAddressesEmpty(t *testing.T) {
	var (
		_, sl = tlog.NewBuffer()
		och   = make(chan opshell.CLine, bufLen)
		iob   = iobroker.New(och)
	)
	_, err := New(
		sl,
		"",
		"",
		iob,
		"",
		[]string{testEmptyCBAddr},
		false,
		true, /* Shouldn't matter. */
		crstemplate.Params{},
	)
	if got, want := err, errNoCallbackAddresses; !errors.Is(got, want) {
		t.Errorf("Incorrect error\n got: %v\nwant: %v", got, want)
	}
}

// Do we log different URL Paths properly?
func TestNew_NonDefaultURLPaths(t *testing.T) {
	var (
		in     = tlog.S("in")
		out    = tlog.S("out")
		inout  = tlog.S("inout")
		script = tlog.S("script")
	)

	/* Non-default paths. */
	tb, _, _, _, _, _ := newTestServer(t.Context(), t, &testServerConfig{
		urlPaths: &crstemplate.URLPaths{
			In:     in,
			InOut:  inout,
			Out:    out,
			Script: script,
		},
	})

	/* Log properly? */
	tb.Expect(t.Context(), t,
		tlog.M.
			With("In", in).
			With("Out", out).
			With("InOut", inout).
			With("Script", script).
			Info(LMURLPaths),
	)
}

// Do we get an error if the HTTP server returns an error?
func TestServerDo_ServeError(t *testing.T) {
	_, _, _, done, c, s := newTestServer(t.Context(), t, &testServerConfig{
		wantErr: net.ErrClosed,
	})

	/* Request, to make sure we're live. */
	res, err := c.Get("https://" + s.l.Addr().String())
	if nil != err {
		t.Errorf("Get returned error: %v", err)
	}
	res.Body.Close()

	/* That's a nice listener you have there; would be a shame if something
	were to happen to it. */
	if err := s.l.Close(); nil != err {
		t.Errorf("Error closing listener: %v", err)
	}
	<-done
}

// Do we report our own address properly?
func TestServerAddr(t *testing.T) {
	_, _, _, _, _, s := newTestServer(t.Context(), t, nil)
	if got, want := s.Addr().String(), s.l.Addr().String(); got != want {
		t.Errorf(
			"Incorrect listen address\n got: %v\nwant: %v",
			got,
			want,
		)
	}
}
