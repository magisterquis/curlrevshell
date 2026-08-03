package iobroker

/*
 * handle_common_test.go
 * Tests for handle_common.go
 * By J. Stuart McMurray
 * Created 20260722
 * Last Modified 20260801
 */

import (
	"context"
	"errors"
	"io"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// Do we disallow empty IDs?
func TestHandleCommon_EmptyID(t *testing.T) {
	var (
		b, _, _, _, tb, sl = newTestBroker(t, t.Context(), nil)
	)

	/* Start handling output. */
	if got, want := b.HandleOutput(t.Context(), sl, "", "", nil),
		ErrIDEmpty; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error with empty ID\n got: %v\nwant: %v",
			got,
			want,
		)
	}

	tb.Expect(t.Context(), t,
		tlog.M.
			Warn(LMIDMissing),
	)
}

// Do we prevent simultaneous connections on the same side?
func TestHandleCommon_SameDirectionTwice(t *testing.T) {
	synctest.Test(t, testHandleCommonSameDirectionTwice)
}
func testHandleCommonSameDirectionTwice(t *testing.T) {
	var (
		ctx, cancel             = context.WithCancel(t.Context())
		b, _, och, done, tb, sl = newTestBroker(t, ctx, nil)
		ech                     = make(chan error, 1)
		id1, tag1               = tlog.S("id1"), tlog.S("tag1")
		id2, tag2               = tlog.S("id2"), tlog.S("tag2")
		pr, _                   = io.Pipe()
	)
	defer cancel()

	/* Start handling output. */
	go func() { ech <- b.HandleOutput(t.Context(), sl, id1, tag1, pr) }()
	opshell.ExpectShellMessages(t, och,
		opshell.CLine{
			Color: LogColor,
			Line:  taggedString(tag1, "Output connected: ID %s", id1),
		},
	)
	m := tlog.M.
		With(LKDirection, LVOutput)
	m1 := m.
		With(LKID, id1)
	tb.Expect(t.Context(), t,
		m1.
			Info(LMNewConnection),
	)

	/* Try to hook up another output. */
	if got, want := b.HandleOutput(t.Context(), sl, id2, tag2, pr),
		ErrStreamAlreadyConnected; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error with empty ID\n got: %v\nwant: %v",
			got,
			want,
		)
	}
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ErrColor,
		Line: taggedString(
			tag2,
			"Rejected unexpected output connection with ID %q",
			id2,
		),
	})
	tb.Expect(t.Context(), t,
		m.
			With(LKID, id2).
			Warn(LMAlreadyConnected),
	)

	/* All work? */
	cancel()
	<-done
	if err := <-ech; nil != err {
		t.Errorf("HandleOutput returned error: %v", err)
	}
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ErrColor,
		Line:  taggedString(tag1, "Output connection closed"),
	})
	tb.Expect(t.Context(), t,
		m1.
			Info(LMConnectionClosed),
	)
}

// Do we prevent connections on diffent sides with different IDs?
func TestHandleCommon_DifferentIDs(t *testing.T) {
	var (
		ctx, cancel             = context.WithCancel(t.Context())
		b, _, och, done, tb, sl = newTestBroker(t, ctx, nil)
		ech                     = make(chan error, 1)
		idIn, tagIn             = tlog.S("id-in"), tlog.S("tag-in")
		idOut, tagOut           = tlog.S("id-out"), tlog.S("tag-out")
		pr, pw                  = io.Pipe()
	)
	defer cancel()

	/* Start handling output. */
	go func() { ech <- b.HandleOutput(t.Context(), sl, idOut, tagOut, pr) }()
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: LogColor,
		Line:  taggedString(tagOut, "Output connected: ID %s", idOut),
	})
	mOut := tlog.M.
		With(LKDirection, LVOutput).
		With(LKID, idOut)
	tb.Expect(t.Context(), t,
		mOut.
			Info(LMNewConnection),
	)

	/* Try to hook up another output. */
	if got, want := b.HandleInput(t.Context(), sl, idIn, tagIn, pw),
		ErrIDIncorrect; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error with empty ID\n got: %v\nwant: %v",
			got,
			want,
		)
	}
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ErrColor,
		Line: taggedString(
			tagIn,
			"Rejected input connection with incorrect ID %q, "+
				"expected %q",
			idIn,
			idOut,
		),
	})
	tb.Expect(t.Context(), t,
		tlog.M.
			Warn(LMIDIncorrect).
			With(LKDirection, LVInput).
			With(LKID, idIn),
	)

	/* All work? */
	cancel()
	<-done
	if err := <-ech; nil != err {
		t.Errorf("HandleOutput returned error: %v", err)
	}
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ErrColor,
		Line:  taggedString(tagOut, "Output connection closed"),
	})
	tb.Expect(t.Context(), t,
		mOut.
			Info(LMConnectionClosed),
	)
}

// Do we get whined at if Run's cancelled?
func TestHandleCommon_RunCancelled(t *testing.T) {
	var (
		ctx, cancel          = context.WithCancel(t.Context())
		b, _, _, done, _, sl = newTestBroker(t, ctx, nil)
		id, tag              = tlog.S("id"), tlog.S("tag")
		pr, _                = io.Pipe()
	)
	cancel()

	<-done

	/* Fail to start handling output. */
	if got, want := b.HandleOutput(t.Context(), sl, id, tag, pr),
		ErrNotRunning; !errors.Is(got, want) {
		t.Errorf("Incorrect error\n got: %v\nwant: %v", got, want)
	}
}

// Do we get whined at if our conext iss cancelled?
func TestHandleCommon_ContextCancelled(t *testing.T) {
	var (
		b, _, _, _, _, sl = newTestBroker(t, t.Context(), nil)
		id, tag           = tlog.S("id"), tlog.S("tag")
		pr, _             = io.Pipe()
	)

	/* Fail to start handling output. */
	osi := <-b.osiCh /* Kinda a dick move. */
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := b.HandleOutput(ctx, sl, id, tag, pr); nil != err {
		t.Errorf("Incorrect error: %v", err)
	}
	b.osiCh <- osi /* Ok, less bad. */
}
