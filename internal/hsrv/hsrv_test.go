package hsrv

/*
 * hsrv_test.go
 * Tests for hserv.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240328
 */

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/opshell"
)

func newTestServer(t *testing.T) (
	chan<- string,
	<-chan opshell.CLine,
	*Server,
) {
	var (
		ich = make(chan string, 10)
		och = make(chan opshell.CLine, 10)
		td  = t.TempDir()
	)
	cbAddrs := []string{"kittens.com:8888", "moose.com"}
	s, cleanup, err := New("127.0.0.1:0", td, "", ich, och, "", cbAddrs)
	if nil != err {
		t.Fatalf("Creating server: %s", err)
	}
	t.Cleanup(cleanup)

	ctx, cancel := context.WithCancel(context.Background())
	go s.Do(ctx)
	t.Cleanup(cancel)

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
	wantLogs := []struct {
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
			Color:       ScriptColor,
			Line:        "\n",
			NoTimestamp: true,
		},
	}, {
		want: opshell.CLine{
			Color: ScriptColor,
			Line: fmt.Sprintf(
				CurlFormat+ShellSuffix,
				s.l.Fingerprint,
				cbAddrs[0],
			),
			NoTimestamp: true,
		},
	}, {
		want: opshell.CLine{
			Color: ScriptColor,
			Line: fmt.Sprintf(
				CurlFormat+ShellSuffix,
				s.l.Fingerprint,
				net.JoinHostPort(cbAddrs[1], listenPort),
			),
			NoTimestamp: true,
		},
	}, {
		want: opshell.CLine{
			Color: ScriptColor,
			Line: fmt.Sprintf(
				CurlFormat+ShellSuffix,
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
	for i, want := range wantLogs {
		got := <-och
		if nil != want.prep {
			got.Line = want.prep(got.Line)
		}
		if got != want.want {
			t.Errorf(
				"Incorrect log message:\n"+
					"   i: %d\n"+
					" got: %#v\n"+
					"want: %#v",
				i,
				got,
				want.want,
			)
		}
	}

	/* Don't keep going if we have an error. */
	if t.Failed() {
		t.FailNow()
	}

	return ich, och, s
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
