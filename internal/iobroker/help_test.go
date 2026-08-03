package iobroker

/*
 * help_test.go
 * Tests for help.go
 * By J. Stuart McMurray
 * Created 20260722
 * Last Modified 20260801
 */

import (
	"errors"
	"io"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// Can we register and print help messages?
func TestBroker_HelpMessages(t *testing.T) {
	/* rs returns a HelpMessage returning s. */
	rs := func(s string) HelpMessage {
		return func() (string, error) { return s, nil }
	}

	var (
		hm1 = tlog.S("help-message-1")
		hm2 = tlog.S("help-message-2-1\nhelp-message-2-2")
		nlM = opshell.CLine{Line: "\n", Plain: true}

		wantMs = []opshell.CLine{
			{Color: opshell.ColorCyan, Line: hm1},
			nlM,
			{Color: opshell.ColorCyan, Line: hm2},
			nlM,
		}
		b, _, och, _, tb, sl = newTestBroker(
			t,
			t.Context(),
			&testBrokerConfig{helpMessages: []helpMessage{
				{name: "hm1", hm: rs(hm1)},
				{name: "hm2", hm: rs(hm2)},
			}},
		)
	)
	opshell.ExpectShellMessages(t, och, wantMs...)

	/* kerchunk connects and disconnects a shell. */
	kerchunk := func(t *testing.T) {
		t.Helper()
		var (
			pr, pw  = io.Pipe()
			id, tag = tlog.S("id"), tlog.S("tag")
			ech     = make(chan error, 1)
			m       = tlog.M.
				With(LKDirection, LVOutput).
				With(LKID, id)
		)
		/* Conect up a pipe. */
		go func() {
			ech <- b.HandleOutput(
				t.Context(),
				sl,
				id,
				tag,
				pr,
			)
		}()
		opshell.ExpectShellMessages(t, och, opshell.CLine{
			Color: LogColor,
			Line: taggedString(
				tag,
				"Output connected: ID %s",
				id,
			),
		})
		tb.Expect(t.Context(), t,
			m.
				Info(LMNewConnection),
		)
		/* Connected, now disconnect it. */
		pw.Close()
		opshell.ExpectShellMessages(t, och, opshell.CLine{
			Color: ErrColor,
			Line:  taggedString(tag, "Output connection closed"),
		})
		tb.Expect(t.Context(), t,
			m.
				Info(LMConnectionClosed),
		)
		if err := <-ech; nil != err {
			t.Errorf("Error from first HandleOutput: %v", err)
		}
		/* Should get help messages. */
		opshell.ExpectShellMessages(t, och, wantMs...)
	}

	/* Connect and disconnect a shell. */
	t.Run("first_shell", kerchunk)

	/* Add a new message for next time. */
	hm3 := tlog.S("help-message-3")
	wantMs = append(wantMs, opshell.CLine{
		Color: opshell.ColorCyan,
		Line:  hm3,
	}, nlM)
	b.RegisterHelpMessage("hm3", rs(hm3))
	t.Run("added help message three", kerchunk)

	/* What happens if a help message fails? */
	hmFail := errors.New(tlog.S("help-message-error"))
	wantMs = append(wantMs, opshell.CLine{
		Color: ErrColor,
		Line:  helpMessageErrorText("hmFail", hmFail),
	}, nlM)
	b.RegisterHelpMessage("hmFail", func() (string, error) {
		return "", hmFail
	})
	t.Run("added failed message", kerchunk)

	/* Does it work with another ok message? */
	hm4 := tlog.S("help-message-4")
	wantMs = append(wantMs, opshell.CLine{
		Color: opshell.ColorCyan,
		Line:  hm4,
	}, nlM)
	b.RegisterHelpMessage("hm4", rs(hm4))
	t.Run("added help message four", func(t *testing.T) {
		t.Run("1", kerchunk)
		t.Run("2", kerchunk)
	})
}

// Does help message error text look ok?
func TestHelpMessageErrorText(t *testing.T) {
	var (
		name = tlog.S("name")
		err  = errors.New(tlog.S("error"))

		want = "Error generating " + name + ": " + err.Error()
	)
	if got := helpMessageErrorText(name, err); got != want {
		t.Errorf(
			"Error text incorrect\n"+
				" name: %s\n"+
				"error: %v\n"+
				"  got: %v\n"+
				" want: %s",
			name,
			err,
			got,
			want,
		)
	}
}
