package hsrv

/*
 * handlers_websocket_test.go
 * Websocket tests for handlers.go
 * By J. Stuart McMurray
 * Created 20260803
 * Last Modified 20260804
 */

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/opshell"
	"golang.org/x/net/websocket"
)

// Can we hook up a websocket connection?
func TestServerWebsocketHandler(t *testing.T) {
	var (
		raCh        = make(chan string, 1)
		exitMsg     = tlog.S("exit")
		id          = t.Name()
		nMsgs       = 10
		shellSuffix = tlog.S("shell-suffix")
		wg          sync.WaitGroup

		tb, ich, och, _, c, s = newTestServer(
			context.WithValue(
				t.Context(),
				testWSRemoteAddrKey{},
				raCh,
			),
			t,
			&testServerConfig{noLogHI: true},
		)
		m = tlog.M.
			With(iobroker.LKID, id)
		mB = m.
			With(iobroker.LKDirection, iobroker.LVBidir)
	)

	/* Connect to the server. */
	u := fmt.Sprintf(
		"wss://%s/%s/%s",
		s.l.Addr(),
		s.params.URLPaths.Websocket,
		id,
	)
	wsConf, err := websocket.NewConfig(u, u)
	if nil != err {
		t.Errorf(
			"Error creating websocket config with URL %q: %v",
			u,
			err,
		)
	}
	wsConf.TlsConfig = c.Transport.(*http.Transport).TLSClientConfig
	ws, err := wsConf.DialContext(t.Context())
	if nil != err {
		t.Fatalf("Error making websocket connection: %v", err)
	}
	defer ws.Close()

	/* Wait until the shell connects. */
	req := &http.Request{RemoteAddr: <-raCh}
	opshell.ExpectShellMessages(t, och,
		testReqCLine(
			t,
			iobroker.LogColor,
			req,
			"Connected: ID %s",
			id,
		),
		testReqCLine(
			t,
			iobroker.LogColor,
			req,
			iobroker.SMShellIsReady,
		),
	)
	tb.Expect(t.Context(), t,

		mB.
			Info(iobroker.LMNewConnection),
		mB.
			Info(iobroker.LMShellStarting),
	)

	/* "Shell" just sends input back to output. */
	wg.Go(func() {
		/* Close the connection when we're done, analogous to a
		shell exiting. */
		defer ws.Close()
		/* Read input lines. */
		scanner := bufio.NewScanner(ws)
		for scanner.Scan() {
			l := scanner.Text()
			/* exitMsg is like exit to a shell. */
			if exitMsg == l {
				return
			}
			/* Just proxy the line back to the opshell. */
			if _, err := fmt.Fprintf(
				ws,
				"%s-%s",
				l,
				shellSuffix,
			); nil != err {
				t.Errorf("Error sending line %q: %v", l, err)
				return
			}
		}
		if err := scanner.Err(); nil != err {
			t.Errorf("Error reading shell input: %v", err)
			return
		}
	})

	/* Send some info through the shell. */
	for i := range nMsgs {
		var (
			msg  = tlog.S("msg-" + strconv.Itoa(i))
			oMsg = msg + "-" + shellSuffix
			iMsg = msg + "\n"
		)
		ich <- msg
		opshell.ExpectShellMessages(t, och, opshell.CLine{
			Line:  oMsg,
			Plain: true,
		})
		tb.WithExpectUnordered().Expect(t.Context(), t,
			m.
				With(iobroker.LKData, iMsg).
				With(
					iobroker.LKDirection,
					iobroker.LVInput,
				).
				Info(iobroker.LMShellIO),
			m.
				With(iobroker.LKData, oMsg).
				With(
					iobroker.LKDirection,
					iobroker.LVOutput,
				).
				Info(iobroker.LMShellIO),
		)
	}

	/* Tell the shell to exit. */
	ich <- exitMsg
	tb.Expect(t.Context(), t, m.
		With(iobroker.LKData, exitMsg+"\n").
		With(iobroker.LKDirection, iobroker.LVInput).
		Info(iobroker.LMShellIO),
	)

	/* Disconnect the request and wait for everything to settle. */
	wg.Wait()

	/* Logs correct? */
	opshell.ExpectShellMessages(t, och,
		testReqCLine(
			t,
			iobroker.ErrColor,
			req,
			iobroker.SMConnectionClosed,
		),
		testReqCLine(
			t,
			iobroker.ErrColor,
			req,
			iobroker.SMShellIsGone,
		),
	)
	tb.Expect(t.Context(), t,
		mB.
			Info(iobroker.LMConnectionClosed),
		mB.
			Info(iobroker.LMShellFinished),
	)
}

// Can we hook up a websocket connection without an origin?
func TestServerWebsocketHandler_NoOrigin(t *testing.T) {
	var (
		id   = t.Name()
		raCh = make(chan string, 1)

		tb, _, och, _, c, s = newTestServer(
			context.WithValue(
				t.Context(),
				testWSRemoteAddrKey{},
				raCh,
			),
			t,
			&testServerConfig{noLogHI: true},
		)
		m = tlog.M.
			With(iobroker.LKDirection, iobroker.LVBidir).
			With(iobroker.LKID, id)
	)

	/* Request with websocketish headers. */
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf(
			"https://%s/%s/%s",
			s.l.Addr(),
			s.params.URLPaths.Websocket,
			id,
		),
		nil,
	)
	if nil != err {
		t.Errorf("Error creating request: %v", err)
	}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Sec-Websocket-Key", "AAAAAAAAAAAAAAAAAAAAAA==")
	req.Header.Set("Sec-Websocket-Version", "13")

	/* Ask for an upgrade? */
	res, err := c.Do(req)
	if nil != err {
		t.Fatalf("Error requesting websocket connection: %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode,
		http.StatusSwitchingProtocols; got != want {
		t.Fatalf(
			"Unexpected HTTP status\n got: %s\nwant %d",
			res.Status,
			want,
		)
	}

	/* Get a shell? */
	req.RemoteAddr = <-raCh
	opshell.ExpectShellMessages(t, och,
		testReqCLine(
			t,
			iobroker.LogColor,
			req,
			"Connected: ID %s",
			id,
		),
		testReqCLine(
			t,
			iobroker.LogColor,
			req,
			iobroker.SMShellIsReady,
		),
	)
	tb.Expect(t.Context(), t,
		m.
			Info(iobroker.LMNewConnection),
		m.
			Info(iobroker.LMShellStarting),
	)

	/* Got a shell, now unget it. */
	res.Body.Close()
	opshell.ExpectShellMessages(t, och,
		testReqCLine(
			t,
			iobroker.ErrColor,
			req,
			iobroker.SMConnectionClosed,
		),
		testReqCLine(
			t,
			iobroker.ErrColor,
			req,
			iobroker.SMShellIsGone,
		),
	)
	tb.Expect(t.Context(), t,
		m.
			Info(iobroker.LMConnectionClosed),
		m.
			Info(iobroker.LMShellFinished),
	)
}
