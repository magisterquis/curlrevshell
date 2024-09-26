package iobroker

/*
 * iobroker_test.go
 * Tests for iobroker.go
 * By J. Stuart McMurray
 * Created 20240925
 * Last Modified 20240926
 */

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sync"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/chanlog"
	"github.com/magisterquis/curlrevshell/lib/opshell"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func newTestBroker(t *testing.T) (*Broker, chan string, chan opshell.CLine) {
	var (
		ich = make(chan string, 1024)
		och = make(chan opshell.CLine, 1024)
		ech = make(chan error, 1)
	)
	iob, err := New(ich, och)
	if nil != err {
		t.Fatalf("Error making broker: %s", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { ech <- iob.Do(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-ech; nil != err {
			t.Fatalf("Error waiting for broker to finish: %s", err)
		}
	})
	return iob, ich, och
}

func TestBroker_Smoketest(t *testing.T) { newTestBroker(t) }

func TestBroker_Disconnect(t *testing.T) {
	type disconnectFunc func(
		*testing.T, /* t */
		io.Closer, /* inr */
		io.Closer, /* outw */
		func(), /* incancel */
		func(), /* outcancel */
	) []string /* closeLogs */
	testDisconnect := func(
		t *testing.T,
		disconnect disconnectFunc,
	) {
		var (
			iob, _, _         = newTestBroker(t)
			evCh              = make(chan Event, EVChanLen)
			inr, inw          = io.Pipe()
			outr, outw        = io.Pipe()
			ctx               = context.Background()
			inctx, incancel   = context.WithCancel(ctx)
			outctx, outcancel = context.WithCancel(ctx)
			cl, sl            = chanlog.New()
			addr              = "moose"
			addrIn            = addr + "_in"
			addrOut           = addr + "_out"
			key               = "kittens"
			wg                sync.WaitGroup
		)
		t.Cleanup(func() {
			incancel()
			outcancel()
			for _, c := range []io.Closer{inr, inw, outr, outw} {
				c.Close()
			}
		})

		/* Get events. */
		iob.AddEventListener(evCh)

		/* Hook up a shell. */
		wg.Add(2)
		go func() {
			defer wg.Done()
			iob.ConnectIn(inctx, sl, addrIn, inw, key)
		}()
		cl.Expect(
			t,
			`{"time":"","level":"INFO","msg":"New connection",`+
				`"direction":"input"}`,
		)
		//opshell.ExpectShellMessages(t, och, opshell.CLine{
		//	Color: logColor,
		//	Line: fmt.Sprintf(
		//		"[%s] Input connected: ID %q",
		//		addrIn,
		//		key,
		//	),
		//})
		go func() {
			defer wg.Done()
			go iob.ConnectOut(outctx, sl, addrOut, outr, key)
		}()
		cl.Expect(
			t,
			`{"time":"","level":"INFO","msg":"New connection",`+
				`"direction":"output"}`,
		)
		//opshell.ExpectShellMessages(t, och, opshell.CLine{
		//	Color: logColor,
		//	Line: fmt.Sprintf(
		//		"[%s] Output connected: ID %q",
		//		addrOut,
		//		key,
		//	),
		//})

		/* Make sure connecting worked. */
		wantEvent := Event{Type: EventTypeConnected}
		if got, ok := <-evCh; !ok {
			t.Fatalf("Event channel closed after shell connection")
		} else if got != wantEvent {
			t.Fatalf(
				"Incorrect event after shell connection:\n"+
					" got: %+v\n"+
					"want: %+v",
				got,
				wantEvent,
			)
		}
		//opshell.ExpectShellMessages(t, och, opshell.CLine{
		//	Color: logColor,
		//	Line: fmt.Sprintf(
		//		"[%s] %s",
		//		addrOut,
		//		ShellReadyMessage,
		//	),
		//})
		/* Test-specific disconnect. */
		closeLogs := disconnect(t, inr, outw, incancel, outcancel)

		/* Make sure both sides disconnect. */
		wg.Wait()

		/* Make sure it worked. */
		wantEvent = Event{Type: EventTypeDisconnected}
		if got, ok := <-evCh; !ok {
			t.Errorf("Event channel closed after shell connection")
		} else if got != wantEvent {
			t.Errorf(
				"Incorrect event after shell connection:\n"+
					" got: %+v\n"+
					"want: %+v",
				got,
				wantEvent,
			)
		}
		cl.ExpectEmpty(t, closeLogs...)

		/* We'll synthesize the shell lines from the log lines. */
		expectedMessages := make([]opshell.CLine, 0, 3)
		addClosed := func(logLine string) {
			var ls struct {
				Direction string
			}
			if err := json.Unmarshal(
				[]byte(logLine),
				&ls,
			); nil != err {
				t.Fatalf(
					"Error unJSONing %s: %s",
					logLine,
					err,
				)
			}
			cl := opshell.CLine{Color: errColor}
			var addr, dir string
			switch dir = ls.Direction; dir {
			case string(LVInput):
				addr = addrIn
			case string(LVOutput):
				addr = addrOut
			default:
				t.Fatalf("Unexpected direction %q", dir)
			}
			cl.Line = fmt.Sprintf(
				"[%s] %s connection closed",
				addr,
				cases.Title(language.English).String(dir),
			)
			expectedMessages = append(expectedMessages, cl)
		}
		for _, v := range closeLogs {
			addClosed(v)
		}
		gone := expectedMessages[len(expectedMessages)-1]
		gone.Line = regexp.MustCompile(`] .*`).ReplaceAllString(
			gone.Line,
			"] "+ShellDisconnectedMessage,
		)
		expectedMessages = append(expectedMessages, gone)

		//opshell.ExpectShellMessages(t, och, expectedMessages...)
	}

	/* Test ALL the disconnects. */
	cs := map[string]disconnectFunc{
		/* This would be nice, but the whole reason we have iobroker is
		because you can't test if an io.Writer is closed without
		writing to it. */
		//"close_in": func(
		//	t *testing.T,
		//	inr io.Closer,
		//	outw io.Closer,
		//	incancel func(),
		//	outcancel func(),
		//) {
		//	if err := inr.Close(); nil != err {
		//		t.Fatalf("Closing input pipe: %s", err)
		//	}
		//},
		"cancel_in": func(
			t *testing.T,
			inr io.Closer,
			outw io.Closer,
			incancel func(),
			outcancel func(),
		) []string {
			incancel()
			return []string{
				`{"time":"","level":"INFO",` +
					`"msg":"Disconnected",` +
					`"direction":"input"}`,
				`{"time":"","level":"INFO",` +
					`"msg":"Disconnected",` +
					`"direction":"output"}`,
			}
		},
		"close_out": func(
			t *testing.T,
			inr io.Closer,
			outw io.Closer,
			incancel func(),
			outcancel func(),
		) []string {
			if err := outw.Close(); nil != err {
				t.Fatalf("Closing input pipe: %s", err)
			}
			return []string{
				`{"time":"","level":"INFO",` +
					`"msg":"Disconnected",` +
					`"direction":"output"}`,
				`{"time":"","level":"INFO",` +
					`"msg":"Disconnected",` +
					`"direction":"input"}`,
			}
		},
		"cancel_out": func(
			t *testing.T,
			inr io.Closer,
			outw io.Closer,
			incancel func(),
			outcancel func(),
		) []string {
			outcancel()
			return []string{
				`{"time":"","level":"INFO",` +
					`"msg":"Disconnected",` +
					`"direction":"output"}`,
				`{"time":"","level":"INFO",` +
					`"msg":"Disconnected",` +
					`"direction":"input"}`,
			}
		},
	}
	for n, f := range cs {
		t.Run(n, func(t *testing.T) { testDisconnect(t, f) })
	}
}
