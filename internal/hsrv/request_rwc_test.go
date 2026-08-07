package hsrv

/*
 * request_rwc_test.go
 * Tests for request_rwc.go
 * By J. Stuart McMurray
 * Created 20260730
 * Last Modified 20260807
 */

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// Can we wrap both sides of an HTTP handler into a single io.ReadWriteCloser?
func TestRequestRWC(t *testing.T) {
	var (
		c2sMsg = tlog.S("c2s")
		pr, pw = io.Pipe() /* Client side. */
		s2cMsg = tlog.S("s2c")
		done   = make(chan struct{})
	)
	defer pw.Close()

	/* Handler to send and receive on the rwc. */
	handler := func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		/* Upgrade to rwc. */
		defer r.Body.Close()
		if err := StartFullDuplex(w, r); nil != err {
			t.Errorf("Error starting duplex comms: %v", err)
		}
		rwc := newRequestRWC(w, r)
		/* Send and receive. */
		eg, ctx := ctxerrgroup.WithContext(r.Context())
		eg.GoTag(ctx, "client->server read", func(
			ctx context.Context,
		) error {
			buf := make([]byte, len(c2sMsg))
			n, err := io.ReadFull(rwc, buf)
			if nil != err {
				t.Errorf(
					"Error reading message from "+
						"client: %v",
					err,
				)
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
		eg.GoTag(ctx, "server->client write", func(
			ctx context.Context,
		) error {
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
		if err := eg.Wait(); nil != err {
			t.Errorf("Handler error: %v", err)
		}
	}

	/* Server which sends and receives via a requestRWC. */
	svr := httptest.NewServer(http.HandlerFunc(handler))
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

	/* Send and receive. */
	eg, ctx := ctxerrgroup.WithContext(t.Context())
	eg.GoTag(ctx, "client->server write", func(ctx context.Context) error {
		_, err := pw.Write([]byte(c2sMsg))
		if nil != err {
			t.Errorf("Error sending message to server: %v", err)
		}
		return err
	})
	eg.GoTag(ctx, "server->client read", func(ctx context.Context) error {
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

	/* Wait for the handler to exit. */
	pw.Close()
	<-done
}

// Can we close without blocking if the client's expected a 100?
func TestRequestRWCClose_Expect100Continue(t *testing.T) {
	synctest.Test(t, testRequestRWCCloseExpect100Continue)
}
func testRequestRWCCloseExpect100Continue(t *testing.T) {
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
