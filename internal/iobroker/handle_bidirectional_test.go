package iobroker

/*
 * handle_bidirectional_test.go
 * Tests for handle_bidirectional.go
 * By J. Stuart McMurray
 * Created 20260723
 * Last Modified 20260801
 */

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/internal/bidirpipe"
	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// Can we handle a bidirectional stream?
func TestBrokerHandleBidirectional_SimpleShells(t *testing.T) {
	b, ich, och, _, tb, sl := newTestBroker(t, t.Context(), nil)

	runShell := func(t *testing.T) {
		var (
			eg, ctx     = ctxerrgroup.WithContext(t.Context())
			id, tag     = tlog.S("id"), tlog.S("tag")
			pc, ps      = bidirpipe.New()
			shellPrefix = tlog.S("shell-prefix")
			exitCmd     = tlog.S("exit")
			opIn        = []string{
				tlog.S("op-in-1"),
				tlog.S("op-in-2"),
				tlog.S("op-in-3"),
				tlog.S("op-in-4"),
			}

			m = tlog.M.With(LKID, id)
		)
		t.Cleanup(func() {
			if err := pc.Close(); nil != err {
				t.Errorf(
					"Error closing client end of pipe: %v",
					err,
				)
			}
			if err := ps.Close(); nil != err {
				t.Errorf(
					"Error closing server end of pipe: %v",
					err,
				)
			}
			eg.Wait() /* Leaky? */
		})

		/* Handle connection. */
		eg.GoTag(ctx, "handler", func(ctx context.Context) error {
			return b.HandleBidirectional(ctx, sl, id, tag, ps)
		})
		opshell.ExpectShellMessages(t, och,
			opshell.CLine{
				Color: LogColor,
				Line:  taggedString(tag, "Connected: ID %s", id),
			},
			opshell.CLine{
				Color: LogColor,
				Line:  taggedString(tag, SMShellIsReady),
			},
		)
		bm := m.
			With(LKDirection, LVBidir)
		tb.Expect(t.Context(), t,
			bm.
				Info(LMNewConnection),
			bm.
				Info(LMShellStarting),
		)

		/* Dummy shell. */
		eg.GoTag(ctx, "shell", func(ctx context.Context) error {
			defer pc.Close()
			scanner := bufio.NewScanner(pc)
			for scanner.Scan() {
				l := scanner.Text()
				/* Analogous to telling the shell to exit. */
				if exitCmd == l {
					return nil
				}
				/* Not exit, send back the line. */
				if _, err := fmt.Fprintf(
					pc,
					"%s-%s\n",
					shellPrefix,
					l,
				); nil != err {
					return fmt.Errorf("write: %w", err)
				}
			}
			if err := scanner.Err(); nil != err {
				return fmt.Errorf("read: %w", err)
			}
			return nil
		})

		/* Send to the shell. */
		eg.GoTag(ctx, "operator_input", func(ctx context.Context) error {
			for _, s := range opIn {
				ich <- s
			}
			return nil
		})

		/* Did we get shell output and logs? */
		wantCLines := make([]opshell.CLine, len(opIn))
		for i, want := range opIn {
			wantCLines[i] = opshell.CLine{
				Line:  shellPrefix + "-" + want + "\n",
				Plain: true,
			}
		}
		opshell.ExpectShellMessages(t, och, wantCLines...)

		/* Were our lines logged? */
		wantLines := make([]tlog.Msg, 0, 2*len(opIn))
		for _, want := range opIn {
			want += "\n"
			wantLines = append(
				wantLines,
				m.
					With(LKDirection, LVInput).
					With(LKData, want).
					Info(LMShellIO),
				m.
					With(LKDirection, LVOutput).
					With(LKData, shellPrefix+"-"+want).
					Info(LMShellIO),
			)
		}
		tb.WithExpectUnordered().Expect(t.Context(), t, wantLines...)

		/* Tell the shell to exit. */
		ich <- exitCmd

		/* Do we get disconnect messages? */
		wantCLines = wantCLines[:0]
		for _, want := range []string{
			SMConnectionClosed,
			SMShellIsGone,
		} {
			wantCLines = append(wantCLines, opshell.CLine{
				Color: ErrColor,
				Line:  taggedString(tag, "%s", want),
			})
		}
		opshell.ExpectShellMessages(t, och, wantCLines...)

		/* Logs look ok? */
		tb.WithExpectUnordered().Expect(t.Context(), t,
			m.
				With(LKData, exitCmd+"\n").
				With(LKDirection, LVInput).
				Info(LMShellIO),
			bm.
				Info(LMConnectionClosed),
		)
		tb.WithExpectEmpty().Expect(t.Context(), t,
			bm.
				Info(LMShellFinished),
		)

		/* Everything go ok? */
		if err := eg.Wait(); nil != err {
			t.Fatalf("Error: %v", err)
		}
	}

	/* Hook up a few shells in a row. */
	nShells := 3
	for i := range nShells {
		t.Run(
			fmt.Sprintf("%d_of_%d", i+1, nShells),
			func(t *testing.T) { synctest.Test(t, runShell) },
		)
	}
}

// Do we allow a bidirectional stream with an empty ID?
func TestBrokerHandleBidirectional_NoID(t *testing.T) {
	var (
		tag                  = tlog.S("tag")
		pc, ps               = bidirpipe.New()
		b, _, och, _, tb, sl = newTestBroker(t, t.Context(), nil)
	)

	/* Don't really want to handle anything. */
	if err := pc.Close(); nil != err {
		t.Errorf(
			"Error closing client end of pipe: %v",
			err,
		)
	}

	/* Handle the stream, which won't take long. */
	if err := b.HandleBidirectional(
		t.Context(),
		sl,
		"",
		tag,
		ps,
	); nil != err {
		t.Errorf("HandleBidiectional error: %v", err)
	}

	/* First shell output line will give us the generated ID. */
	l, ok := <-och
	if !ok {
		t.Fatalf("Did not get first shell output line")
	}
	ms := regexp.MustCompile(
		`Connected: ID (\S+)$`,
	).FindStringSubmatch(l.Line)
	if 2 != len(ms) {
		t.Fatalf("Invalid first line: %q", l.Line)
	} else if "" == ms[1] {
		t.Fatalf("Did not get a generated ID")
	}
	id := ms[1]

	/* Shell output correct? */
	toch := make(chan opshell.CLine, 1)
	toch <- l
	opshell.ExpectShellMessages(t, toch, opshell.CLine{
		Color: LogColor,
		Line:  taggedString(tag, "Connected: ID %s", id),
	})
	wantCLines := make([]opshell.CLine, 0, 3)
	for _, want := range []struct {
		s string
		c opshell.Color
	}{
		{s: SMShellIsReady, c: LogColor},
		{s: SMConnectionClosed, c: ErrColor},
		{s: SMShellIsGone, c: ErrColor},
	} {
		wantCLines = append(wantCLines, opshell.CLine{
			Color: want.c,
			Line:  taggedString(tag, "%s", want.s),
		})
	}
	opshell.ExpectShellMessages(t, och, wantCLines...)

	/* Logs correct? */
	m := tlog.M.
		With(LKID, id).
		With(LKDirection, LVBidir)
	tb.Expect(t.Context(), t,
		m.
			Info(LMNewConnection),
		m.
			Info(LMShellStarting),
		m.
			Info(LMConnectionClosed),
		m.
			Info(LMShellFinished),
	)
}

// Do we give up if the context is done before getting handling?
func TestBrokerHandleBidirectional_ContextDoneBeforeHandle(t *testing.T) {
	synctest.Test(t, testBrokerHandleBidirectionalContextDoneBeforeHandle)
}
func testBrokerHandleBidirectionalContextDoneBeforeHandle(t *testing.T) {
	var (
		ctx, cancel       = context.WithCancel(t.Context())
		id, tag           = tlog.S("id"), tlog.S("tag")
		b, _, _, _, _, sl = newTestBroker(t, t.Context(), nil)
		_, ps             = bidirpipe.New()
	)

	/* Handle the stream, which won't take long. */
	cancel()
	if err := b.HandleBidirectional(
		ctx,
		sl,
		id,
		tag,
		ps,
	); nil != err {
		t.Errorf("HandleBidiectional error: %v", err)
	}
}

// Do we give up if the context is done between getting opshell channels (isi)?
func TestBrokerHandleBidirectional_ContextDoneBeforeISI(t *testing.T) {
	synctest.Test(t, testBrokerHandleBidirectionalContextDoneBeforeISI)
}
func testBrokerHandleBidirectionalContextDoneBeforeISI(t *testing.T) {
	var (
		_, ps             = bidirpipe.New()
		b, _, _, _, _, sl = newTestBroker(t, t.Context(), nil)
		ctx, cancel       = context.WithCancel(t.Context())
		ech               = make(chan error, 1)
		id, tag           = tlog.S("id"), tlog.S("tag")
		gotOSI            = make(chan struct{})
		getISI            = make(chan struct{})
	)

	/* Make sure we don't ever get osi. */
	isi := <-b.isiCh
	defer func() { b.isiCh <- isi }()

	/* Handle the stream, which won't take long. */
	ctx = context.WithValue(ctx, testGotTSIKey[outputStreamInfo]{}, gotOSI)
	ctx = context.WithValue(ctx, testGetTSIKey[inputStreamInfo]{}, getISI)
	go func() {
		ech <- b.HandleBidirectional(
			ctx,
			sl,
			id,
			tag,
			ps,
		)
	}()

	/* Wait until we've got isi, cancel the context, and get osi. */
	<-gotOSI
	cancel()
	synctest.Wait()
	close(getISI)

	/* Should be done soon. */
	if err := <-ech; nil != err {
		t.Errorf("HandleBidiectional error: %v", err)
	}
}

// Do we give up if the context is done between getting opshell channels (osi)?
func TestBrokerHandleBidirectional_ContextDoneBeforeOSI(t *testing.T) {
	synctest.Test(t, testBrokerHandleBidirectionalContextDoneBeforeOSI)
}
func testBrokerHandleBidirectionalContextDoneBeforeOSI(t *testing.T) {
	var (
		_, ps             = bidirpipe.New()
		b, _, _, _, _, sl = newTestBroker(t, t.Context(), nil)
		ctx, cancel       = context.WithCancel(t.Context())
		ech               = make(chan error, 1)
		id, tag           = tlog.S("id"), tlog.S("tag")
		gotISI            = make(chan struct{})
		getOSI            = make(chan struct{})
	)

	/* Make sure we don't ever get osi. */
	osi := <-b.osiCh
	defer func() { b.osiCh <- osi }()

	/* Handle the stream, which won't take long. */
	ctx = context.WithValue(ctx, testGotTSIKey[inputStreamInfo]{}, gotISI)
	ctx = context.WithValue(ctx, testGetTSIKey[outputStreamInfo]{}, getOSI)
	go func() {
		ech <- b.HandleBidirectional(
			ctx,
			sl,
			id,
			tag,
			ps,
		)
	}()

	/* Wait until we've got isi, cancel the context, and get osi. */
	<-gotISI
	cancel()
	synctest.Wait()
	close(getOSI)

	/* Should be done soon. */
	if err := <-ech; nil != err {
		t.Errorf("HandleBidiectional error: %v", err)
	}
}

// Do we give up if the broker is done before getting any opshell channels?
func TestBrokerHandleBidirectional_BrokerDoneBeforeChannels(t *testing.T) {
	synctest.Test(t, testBrokerHandleBidirectionalBrokerDoneBeforeChannels)
}
func testBrokerHandleBidirectionalBrokerDoneBeforeChannels(t *testing.T) {
	var (
		ctx, cancel = context.WithCancel(t.Context())
		id, tag     = tlog.S("id"), tlog.S("tag")
		_, ps       = bidirpipe.New()

		b, _, _, done, _, sl = newTestBroker(t, ctx, nil)
	)
	cancel()
	<-done

	/* Handle the stream, which won't take long. */
	if got, want := b.HandleBidirectional(
		t.Context(),
		sl,
		id,
		tag,
		ps,
	), ErrNotRunning; !errors.Is(got, want) {
		t.Errorf("Incorrect error\n got: %v\nwant: %v", got, want)
	}
}

// Do we note an already-connected stream properly?
func TestBrokerHandleBidirectional_AlreadyConnected(t *testing.T) {
	synctest.Test(t, testBrokerHandleBidirectionalAlreadyConnected)
}
func testBrokerHandleBidirectionalAlreadyConnected(t *testing.T) {
	var (
		b, _, och, _, tb, sl = newTestBroker(t, t.Context(), nil)
		ido, tago            = tlog.S("id-o"), tlog.S("tag-o")
		idb, tagb            = tlog.S("id-b"), tlog.S("tag-b")
		pb, _                = bidirpipe.New()
		pr, pw               = io.Pipe()
		ech                  = make(chan error, 1)
	)

	/* Connect up an output connection. */
	go func() { ech <- b.HandleOutput(t.Context(), sl, ido, tago, pr) }()
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: LogColor,
		Line:  taggedString(tago, "Output connected: ID %s", ido),
	})
	mo := tlog.M.
		With(LKDirection, LVOutput).
		With(LKID, ido)
	tb.Expect(t.Context(), t,
		mo.
			Info(LMNewConnection),
	)

	/* Handle the bidirectional stream, which won't take long. */
	if got, want := b.HandleBidirectional(
		t.Context(),
		sl,
		idb,
		tagb,
		pb,
	), ErrStreamAlreadyConnected; !errors.Is(got, want) {
		t.Errorf("Incorrect error\n got: %v\nwant: %v", got, want)
	}
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ErrColor,
		Line: taggedString(
			tagb,
			"Rejected unexpected connection with ID %q",
			idb,
		),
	})
	tb.Expect(t.Context(), t,
		tlog.M.
			With(LKDirection, LVBidir).
			With(LKID, idb).
			Warn(LMAlreadyConnected),
	)

	/* Disconnect the output connection. */
	if err := pw.Close(); nil != err {
		t.Errorf("Error closing output stream: %v", err)
	}
	if err := <-ech; nil != err {
		t.Errorf("HandleOutput returned error: %v", err)
	}
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ErrColor,
		Line:  taggedString(tago, "Output connection closed"),
	})
	tb.Expect(t.Context(), t,
		mo.
			Info(LMConnectionClosed),
	)
}

// Do we note an unexpected proxy error?
func TestBrokerHandleBidirectional_ProxyError(t *testing.T) {
	synctest.Test(t, testBrokerHandleBidirectionalProxyError)
}
func testBrokerHandleBidirectionalProxyError(t *testing.T) {
	var (
		_, ps                = bidirpipe.New()
		b, _, och, _, tb, sl = newTestBroker(t, t.Context(), nil)
		ech                  = make(chan error, 1)
		id, tag              = tlog.S("id"), tlog.S("tag")
		wantErr              = io.ErrClosedPipe
	)

	/* Handle connection. */
	go func() {
		ech <- b.HandleBidirectional(
			t.Context(),
			sl,
			id,
			tag,
			ps,
		)
	}()
	opshell.ExpectShellMessages(t, och,
		opshell.CLine{
			Color: LogColor,
			Line:  taggedString(tag, "Connected: ID %s", id),
		},
		opshell.CLine{
			Color: LogColor,
			Line:  taggedString(tag, SMShellIsReady),
		},
	)
	m := tlog.M.
		With(LKDirection, LVBidir).
		With(LKID, id)
	tb.Expect(t.Context(), t,
		m.
			Info(LMNewConnection),
		m.
			Info(LMShellStarting),
	)

	/* Cause an error. */
	if err := ps.Close(); nil != err {
		t.Errorf("Error closing server end of pipe: %v", err)
	}
	if got, want := <-ech, wantErr; !errors.Is(got, want) {
		t.Errorf("Incorrect error\n got: %v\nwant: %v", got, want)
	}

	/* Do we get disconnect messages? */
	wantCLines := make([]opshell.CLine, 2)
	for i, want := range []string{
		SMConnectionClosed,
		SMShellIsGone,
	} {
		wantCLines[i] = opshell.CLine{
			Color: ErrColor,
			Line:  taggedString(tag, "%s", want),
		}
	}
	opshell.ExpectShellMessages(t, och, wantCLines...)

	/* Logs look ok? */
	m = m.With(LKError, "output: "+wantErr.Error())
	tb.Expect(t.Context(), t,
		m.
			Info(LMConnectionClosed),
		m.
			Info(LMShellFinished),
	)
}
