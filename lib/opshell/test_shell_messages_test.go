package opshell

/*
 * test_shell_messages_test.go
 * Tests for test_shell_messages.go
 * By J. Stuart McMurray
 * Created 20260730
 * Last Modified 20260814
 */

import (
	"io"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/tlog"
)

// does testTErrorf work?
func TestTestTErrorf(t *testing.T) {
	var (
		want string
		tef  = new(testTErrorf)
		s1   = tlog.S("s-1")
	)
	for _, c := range []struct {
		format string
		args   []any
		want   string
	}{{
		format: "foo: %s",
		args:   []any{s1},
		want:   "foo: " + s1,
	}, {
		format: "a\nb\nc: %d %v",
		args:   []any{123, io.EOF},
		want:   "a\nb\nc: 123 EOF",
	}} {
		tef.Errorf(c.format, c.args...)
		want += c.want + "\n"
	}
	if got := tef.String(); got != want {
		t.Errorf(
			"Incorrect lines buffered\ngot:\n%s\nwant:\n%s",
			got,
			want,
		)
	}
}

// Can we check for shell messages correctly?
func TestExpectShellMessages(t *testing.T) {
	var (
		msgs = []CLine{{
			Color: ColorBlue,
			Line:  tlog.S("line-1"),
		}, {
			Line:   tlog.S("line-2"),
			Prompt: tlog.S("prompt-2"),
		}, {
			Line:        tlog.S("line-3"),
			NoTimestamp: true,
			Plain:       true,
		}}

		och = make(chan CLine, len(msgs))
	)
	for _, msg := range msgs {
		och <- msg
	}
	ExpectShellMessages(t, och, msgs...)
}

// Do we log test errors on incorrect/missing messages?
func TestExpectShellMessages_Errorf(t *testing.T) {
	var (
		haveLine  = CLine{Line: tlog.S("have-line")}
		och       = make(chan CLine, 1)
		tef       = new(testTErrorf)
		wantLines = []CLine{{
			Line: tlog.S("want-line-1"),
		}, {
			Line: tlog.S("want-line-2"),
		}}

		want = `Incorrect shell message 1/2:
 got: opshell.CLine{Color:0, Line:"` + haveLine.Line + `", Prompt:"", NoTimestamp:false, Plain:false}
want: opshell.CLine{Color:0, Line:"` + wantLines[0].Line + `", Prompt:"", NoTimestamp:false, Plain:false}
Only got 1/2 shell messages
Missing shell message: opshell.CLine{Color:0, Line:"` + wantLines[1].Line + `", Prompt:"", NoTimestamp:false, Plain:false}` + "\n"
	)
	/* Prep one incorrect message. */
	och <- haveLine
	close(och)
	/* Check, should get errors. */
	expectShellMessages(tef, och, wantLines...)
	/* Did it work? */
	if got := tef.String(); got != want {
		t.Errorf(
			"Got incorrect error messages\ngot:\n%s\nwant:\n%s",
			got,
			want,
		)
	}
}

// Can we make sure there's no shell messages?
func TestExpectNoShellMessages(t *testing.T) {
	och := make(chan CLine)
	close(och)
	ExpectNoShellMessages(t, och)
}

// Do we log test errors on leftover shell messages?
func TestExpectNoShellMessages_Leftovers(t *testing.T) {
	var (
		tef  = new(testTErrorf)
		have = []CLine{{
			Line: tlog.S("line-1"),
		}, {
			Line: tlog.S("line-2"),
		}}

		och  = make(chan CLine, len(have))
		want = `Leftover shell message: opshell.CLine{Color:0, ` +
			`Line:"` + have[0].Line + `", Prompt:"", ` +
			`NoTimestamp:false, Plain:false}` + "\n" +
			`Leftover shell message: opshell.CLine{Color:0, ` +
			`Line:"` + have[1].Line + `", Prompt:"", ` +
			`NoTimestamp:false, Plain:false}` + "\n"
	)
	/* Channel with a leftover line. */
	for _, l := range have {
		och <- l
	}
	close(och)
	/* Make sure there's no leftover lines (there is one). */
	expectNoShellMessages(tef, och)
	/* Did it work? */
	if got := tef.String(); got != want {
		t.Errorf(
			"Got incorrect error message\ngot:\n%s\nwant:\n%s",
			got,
			want,
		)
	}
}
