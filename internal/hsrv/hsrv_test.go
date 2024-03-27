package hsrv

/*
 * hsrv_test.go
 * Tests for hserv.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240327
 */

import (
	"context"
	"curlrevshell/lib/opshell"
	"fmt"
	"testing"
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
	s, cleanup, err := New("127.0.0.1:0", td, "", ich, och, "")
	if nil != err {
		t.Fatalf("Creating server: %s", err)
	}
	t.Cleanup(cleanup)

	ctx, cancel := context.WithCancel(context.Background())
	go s.Do(ctx)
	t.Cleanup(cancel)

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
			Color: ScriptColor,
			Line:  "\n",
		},
	}, {
		want: opshell.CLine{
			Color: ScriptColor,
			Line: fmt.Sprintf(
				CurlFormat,
				s.l.Fingerprint,
				s.l.Addr().String(),
			),
		},
	}, {
		want: opshell.CLine{
			Color: ScriptColor,
			Line:  "\n",
		},
	}, {
		want: opshell.CLine{
			Color: ScriptColor,
			Line:  "To get a shell:",
		},
	}, {
		want: opshell.CLine{
			Color: ScriptColor,
			Line:  "\n",
		},
	}, {
		want: opshell.CLine{
			Color: ScriptColor,
			Line: fmt.Sprintf(
				CurlFormat+ShellSuffix,
				s.l.Fingerprint,
				s.l.Addr().String(),
			),
		},
	}, {
		want: opshell.CLine{
			Color: ScriptColor,
			Line:  "\n",
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
