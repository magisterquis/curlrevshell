package hsrv

/*
 * request_rwc_test.go
 * Tests for request_rwc.go
 * By J. Stuart McMurray
 * Created 20260730
 * Last Modified 20260801
 */

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"sync"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// Can we wrap both sides of an HTTP handler into a single io.ReadWriteCloser?
func TestRequestRWC(t *testing.T) {
	var (
		c2sMsg      = tlog.S("c2s")
		ctx, cancel = context.WithCancel(t.Context())
		hDone       = make(chan struct{})   /* Handler's exiting. */
		pr, pw      = io.Pipe()             /* Client side. */
		rwcCh       = make(chan requestRWC) /* From handler. */
		s2cMsg      = tlog.S("s2c")
	)
	defer cancel()
	defer pr.Close()
	defer pw.Close()

	/* Server which accepts a connection and sends it back as a
	requestRWC. */
	handler := func(w http.ResponseWriter, r *http.Request) {
		defer close(hDone)
		if err := StartFullDuplex(w, r); nil != err {
			t.Errorf("Error starting duplex comms: %v", err)
		}
		rwcCh <- newRequestRWC(w, r)
		<-rwcCh
	}
	svr := httptest.NewUnstartedServer(http.HandlerFunc(handler))
	svr.Config.BaseContext = func(net.Listener) context.Context {
		return ctx
	}
	svr.Start()
	defer svr.Close()

	/* Connect(ish) to the server. */
	res, err := svr.Client().Post(svr.URL, "", pr)
	if nil != err {
		t.Fatalf("POST error: %v", err)
	}
	defer res.Body.Close()
	if http.StatusOK != res.StatusCode {
		t.Fatalf("Non-OK response status: %s", res.Status)
	}

	/* Get the server side of the connection. */
	rwc := <-rwcCh

	/* Send and receive. */
	eg, ectx := ctxerrgroup.WithContext(t.Context())
	eg.GoTag(ectx, "client->server write", func(ctx context.Context) error {
		_, err := pw.Write([]byte(c2sMsg))
		if nil != err {
			t.Errorf("Error sending message to server: %v", err)
		}
		return err
	})
	eg.GoTag(ectx, "client->server read", func(ctx context.Context) error {
		buf := make([]byte, len(c2sMsg))
		n, err := io.ReadFull(rwc, buf)
		if nil != err {
			t.Errorf("Error reading message from client: %v", err)
		}
		if got, want := string(buf[:n]), c2sMsg; got != want {
			t.Errorf(
				"Incorrect message from client\n"+
					" got: %q\n"+
					"want: %q",
				got,
				want,
			)
		}
		return err
	})
	eg.GoTag(ectx, "server->client write", func(ctx context.Context) error {
		if _, err := rwc.Write([]byte(s2cMsg)); nil != err {
			t.Errorf("Error sending message to client: %v", err)
			return err
		}
		if err := rwc.Flush(); nil != err {
			t.Errorf("Flush error: %v", err)
			return err
		}
		return nil
	})
	eg.GoTag(ectx, "server->client read", func(ctx context.Context) error {
		buf := make([]byte, len(s2cMsg))
		n, err := io.ReadFull(res.Body, buf)
		if nil != err {
			t.Errorf("Error reading message from client: %v", err)
		}
		if got, want := string(buf[:n]), s2cMsg; got != want {
			t.Errorf(
				"Incorrect message from client\n"+
					" got: %q\n"+
					"want: %q",
				got,
				want,
			)
		}
		return err
	})
	if err := eg.Wait(); nil != err {
		t.Fatalf("Error: %v", err)
	}

	/* One-sided Close methods are all no-ops, but for just in case. */
	if err := rwc.CloseRead(); nil != err {
		t.Errorf("CloseRead returned error: %v", err)
	}
	if err := rwc.CloseWrite(); nil != err {
		t.Errorf("CloseWrite returned error: %v", err)
	}

	/* Close should close the handler, ideally. */
	if err := rwc.Close(); nil != err {
		t.Errorf("Closer eturned error: %v", err)
	}
	/* Send it back to end the handler. */
	rwcCh <- rwc
	if _, err := io.Copy(io.Discard, res.Body); nil != err {
		t.Errorf("Unexpected error waiting for body to close: %v", err)
	}
	<-hDone /* Handler should also end. */

}

// Can we close without blocking if the client's expected a 100?
func TestRequestRWCClose_Expect100Continue(t *testing.T) {
	var (
		ctx, cancel    = context.WithCancel(t.Context())
		got100Continue bool
		id             = tlog.S("id")
		inB            = new(bytes.Buffer)
		pr, pw         = io.Pipe()
		shellInput     = tlog.S("input")
		wg             sync.WaitGroup

		tb, ich, och, done, c, s = newTestServer(
			ctx,
			t,
			&testServerConfig{noLogHI: true},
		)

		m  = tlog.M.With(iobroker.LKID, id)
		mB = m.With(iobroker.LKDirection, iobroker.LVBidir)
	)
	defer pr.Close()
	defer pw.Close()

	/* Request that expects a 100-continue. */
	addrCh := make(chan string, 1)
	req, err := http.NewRequestWithContext(
		httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
			GotConn: func(ci httptrace.GotConnInfo) {
				addrCh <- ci.Conn.LocalAddr().String()
			},
			Got100Continue: func() {
				got100Continue = true
			},
		}),
		http.MethodPut,
		"https://"+s.l.Addr().String()+
			"/"+s.params.URLPaths.InOut+"/"+id,
		pr,
	)
	if nil != err {
		t.Fatalf("Error creating HTTP request: %v", err)
	}
	req.Header.Set("Expect", "100-continue")

	/* Send request. */
	res, err := c.Do(req)
	if nil != err {
		t.Fatalf("Error making HTTP request: %v", err)
	}
	defer res.Body.Close()

	/* Add the RemoteAddr (really, local address) in the request for
	checking logs. */
	req.RemoteAddr = <-addrCh

	/* Shell connect ok? */
	opshell.ExpectShellMessages(t, och,
		testReqCLine(
			t,
			iobroker.LogColor,
			req,
			"Connected: ID %s", id,
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

	/* Grab shell input from the shell response. */
	wg.Go(func() {
		if _, err := io.Copy(inB, res.Body); nil != err {
			t.Errorf("Error reading HTTP response: %v", err)
		}
	})

	/* Send some data, to make sure we're connected. */
	ich <- shellInput
	tb.Expect(t.Context(), t,
		m.
			With(iobroker.LKDirection, iobroker.LVInput).
			With(iobroker.LKData, shellInput+"\n").
			Info(iobroker.LMShellIO),
	)

	/* Did we actually get a 100 Continue? */
	if !got100Continue {
		t.Errorf("Did not get 100 Continue")
	}

	/* All done. */
	cancel()
	<-done
	wg.Wait()

	/* Did we get the right body? */
	if got, want := inB.String(), shellInput+"\n"; got != want {
		t.Errorf(
			"Shell input incorrect\n got: %q\nwant: %q",
			got,
			want,
		)
	}

	/* Disconnect happily? */
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

// In which we test a no-op.
func TestRequestRWCClose_NoCloser(t *testing.T) {
	if err := (requestRWC{}).Close(); nil != err {
		t.Errorf("Unexpected error: %v", err)
	}
}
