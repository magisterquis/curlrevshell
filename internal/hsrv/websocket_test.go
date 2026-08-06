package hsrv

/*
 * websocket_test.go
 * Tests for websocket.go
 * By J. Stuart McMurray
 * Created 20260805
 * Last Modified 20260807
 */

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/opshell"
	"golang.org/x/net/websocket"
)

// testScriptIDExtractRE attempts to extract an ID from a /c script.
var testScriptIDExtractRE = regexp.MustCompile(`curl -sk .*/i/(\S+) `)

// testWebsocketDial connects to s on the given path and ID, using c for
// config.
func testWebsocketDial(
	ctx context.Context,
	t *testing.T,
	s *Server,
	c *http.Client,
	path string,
	id string,
) *websocket.Conn {
	/* URL to which we'll connect. */
	u := fmt.Sprintf("wss://%s/%s/%s", s.l.Addr(), path, id)
	/* Roll a config. */
	conf, err := websocket.NewConfig(u, u)
	if nil != err {
		t.Fatalf("Error rolling websocket config for %s: %v", u, err)
	}
	conf.TlsConfig = c.Transport.(*http.Transport).TLSClientConfig
	/* Actually make the connection. */
	ws, err := conf.DialContext(ctx)
	if nil != err {
		t.Fatalf("Error making websocket connection to %s: %v", u, err)
	}
	return ws
}

// Can we get shell input via a websocket?
func TestServerInputHandler_Websocket(t *testing.T) {
	var (
		id  = t.Name()
		msg = tlog.S("shell-input")
		buf = make([]byte, len(msg)+1)

		m = tlog.M.
			With(iobroker.LKDirection, iobroker.LVInput).
			With(iobroker.LKID, id)
		tb, ich, och, _, c, s = newTestServer(
			t.Context(),
			t,
			&testServerConfig{noLogHI: true},
		)
	)

	/* Connect to the server. */
	ws := testWebsocketDial(
		t.Context(),
		t,
		s,
		c,
		s.params.URLPaths.In,
		id,
	)
	opshell.ExpectShellMessages(t, och,
		testWSCLine(
			t,
			iobroker.LogColor,
			ws,
			"Input connected: ID %s", id,
		),
	)
	tb.Expect(t.Context(), t,
		tlog.M.
			Debug(LMUpgradedToWebsocket),
		m.
			Info(iobroker.LMNewConnection),
	)

	/* Send a message, see if we get it. */
	ich <- msg
	if _, err := io.ReadFull(ws, buf); nil != err {
		t.Errorf("Error reading from websocket: %v", err)
	}
	if got, want := string(buf), msg+"\n"; got != want {
		t.Errorf(
			"Shell got incorrect message\n got: %q\nwant: %q",
			got,
			want,
		)
	}
	tb.Expect(t.Context(), t,
		m.
			With(iobroker.LKData, msg+"\n").
			Info(iobroker.LMShellIO),
	)

	/* Close the connection. */
	if err := ws.Close(); nil != err {
		t.Errorf("Error closing websocket connection: %v", err)
	}
	opshell.ExpectShellMessages(t, och,
		testWSCLine(
			t,
			iobroker.ErrColor,
			ws,
			"Input connection closed",
		),
	)
	tb.Expect(t.Context(), t,
		m.
			Info(iobroker.LMConnectionClosed),
	)
}

// Can we get shell output via a websocket?
func TestServerOutputHandler_Websocket(t *testing.T) {
	var (
		id  = t.Name()
		msg = tlog.S("shell-output")

		m = tlog.M.
			With(iobroker.LKDirection, iobroker.LVOutput).
			With(iobroker.LKID, id)
		tb, _, och, _, c, s = newTestServer(
			t.Context(),
			t,
			&testServerConfig{noLogHI: true},
		)
	)

	/* Connect to the server. */
	ws := testWebsocketDial(
		t.Context(),
		t,
		s,
		c,
		s.params.URLPaths.Out,
		id,
	)
	opshell.ExpectShellMessages(t, och,
		testWSCLine(
			t,
			iobroker.LogColor,
			ws,
			"Output connected: ID %s", id,
		),
	)
	tb.Expect(t.Context(), t,
		tlog.M.
			Debug(LMUpgradedToWebsocket),
		m.
			Info(iobroker.LMNewConnection),
	)

	/* Send some output, see if we get it. */
	if _, err := io.WriteString(ws, msg); nil != err {
		t.Fatalf("Error sending output via websocket: %v", err)
	}
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Line:  msg,
		Plain: true,
	})
	tb.Expect(t.Context(), t,
		m.
			With(iobroker.LKData, msg).
			Info(iobroker.LMShellIO),
	)

	/* Close the connection. */
	if err := ws.Close(); nil != err {
		t.Errorf("Error closing websocket connection: %v", err)
	}
	opshell.ExpectShellMessages(t, och,
		testWSCLine(
			t,
			iobroker.ErrColor,
			ws,
			"Output connection closed",
		),
	)
	tb.Expect(t.Context(), t,
		m.
			Info(iobroker.LMConnectionClosed),
	)
}

// Can we send shell I/O both directions with a websocket?
func TestServerInOutHandler_Websocket(t *testing.T) {
	var (
		id     = t.Name()
		msgIn  = tlog.S("shell-input")
		msgOut = tlog.S("shell-output")

		buf                   = make([]byte, len(msgIn)+1)
		tb, ich, och, _, c, s = newTestServer(
			t.Context(),
			t,
			&testServerConfig{noLogHI: true},
		)

		m  = tlog.M.With(iobroker.LKID, id)
		mB = m.With(iobroker.LKDirection, iobroker.LVBidir)
	)

	/* Connect to the server. */
	ws := testWebsocketDial(
		t.Context(),
		t,
		s,
		c,
		s.params.URLPaths.InOut,
		id,
	)
	opshell.ExpectShellMessages(t, och,
		testWSCLine(
			t,
			iobroker.LogColor,
			ws,
			"Connected: ID %s", id,
		),
		testWSCLine(
			t,
			iobroker.LogColor,
			ws,
			iobroker.SMShellIsReady,
		),
	)
	tb.Expect(t.Context(), t,
		tlog.M.
			Debug(LMUpgradedToWebsocket),
		mB.
			Info(iobroker.LMNewConnection),
		mB.
			Info(iobroker.LMShellStarting),
	)

	/* Send a message, see if we get it. */
	ich <- msgIn
	if _, err := io.ReadFull(ws, buf); nil != err {
		t.Errorf("Error reading from websocket: %v", err)
	}
	if got, want := string(buf), msgIn+"\n"; got != want {
		t.Errorf(
			"Incorrect shell input message\n got: %q\nwant: %q",
			got,
			want,
		)
	}
	tb.Expect(t.Context(), t,
		m.
			With(iobroker.LKData, msgIn+"\n").
			With(iobroker.LKDirection, iobroker.LVInput).
			Info(iobroker.LMShellIO),
	)

	/* Send some output, see if we get it. */
	if _, err := io.WriteString(ws, msgOut); nil != err {
		t.Fatalf("Error sending output via websocket: %v", err)
	}
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Line:  msgOut,
		Plain: true,
	})
	tb.Expect(t.Context(), t,
		m.
			With(iobroker.LKData, msgOut).
			With(iobroker.LKDirection, iobroker.LVOutput).
			Info(iobroker.LMShellIO),
	)

	/* Close the connection. */
	if err := ws.Close(); nil != err {
		t.Errorf("Error closing websocket connection: %v", err)
	}
	opshell.ExpectShellMessages(t, och,
		testWSCLine(
			t,
			iobroker.ErrColor,
			ws,
			iobroker.SMConnectionClosed,
		),
		testWSCLine(
			t,
			iobroker.ErrColor,
			ws,
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

// Can we get a callback script with a websocket?
func TestServerScriptHandler_Websocket(t *testing.T) {
	var (
		tb, _, och, _, c, s = newTestServer(
			t.Context(),
			t,
			&testServerConfig{noLogHI: true},
		)
	)

	/* Connect to the server and read the script. */
	ws := testWebsocketDial(
		t.Context(),
		t,
		s,
		c,
		s.params.URLPaths.Script,
		"",
	)
	b, err := io.ReadAll(ws)
	if nil != err {
		t.Fatalf("Error reading script: %v", err)
	}
	/* Extract our ID from the script, which also validates it at least
	looks kinda scriptish. */
	ms := testScriptIDExtractRE.FindSubmatch(b)
	if 2 != len(ms) {
		t.Fatalf(
			"Could not extract ID from script\n"+
				" regex: %s\n"+
				"script:\n%s",
			testScriptIDExtractRE,
			b,
		)
	}
	/* C2 address is our websocket's remote address. */
	ra := ws.RemoteAddr().String()
	u, err := url.Parse(ra)
	if nil != err {
		t.Fatalf("Error parsing URL %s: %v", ra, err)
	}
	opshell.ExpectShellMessages(t, och,
		testWSCLine(
			t,
			scriptColor,
			ws,
			"Sent script: ID:%s C2Addr:%s Path:%s",
			ms[1],
			u.Host,
			"/"+s.params.URLPaths.Script+"/",
		),
	)
	tb.Expect(t.Context(), t,
		tlog.M.
			Debug(LMUpgradedToWebsocket),
	)
}

// testWSCLine returns a CLine tagged with the IP in ws's RemoteAddr.
func testWSCLine(
	t *testing.T,
	color opshell.Color,
	ws *websocket.Conn, /* Tag from RemoteAddr. */
	f string, a ...any, /* Rest of the line. */
) opshell.CLine {
	t.Helper()
	/* Extract the address. */
	ra := ws.RemoteAddr().String()
	u, err := url.Parse(ra)
	if nil != err {
		t.Fatalf("Error parsing %s: %v", ra, err)
	}
	/* Turn into a CLine. */
	return opshell.CLine{
		Color: color,
		Line: fmt.Sprintf(
			"[%s] %s",
			u.Hostname(),
			fmt.Sprintf(f, a...),
		),
	}
}
