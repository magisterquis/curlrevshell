package hsrv

/*
 * hsrv_test.go
 * Tests for hserv.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240729
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
	"strings"
	"testing"
	"time"

	"github.com/magisterquis/curlrevshell/lib/opshell"
)

var (
	// errTestEnding indicates we're cancelling a context because the test
	// is over.
	errTestEnding = errors.New("test ending")
	// errServerDone indicates Server.Do returned
	errServerDone = errors.New("server.Do returned")
)

func newTestServer(t *testing.T) (
	chanLog, /* Server logs. */
	chan<- string, /* From shell */
	<-chan opshell.CLine,
	*Server,
) {
	var (
		cl  = chanLog(make(chan string, 1024))
		ich = make(chan string, 10)
		och = make(chan opshell.CLine, 10)
		ech = make(chan error, 1)
		td  = t.TempDir()
	)
	cbAddrs := []string{"kittens.com:8888", "moose.com"}
	s, cleanup, err := New(
		slog.New(slog.NewJSONHandler(
			chanLog(cl),
			&slog.HandlerOptions{Level: slog.LevelDebug},
		)),
		"127.0.0.1:0",
		td,
		"",
		ich,
		och,
		"",
		cbAddrs,
		true,
		false,
	)
	if nil != err {
		t.Fatalf("Creating server: %s", err)
	}

	/* Start the server going. */
	ctx, cancel := context.WithCancelCause(context.Background())
	go func() { ech <- s.Do(ctx); close(ech); cancel(errServerDone) }()

	t.Cleanup(func() {
		cleanup()   /* Stop the server politely. */
		go func() { /* And forcefully after a second. */
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
				cancel(errTestEnding)
			}
			if !timer.Stop() {
				<-timer.C
			}
		}()
		err := <-ech
		if nil != err && !errors.Is(
			err,
			ErrOneShellClosed,
		) && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("Unexpected server error: %s", err)
		}
		close(cl)
	})

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
	wantCLines := []struct {
		prep func(s string) string
		want opshell.CLine
	}{{
		want: opshell.CLine{
			Line: fmt.Sprintf("Listening on %s", s.l.Addr()),
		},
	}, {
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
	}, {
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
				),
				fmt.Sprintf(
					CurlFormat+ShellSuffix,
					s.l.Fingerprint,
					net.JoinHostPort(
						cbAddrs[1],
						listenPort,
					),
				),
				fmt.Sprintf(
					CurlFormat+ShellSuffix,
					s.l.Fingerprint,
					s.l.Addr().String(),
				),
			}, "\n") + "\n\n",
			NoTimestamp: true,
		},
	}}
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

	return cl, ich, och, s
}

func TestServer_Smoketest(t *testing.T) {
	newTestServer(t)
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

// chanLog wraps a chan string as a blockingish logfile.  Writes are sent as
// strings less timestamps and surrounding whitespace to the wrapped chan.
// Don't send it anything which isn't a log line.
type chanLog chan string

// Write converts b to a string and sends it to cl.  It always returns
// len(b), nil.
func (cl chanLog) Write(b []byte) (int, error) {
	line := string(b)

	/* Set the timestamp to the empty string. */
	parts := strings.Split(line, `"`)
	parts[3] = ""
	line = strings.Join(parts, `"`)

	/* Remove excess spaces. */
	line = strings.TrimSpace(line)

	/* Send it out. */
	cl <- string(line)
	return len(b), nil
}

// Expect expects the log entries on cl.  It calls t.Errorf for mismatches and
// blocks until as many log entries as supplied lines are read.
func (cl chanLog) Expect(t *testing.T, lines ...string) {
	t.Helper()
	for _, want := range lines { /* Make sure we get each line. */
		got, ok := <-cl
		if !ok { /* Channel closed early. */
			t.Errorf(
				"Log channel closed while waiting for %q",
				got,
			)
			return
		} else if got != want { /* Wrong line. */
			t.Errorf(
				"Unexpected log line:\n got: %s\nwant: %s",
				got,
				want,
			)
		}
	}
}

// ExpectEmpty is like cl.Expect, except it checks that the log is empty and
// calls t.Errorf which each line if not.  This is inherently racy and should
// be called concurrently to calls to cl.Write.
func (cl chanLog) ExpectEmpty(t *testing.T, lines ...string) {
	t.Helper()
	/* Check for lines we want. */
	cl.Expect(t, lines...)
	/* Anything left is an error. */
	for 0 != len(cl) {
		select { /* Check to see if we have anything.  */
		case l, ok := <-cl: /* We do, nuts. */
			if !ok { /* Closed.  This works. */
				return
			}
			/* Tell someone about the unexpected line. */
			t.Errorf("Unexpected log line: %s", l)
		default: /* Empty, good. */
			return
		}
	}
}

func TestChanLog(t *testing.T) {
	cl := chanLog(make(chan string, 10))
	sl := slog.New(slog.NewJSONHandler(chanLog(cl), nil))
	have := "kittens"
	want := `{"time":"","level":"INFO","msg":"kittens"}`

	t.Run("Expect", func(t *testing.T) {
		sl.Info(have)
		cl.Expect(t, want)
	})

	t.Run("ExpectEmpty", func(t *testing.T) {
		sl.Info(have)
		cl.ExpectEmpty(t, want)
	})
}

// Make sure the server returns after a single shell, if oneShell is set.
func TestServer_OneShell(t *testing.T) {
	cl, _, _, s := newTestServer(t)
	s.oneShell = true /* Only handle one shell. */

	/* HTTP Client which does not certificate validation. */
	httpc := http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		}},
	}

	/* Connect a shell. */
	id := "kittens"
	ech := make(chan error, 2)
	doneCh := make(chan struct{})
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

	/* Wait for connections to happen and the listener to close. */
	type lmsg struct {
		Msg       string
		Direction string
	}
	var (
		got  = make(map[lmsg]int)
		want = map[lmsg]int{
			{Msg: LMOneShellClosingListener}:            1,
			{Msg: LMNewConnection, Direction: "input"}:  1,
			{Msg: LMNewConnection, Direction: "output"}: 1,
		}
	)
	for i := 0; i < 3; i++ {
		select {
		case l := <-cl:
			var msg lmsg
			if err := json.Unmarshal([]byte(l), &msg); nil != err {
				t.Fatalf("Error unmarshaling %s: %s", l, err)
			}
			/* Direction doesn't matter when we close the
			listener. */
			if LMOneShellClosingListener == msg.Msg {
				msg.Direction = ""
			}
			got[msg]++
		case err := <-ech:
			t.Fatalf(
				"Request error before listener closed: %s",
				err,
			)
		}
	}
	if !maps.Equal(got, want) {
		t.Fatalf("Incorrect logs:\ngot: %q\nwant: %q", got, want)
	}

	close(doneCh)
	for i := 0; i < len(ech); i++ {
		if err := <-ech; nil != err {
			t.Errorf(
				"Request error after listener closed: %s",
				err,
			)
		}
	}

	cl.ExpectEmpty(t)
}
