package iobroker

/*
 * iobroker_test.go
 * Tests for iobroker.go
 * By J. Stuart McMurray
 * Created 20260620
 * Last Modified 20260814
 */

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
	"github.com/magisterquis/curlrevshell/lib/tlog"
)

const testChanLen = 128

// testBrokerConfig configures the broker returned from newTestBroker.
type testBrokerConfig struct {
	oneShell     bool
	wantErr      error /* What Broker.Run should return. */
	helpMessages []helpMessage
}

// newTestBroker returns a new running broker plus i/o channels and a channel
// which will be closed when the broker's Run method returns.
// The returned tlog.Buffer must be empty when the test finishes.
//
// Call like
//
//	b, ich, och, done, tb, sl := newTestBroker(t, ctx, nil)
func newTestBroker(
	t *testing.T,
	ctx context.Context,
	conf *testBrokerConfig,
) (
	*Broker,
	chan<- string, /* ich */
	<-chan opshell.CLine, /* och */
	<-chan struct{}, /* Broker finished. */
	*tlog.Buffer,
	*slog.Logger,
) {
	var (
		ich        = make(chan string, testChanLen)
		och        = make(chan opshell.CLine, testChanLen)
		ech        = make(chan error, 1)
		done       = make(chan struct{})
		runStarted = make(chan struct{})
		tb, sl     = tlog.NewBuffer()

		b = New(och)
	)

	/* Make sure we have a config, even an empty one. */
	if nil == conf {
		conf = new(testBrokerConfig)
	}

	/* Add help functions, if we have them. */
	for _, hm := range conf.helpMessages {
		b.RegisterHelpMessage(hm.name, hm.hm)
	}

	/* Start a broker running. */
	ctx = context.WithValue(ctx, testRunStartedKey{}, runStarted)
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer close(done)
		ech <- b.Run(ctx, ich, conf.oneShell)
	}()
	<-runStarted

	/* Make sure it finishes.  Shouldn't get an error. */
	t.Cleanup(func() {
		cancel()
		<-done
		/* Safely drain and close ich. */
		safeClose(ich)
		/* och is a bit easier. */
		close(och)
		/* Drain the output channel. */
		opshell.ExpectNoShellMessages(t, och)
		if got := <-ech; !errors.Is(got, conf.wantErr) {
			t.Errorf(
				"Broker returned incorrect error\n"+
					" got: %v\n"+
					"want: %v",
				got,
				conf.wantErr,
			)
		}
		tb.CloseExpectEmpty(context.Background(), t)
	})

	return b, ich, och, done, tb, sl
}

// Can we make, start, and stop a Broker?
func TestBroker_Smoketest(t *testing.T) {
	synctest.Test(t, testBrokerSmoketest)
}
func testBrokerSmoketest(t *testing.T) {
	newTestBroker(t, t.Context(), nil)
}

// Can we stop a broker by cancelling a context before it starts??
func TestBroker_CancelContextBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, _, _, done, _, _ := newTestBroker(t, ctx, nil)
	cancel()
	<-done
}

// Do we get a panic if we call Run twice?
func TestBroker_RunTwicePanic(t *testing.T) {
	synctest.Test(t, testBrokerRunTwicePanic)
}
func testBrokerRunTwicePanic(t *testing.T) {
	/* Should panic eventually. */
	b, _, _, _, _, _ := newTestBroker(t, t.Context(), nil)
	defer func() {
		if v := recover(); nil == v {
			t.Errorf("No panic on second run")
		} else if got, ok := v.(error); !ok {
			t.Errorf("Non-error %T recovered: %v", v, v)
		} else if want := errBrokerAlreadyRunning; !errors.Is(
			got,
			want,
		) {
			t.Errorf(
				"Incorrect recovered error\n"+
					" got: %v\n"+
					"want: %v",
				got,
				want,
			)
		}
	}()

	/* Panic? */
	err := b.Run(t.Context(), make(chan string), true)
	t.Fatalf("Second run did not panic and returned %v", err)
}

// Can we handle cancelling the Broker's context after a shell connects?
func TestBroker_ConnectAndCancelContext(t *testing.T) {
	synctest.Test(t, testBrokerConnectAndCancelContext)
}
func testBrokerConnectAndCancelContext(t *testing.T) {
	/* Context things. */
	ctx, cancel := context.WithCancel(t.Context())
	eg, ctx := ctxerrgroup.WithContext(ctx)
	var (
		b, _, och, done, tb, sl = newTestBroker(t, ctx, nil)
		id                      = tlog.S("id")
		_, inw                  = io.Pipe()
		outr, _                 = io.Pipe()
		tag                     = tlog.S("tag")
		m                       = tlog.M.With(LKID, id)
	)

	/* Hook up a shell. */
	eg.GoTag(ctx, "input_stream", func(ctx context.Context) error {
		return b.HandleInput(t.Context(), sl, id, tag, inw)
	})
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: LogColor,
		Line:  fmt.Sprintf("[%s] Input connected: ID %s", tag, id),
	})
	tb.Expect(t.Context(), t,
		m.
			With(LKDirection, LVInput).
			Info(LMNewConnection),
	)
	eg.GoTag(ctx, "output_stream", func(ctx context.Context) error {
		return b.HandleOutput(t.Context(), sl, id, tag, outr)
	})
	opshell.ExpectShellMessages(t, och,
		opshell.CLine{
			Color: LogColor,
			Line: fmt.Sprintf(
				"[%s] Output connected: ID %s",
				tag,
				id,
			),
		},
		opshell.CLine{
			Color: LogColor,
			Line:  fmt.Sprintf("[%s] %s", tag, SMShellIsReady),
		},
	)
	tb.Expect(t.Context(), t,
		m.
			With(LKDirection, LVOutput).
			Info(LMNewConnection),
	)

	cancel()
	<-done
	if err := eg.Wait(); nil != err {
		t.Errorf("Handle error: %v", err)
	}

	/* Do we get disconnect messages? */
	var (
		wantSs = []string{
			"Output connection closed",
			"Input connection closed",
			SMShellIsGone,
		}

		wantMs = make(map[opshell.CLine]struct{})
	)
	for _, wantS := range wantSs {
		wantMs[opshell.CLine{
			Color: ErrColor,
			Line:  taggedString(tag, "%s", wantS),
		}] = struct{}{}
	}
	for range len(wantSs) {
		got := <-och
		if _, ok := wantMs[got]; ok {
			delete(wantMs, got)
			continue
		}
		t.Errorf("Unexpected output line: %#v", got)
	}
	for _, wantM := range wantMs {
		t.Errorf("Missing output line: %#v", wantM)
	}

	/* Logs look ok? */
	tb.Expect(t.Context(), t,
		m.
			Info(LMShellStarting),
	)
	tb.WithExpectUnordered().Expect(t.Context(), t,
		m.
			With(LKDirection, LVOutput).
			Info(LMConnectionClosed),
		m.
			With(LKDirection, LVInput).
			Info(LMConnectionClosed),
	)
	tb.Expect(t.Context(), t,
		m.
			Info(LMShellFinished),
	)
}

// Can we handle a simple shell connecting?
func TestBroker_SimpleShell(t *testing.T) {
	synctest.Test(t, testBrokerSimpleShell)
}
func testBrokerSimpleShell(t *testing.T) {
	/* Context things. */
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	/* Test broker. */
	var (
		id                        = tlog.S("id")
		inr, inw                  = io.Pipe()
		outr, outw                = io.Pipe()
		shellPrefix               = tlog.S("shell-prefix")
		tag                       = tlog.S("tag")
		b, ich, och, done, tb, sl = newTestBroker(
			t,
			ctx,
			&testBrokerConfig{wantErr: ErrInputClosed},
		)
		opIn = []string{
			tlog.S("op-in-1"),
			tlog.S("op-in-2"),
			tlog.S("op-in-3"),
			tlog.S("op-in-4"),
		}

		m = tlog.M.With(LKID, id)
	)

	eg, ctx := ctxerrgroup.WithContext(ctx)

	/* Hook up a shell. */
	eg.GoTag(ctx, "input_stream", func(ctx context.Context) error {
		defer inw.Close()
		return b.HandleInput(ctx, sl, id, tag, inw)
	})
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: LogColor,
		Line:  fmt.Sprintf("[%s] Input connected: ID %s", tag, id),
	})
	tb.Expect(t.Context(), t,
		m.
			With(LKDirection, LVInput).
			Info(LMNewConnection),
	)
	eg.GoTag(ctx, "output_stream", func(ctx context.Context) error {
		return b.HandleOutput(ctx, sl, id, tag, outr)
	})
	opshell.ExpectShellMessages(t, och,
		opshell.CLine{
			Color: LogColor,
			Line:  fmt.Sprintf("[%s] Output connected: ID %s", tag, id),
		},
		opshell.CLine{
			Color: LogColor,
			Line:  fmt.Sprintf("[%s] %s", tag, SMShellIsReady),
		},
	)
	tb.Expect(t.Context(), t,
		m.
			With(LKDirection, LVOutput).
			Info(LMNewConnection),
		m.
			Info(LMShellStarting),
	)
	if t.Failed() {
		t.FailNow()
	}

	/* Shell reads lines and writes them back with a prefix. */
	eg.GoTag(ctx, "shell", func(ctx context.Context) error {
		defer outw.Close()
		scanner := bufio.NewScanner(inr)
		for scanner.Scan() {
			l := scanner.Text()
			if _, err := fmt.Fprintf(
				outw,
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

	/* Operator sends a bunch of lines and expects them back with the
	shell's prefix. */
	eg.GoTag(ctx, "operator_input", func(ctx context.Context) error {
		for _, s := range opIn {
			ich <- s
		}
		return nil
	})

	/* Did we get shell output? */
	wantCLines := make([]opshell.CLine, len(opIn))
	for i, want := range opIn {
		wantCLines[i] = opshell.CLine{
			Line:  shellPrefix + "-" + want + "\n",
			Plain: true,
		}
	}
	opshell.ExpectShellMessages(t, och, wantCLines...)

	/* Looks good, close the shell's input to start everything stopping. */
	close(ich)

	/* Do we get disconnect messages? */
	wantCLines = wantCLines[:0]
	for _, want := range []string{
		"Input connection closed",
		"Output connection closed",
		SMShellIsGone,
	} {
		wantCLines = append(wantCLines, opshell.CLine{
			Color: ErrColor,
			Line:  fmt.Sprintf("[%s] %s", tag, want),
		})
	}
	opshell.ExpectShellMessages(t, och, wantCLines...)

	/* Did it all go ok? */
	if err := eg.Wait(); nil != err {
		t.Fatalf("Error: %v", err)
	}

	/* Get our comms? */
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

	/* Can we shut down the broker? */
	<-done
	tb.Expect(t.Context(), t,
		m.
			With(LKDirection, LVInput).
			Info(LMConnectionClosed),
		m.
			With(LKDirection, LVOutput).
			Info(LMConnectionClosed),
		m.
			Info(LMShellFinished),
	)
	runtime.KeepAlive(ctx)
}

// Can we handle multiple shell connections?
func TestBroker_MultipleShells(t *testing.T) {
	/* Test broker. */
	b, ich, och, _, tb, sl := newTestBroker(t, t.Context(), nil)

	/* runShell hooks up a shell (homemade cat) to b, sends some data, and
	makes sure it made it back.  Plus logging/output checking. */
	runShell := func(t *testing.T) {
		var (
			eg, ctx     = ctxerrgroup.WithContext(t.Context())
			id          = tlog.S("id")
			inr, inw    = io.Pipe()
			outr, outw  = io.Pipe()
			shellPrefix = tlog.S("shell-prefix")
			tag         = tlog.S("tag")
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
			inr.Close()
			inw.Close()
			outr.Close()
			outw.Close()
		})

		/* Hook up a shell. */
		eg.GoTag(ctx, "input_stream", func(ctx context.Context) error {
			return b.HandleInput(ctx, sl, id, tag, inw)
		})
		opshell.ExpectShellMessages(t, och, opshell.CLine{
			Color: LogColor,
			Line: fmt.Sprintf(
				"[%s] Input connected: ID %s",
				tag,
				id,
			),
		})
		tb.Expect(t.Context(), t,
			m.
				With(LKDirection, LVInput).
				Info(LMNewConnection),
		)
		eg.GoTag(ctx, "output_stream", func(
			ctx context.Context,
		) error {
			return b.HandleOutput(ctx, sl, id, tag, outr)
		})
		opshell.ExpectShellMessages(t, och,
			opshell.CLine{
				Color: LogColor,
				Line: fmt.Sprintf(
					"[%s] Output connected: ID %s",
					tag,
					id,
				),
			},
			opshell.CLine{
				Color: LogColor,
				Line:  fmt.Sprintf("[%s] %s", tag, SMShellIsReady),
			},
		)
		tb.Expect(t.Context(), t,
			m.
				With(LKDirection, LVOutput).
				Info(LMNewConnection),
			m.
				Info(LMShellStarting),
		)
		if t.Failed() {
			t.FailNow()
		}

		/* Shell reads lines and writes them back with a prefix. */
		eg.GoTag(ctx, "shell", func(ctx context.Context) error {
			defer outw.Close()
			scanner := bufio.NewScanner(inr)
			for scanner.Scan() {
				l := scanner.Text()
				/* Analogous to telling the shell to exit. */
				if exitCmd == l {
					return nil
				}
				/* Not exit, send back the line. */
				if _, err := fmt.Fprintf(
					outw,
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

		/* Operator sends a bunch of lines and expects them back with
		the shell's prefix. */
		eg.GoTag(ctx, "operator_input", func(
			ctx context.Context,
		) error {
			for _, s := range opIn {
				ich <- s
			}
			return nil
		})

		/* Did we get shell output? */
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
			"Output connection closed",
			"Input connection closed",
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
			m.
				With(LKDirection, LVOutput).
				Info(LMConnectionClosed),
			m.
				With(LKDirection, LVInput).
				Info(LMConnectionClosed),
		)
		tb.WithExpectEmpty().Expect(t.Context(), t,
			m.
				Info(LMShellFinished),
		)

		/* Did it all go ok? */
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

// Do we stop brokering after a single shell with -one-shell?
func TestBroker_OneShellDisconnected(t *testing.T) {
	synctest.Test(t, testBrokerOneShellDisconnected)
}
func testBrokerOneShellDisconnected(t *testing.T) {
	var (
		id                      = tlog.S("id")
		tag                     = tlog.S("tag")
		pr, pw                  = io.Pipe()
		b, _, och, done, tb, sl = newTestBroker(
			t,
			t.Context(),
			&testBrokerConfig{
				oneShell: true,
				wantErr:  ErrOneShell,
			},
		)
	)
	/* Closing right away makes for a very fast shell. */
	defer pr.Close()
	pw.Close()

	/* Hook up and remove a shell.  Output should be enough. */
	if err := b.HandleOutput(t.Context(), sl, id, tag, pr); nil != err {
		t.Errorf("Unexpected error from HandleOutput: %v", err)
	}

	/* Should exit soon. */
	<-done

	/* Output correct? */
	addTag := func(s string) string {
		return taggedString(tag, "%s", s)
	}
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: LogColor,
		Line:  addTag("Output connected: ID " + id),
	},
		opshell.CLine{
			Color: ErrColor,
			Line:  addTag("Output connection closed"),
		},
	)

	/* Logging correct? */
	m := tlog.M.
		With(LKDirection, LVOutput).
		With(LKID, id)
	tb.Expect(t.Context(), t,
		m.
			Info(LMNewConnection),
		m.
			Info(LMConnectionClosed),
	)
}

// Do we return nil after the context is cancelled, even with -one-shell?
func TestBroker_OneShellContextDone(t *testing.T) {
	synctest.Test(t, testBrokerOneShellContextDone)
}
func testBrokerOneShellContextDone(t *testing.T) {
	/* Start a test broker we can stop when it's waiting on a shell. */
	var (
		ch          = make(chan struct{})
		hssStarted  = sync.OnceFunc(func() { close(ch) })
		ctx, cancel = context.WithCancel(t.Context())
	)
	defer cancel()
	newTestBroker(
		t,
		context.WithValue(
			ctx,
			handleSingleShellStartedKey{},
			hssStarted,
		),
		&testBrokerConfig{oneShell: true},
	)

	/* Wait until the broker's waiting on a shell, then tell it to stop.
	Cleanup will make sure the error is nil. */
	<-ch
	cancel()
}

// Are we ok if ich is closed?
func TestBroker_InputClosed(t *testing.T) {
	synctest.Test(t, testBrokerInputClosed)
}
func testBrokerInputClosed(t *testing.T) {
	_, ich, _, done, _, _ := newTestBroker(
		t,
		t.Context(),
		&testBrokerConfig{wantErr: ErrInputClosed},
	)
	close(ich)
	<-done
}

// Does safeClose close safely?
func TestSafeClose(t *testing.T) {
	for n, f := range map[string]func() chan int{
		"closed/empty": func() chan int {
			ch := make(chan int)
			close(ch)
			return ch
		},
		"closed/not_empty": func() chan int {
			ch := make(chan int, 10)
			for i := range len(ch) {
				ch <- i
			}
			close(ch)
			return ch
		},
		"not_closed/empty": func() chan int {
			return make(chan int)
		},
		"not_closed/not_empty": func() chan int {
			ch := make(chan int, 10)
			for i := range len(ch) {
				ch <- i
			}
			return ch
		},
	} {
		t.Run(n, func(t *testing.T) { safeClose(f()) })
	}
}

// safeClose drains and closes the channel ch.  If ch is already closed, it
// is drained but not closed again.  safeClose must not be called concurrently
// with any other access to ch.
func safeClose[T any](ch chan T) {
	for {
		select {
		case _, ok := <-ch:
			if !ok { /* Drained and closed */
				return
			}
			/* Draining. */
		default: /* Drained, not closed. */
			close(ch)
			return
		}
	}
}
