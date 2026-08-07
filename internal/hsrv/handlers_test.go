package hsrv

/*
 * handlers_test.go
 * Tests for handlers.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20260807
 */

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// Can we generate a mux without blowing up?
func TestServerNewMux_Smoketest(t *testing.T) {
	_, _, _, _, _, s := newTestServer(t.Context(), t, nil)
	s.newMux()
}

// Can we serve static files from a directory?
func TestServerFileHandler_Dir(t *testing.T) {
	var (
		data                = tlog.S("data")
		fn                  = tlog.S("fn")
		index               = "index.html"
		indexData           = tlog.S("index")
		tb, _, och, _, c, s = newTestServer(
			t.Context(),
			t,
			&testServerConfig{
				makeFDir: true,
				noLogHI:  true,
			},
		)

		ffn = filepath.Join(s.params.StaticFilesDir, fn)
		ifn = filepath.Join(s.params.StaticFilesDir, "index.html")
		m   = tlog.M.With(LKStaticFilesDir, s.params.StaticFilesDir)
	)

	/* Make a file to serve. */
	if err := os.WriteFile(ffn, []byte(data), 0600); nil != err {
		t.Fatalf("Error writing %s: %s", ffn, err)
	}

	/* get checks the path p on the server.  It calls t.Fatalf on error. */
	try := func(
		t *testing.T,
		p string, /* Path to request. */
		wantStatus int, /* HTTP status code we expect. */
		wantBody string, /* Response body we expect. */
		wantCLine opshell.CLine,
		wantLog tlog.Msg,
	) {
		/* Make sure path starts with a /. */
		p = "/" + strings.TrimLeft(p, "/")
		/* Try to get the path. */
		res, err := c.Get("https://" + s.l.Addr().String() + p)
		if nil != err {
			t.Fatalf("Error GETting %s: %v", p, err)
		}
		defer res.Body.Close()
		/* Status ok? */
		if got, want := res.StatusCode, wantStatus; got != want {
			t.Errorf(
				"Incorrect status\n got: %d (%s)\nwant: %d",
				got, res.Status,
				want,
			)
		}
		/* Get the body. */
		b, err := io.ReadAll(res.Body)
		if nil != err {
			t.Fatalf("Error reading response: %v", err)
		}
		/* Is it correct? */
		if got, want := string(b), wantBody; got != want {
			t.Errorf(
				"Incorrect body\ngot:\n%s\nwant:\n%s",
				got,
				want,
			)
		}
		/* Output correct? */
		opshell.ExpectShellMessages(t, och, wantCLine)
		/* Log correct? */
		if !wantLog.IsZero() {
			tb.Expect(t.Context(), t, wantLog)
		}
	}

	/* Can we get the file? */
	t.Run("get file", func(t *testing.T) {
		try(
			t,
			fn,
			http.StatusOK,
			data,
			opshell.CLine{
				Color: fileColor,
				Line:  "[127.0.0.1] File requested: /" + fn,
			},
			m.
				With(LKRequestedFile, "/"+fn).
				Info(LMFileRequested),
		)
	})

	/* Do we get a 404 for file that doesn't exist. */
	t.Run("nonexistent file", func(t *testing.T) {
		nfn := tlog.S("nope")
		try(t,
			nfn,
			http.StatusNotFound,
			"404 page not found\n",
			opshell.CLine{
				Color: fileColor,
				Line:  "[127.0.0.1] File requested: /" + nfn,
			},
			m.
				With(LKRequestedFile, "/"+nfn).
				Info(LMFileRequested),
		)
	})

	/* Do we get a directory listing? */
	t.Run("directory listing", func(t *testing.T) {
		try(t,
			"/",
			http.StatusOK,
			`<!doctype html>
<meta name="viewport" content="width=device-width">
<pre>
<a href="`+fn+`">`+fn+`</a>
</pre>`+"\n",
			opshell.CLine{
				Color: fileColor,
				Line:  "[127.0.0.1] File requested: /",
			},
			m.
				With(LKRequestedFile, "/").
				Info(LMFileRequested),
		)
	})

	/* Does an index.html hide /? */
	t.Run("index.html/file", func(t *testing.T) {
		if err := os.WriteFile(
			ifn,
			[]byte(indexData),
			0600,
		); nil != err {
			t.Fatalf("Error writing %s: %v", index, err)
		}
		try(t,
			"/",
			http.StatusOK,
			indexData,
			opshell.CLine{
				Color: fileColor,
				Line:  "[127.0.0.1] File requested: /",
			},
			m.
				With(LKRequestedFile, "/").
				Info(LMFileRequested),
		)
		if err := os.Remove(ifn); nil != err {
			t.Errorf("Error removing %s: %v", index, err)
		}
	})

	/* What if the directory doesn't exist? */
	t.Run("missing_dir", func(t *testing.T) {
		origSFD := s.params.StaticFilesDir
		s.params.StaticFilesDir = filepath.Join(
			s.params.StaticFilesDir,
			tlog.S("nope"),
		)
		try(t,
			"/",
			http.StatusInternalServerError,
			"\n",
			opshell.CLine{
				Color: fileColor,
				Line:  "[127.0.0.1] File requested: /",
			},
			tlog.Msg{},
		)
		opshell.ExpectShellMessages(t, och, opshell.CLine{
			Color: errorColor,
			Line: fmt.Sprintf(
				"[127.0.0.1] Could not open %s: %v",
				s.params.StaticFilesDir,
				&os.PathError{
					Op:   "open",
					Path: s.params.StaticFilesDir,
					Err:  syscall.ENOENT,
				},
			),
		})
		s.params.StaticFilesDir = origSFD
	})
}

// Can we serve a single static file?
func TestServerFileHandler_SingleFile(t *testing.T) {
	var (
		data                = tlog.S("data")
		fn                  = filepath.Join(t.TempDir(), "fn")
		tb, _, och, _, c, s = newTestServer(
			t.Context(),
			t,
			&testServerConfig{
				makeFDir: true,
				noLogHI:  true,
			},
		)

		m = tlog.M.With(LKStaticFilesDir, fn)
	)

	/* Serve a single file. */
	s.params.StaticFilesDir = fn
	if err := os.WriteFile(fn, []byte(data), 0600); nil != err {
		t.Fatalf("Error writing file: %v", err)
	}

	/* get checks the path p on the server.  It calls t.Fatalf on error. */
	try := func(
		t *testing.T,
		p string, /* Path to request. */
	) {
		/* Try to get the path. */
		res, err := c.Get("https://" + s.l.Addr().String() + p)
		if nil != err {
			t.Fatalf("Error GETting %q: %v", p, err)
		}
		defer res.Body.Close()
		/* Status ok? */
		if got, want := res.StatusCode, http.StatusOK; got != want {
			t.Errorf(
				"Incorrect status\n got: %d (%s)\nwant: %d",
				got, res.Status,
				want,
			)
		}
		/* Get the body. */
		b, err := io.ReadAll(res.Body)
		if nil != err {
			t.Fatalf("Error reading response: %v", err)
		}
		/* Is it correct? */
		if got, want := string(b), data; got != want {
			t.Errorf(
				"Incorrect body\ngot:\n%s\nwant:\n%s",
				got,
				want,
			)
		}
		/* Output correct? */
		if "" == p {
			p = "/"
		}
		opshell.ExpectShellMessages(t, och, opshell.CLine{
			Color: fileColor,
			Line:  "[127.0.0.1] File requested: " + p,
		})
		/* Log correct? */
		tb.Expect(t.Context(), t,
			m.
				With(LKRequestedFile, p).
				Info(LMFileRequested),
		)
	}

	/* No matter what we request, we should get more or less the same
	output. */
	for n, c := range map[string]string{
		"empty_path": "",
		"filename":   "/filename",
		"index.html": "/index.html",
		"slash":      "/",
	} {
		t.Run(n, func(t *testing.T) { try(t, c) })
	}
}

// Do we handle not being able to stat a file properly?
func TestServerFileHandler_StatError(t *testing.T) {
	var (
		_, _, och, _, c, s = newTestServer(
			context.WithValue(
				t.Context(),
				testCloseFileBeforeStatKey{},
				true,
			),
			t,
			&testServerConfig{
				makeFDir: true,
				noLogHI:  true,
			},
		)
	)

	/* Try to get the path. */
	res, err := c.Get("https://" + s.l.Addr().String())
	if nil != err {
		t.Fatalf("GET error: %v", err)
	}
	defer res.Body.Close()
	/* Status ok? */
	if got, want := res.StatusCode,
		http.StatusInternalServerError; got != want {
		t.Errorf(
			"Incorrect status\n got: %d (%s)\nwant: %d",
			got, res.Status,
			want,
		)
	}
	/* Get the body. */
	b, err := io.ReadAll(res.Body)
	if nil != err {
		t.Fatalf("Error reading response: %v", err)
	}
	/* Is it correct? */
	if got, want := string(b), "\n"; got != want {
		t.Errorf(
			"Incorrect body\ngot:\n%s\nwant:\n%s",
			got,
			want,
		)
	}
	/* Output correct? */
	opshell.ExpectShellMessages(t, och,
		opshell.CLine{
			Color: fileColor,
			Line:  "[127.0.0.1] File requested: /",
		},
		opshell.CLine{
			Color: errorColor,
			Line: fmt.Sprintf(
				"[127.0.0.1] Could not get info about %s: %s",
				s.params.StaticFilesDir,
				&os.PathError{
					Op:   "stat",
					Path: s.params.StaticFilesDir,
					Err:  os.ErrClosed,
				},
			),
		},
	)
}

/* Make sure closing stdin stops the handler. */
func TestServerInputHandler_CloseStdin(t *testing.T) {
	var (
		buf                      = new(bytes.Buffer)
		id                       = t.Name()
		msgs                     = make([]string, 10)
		rr                       = httptest.NewRecorder()
		wg                       sync.WaitGroup
		wantLogs                 []tlog.Msg
		tb, ich, och, done, _, s = newTestServer(
			t.Context(),
			t,
			&testServerConfig{
				noLogHI: true,
				wantErr: iobroker.ErrInputClosed,
			},
		)

		req = httptest.NewRequestWithContext(
			testCtxWithNoLogHI(t),
			http.MethodGet,
			"/"+s.params.URLPaths.In+"/"+id,
			nil,
		)
		m = tlog.M.
			With(iobroker.LKDirection, iobroker.LVInput).
			With(iobroker.LKID, id)
	)

	/* Connect to the input side. */
	rr.Body = buf
	req.SetPathValue(idParam, id)
	wg.Go(func() { s.inputHandler(rr, req) })
	opshell.ExpectShellMessages(t, och,
		testReqCLine(
			t,
			connectedColor,
			req,
			"Input connected: ID %s",
			id,
		),
	)
	tb.Expect(t.Context(), t,
		m.
			Info(iobroker.LMNewConnection),
	)

	/* Send some input to make sure the handler's started. */
	for i := range msgs {
		msgs[i] = tlog.S("msg-" + strconv.Itoa(i))
		wantLogs = append(wantLogs, m.
			With(iobroker.LKData, msgs[i]+"\n").
			Info(iobroker.LMShellIO),
		)
		ich <- msgs[i]
	}
	tb.Expect(t.Context(), t, wantLogs...)

	/* Shell input get there? */
	if got, want := buf.String(),
		strings.Join(msgs, "\n")+"\n"; got != want {
		t.Errorf(
			"Incorrect shell input\n got: %q\nwant: %q",
			got,
			want,
		)
	}

	/* Close the input and wait for everything to settle. */
	close(ich)
	wg.Wait()
	<-done

	/* Logs correct? */
	opshell.ExpectShellMessages(t, och,
		testReqCLine(t, errorColor, req, "Input connection closed"),
	)
	tb.Expect(t.Context(), t,
		m.
			Info(iobroker.LMConnectionClosed),
	)
}

// Can we handle input connecting and then disconnecting?
func TestServerInputHandler_Disconnect(t *testing.T) {
	var (
		buf         = new(bytes.Buffer)
		id          = t.Name()
		msgs        = make([]string, 10)
		rr          = httptest.NewRecorder()
		wantLogs    []tlog.Msg
		wg          sync.WaitGroup
		ctx, cancel = context.WithCancel(
			testCtxWithNoLogHI(t),
		)
		tb, ich, och, _, _, s = newTestServer(
			t.Context(),
			t,
			&testServerConfig{noLogHI: true},
		)

		req = httptest.NewRequestWithContext(
			ctx,
			http.MethodGet,
			"/"+s.params.URLPaths.In+"/"+id,
			nil,
		)
		m = tlog.M.
			With(iobroker.LKDirection, iobroker.LVInput).
			With(iobroker.LKID, id)
	)
	defer cancel()

	/* Connect to the input side. */
	rr.Body = buf
	req.SetPathValue(idParam, id)
	wg.Go(func() { s.inputHandler(rr, req) })
	opshell.ExpectShellMessages(t, och,
		testReqCLine(
			t,
			connectedColor,
			req,
			"Input connected: ID %s",
			id,
		),
	)
	tb.Expect(t.Context(), t,
		m.
			Info(iobroker.LMNewConnection),
	)

	/* Send some input to make sure the handler's started. */
	for i := range msgs {
		msgs[i] = tlog.S("msg-" + strconv.Itoa(i))
		wantLogs = append(wantLogs, m.
			With(iobroker.LKData, msgs[i]+"\n").
			Info(iobroker.LMShellIO),
		)
		ich <- msgs[i]
	}
	tb.Expect(t.Context(), t, wantLogs...)

	/* Shell input get there? */
	if got, want := buf.String(),
		strings.Join(msgs, "\n")+"\n"; got != want {
		t.Errorf(
			"Incorrect shell input\n got: %q\nwant: %q",
			got,
			want,
		)
	}

	/* Disconnect the request and wait for everything to settle. */
	cancel()
	wg.Wait()

	/* Logs correct? */
	opshell.ExpectShellMessages(t, och,
		testReqCLine(t, errorColor, req, "Input connection closed"),
	)
	tb.Expect(t.Context(), t,
		m.
			Info(iobroker.LMConnectionClosed),
	)
}

// Can we get shell output?
func TestServerOutputHandler(t *testing.T) {
	var (
		ctx, cancel         = context.WithCancel(testCtxWithNoLogHI(t))
		id                  = t.Name()
		msgs                = make([]string, 10)
		pr, pw              = io.Pipe()
		rr                  = httptest.NewRecorder()
		wantCLines          []opshell.CLine
		wantLogs            []tlog.Msg
		wg                  sync.WaitGroup
		tb, _, och, _, _, s = newTestServer(
			t.Context(),
			t,
			&testServerConfig{noLogHI: true},
		)

		req = httptest.NewRequestWithContext(
			ctx,
			http.MethodPut,
			"/"+s.params.URLPaths.Out+"/"+id,
			pr,
		)
		m = tlog.M.
			With(iobroker.LKDirection, iobroker.LVOutput).
			With(iobroker.LKID, id)
	)
	defer cancel()
	defer pr.Close()
	defer pw.Close()

	/* Connect to the output side. */
	req.SetPathValue(idParam, id)
	wg.Go(func() { s.outputHandler(rr, req) })
	opshell.ExpectShellMessages(t, och,
		testReqCLine(
			t,
			connectedColor,
			req,
			"Output connected: ID %s",
			id,
		),
	)
	tb.Expect(t.Context(), t,
		m.
			Info(iobroker.LMNewConnection),
	)

	/* Send some output to make sure the handler's started. */
	for i := range msgs {
		msgs[i] = tlog.S("msg-" + strconv.Itoa(i))
		wantLogs = append(wantLogs, m.
			With(iobroker.LKData, msgs[i]).
			Info(iobroker.LMShellIO),
		)
		wantCLines = append(wantCLines, opshell.CLine{
			Line:  msgs[i],
			Plain: true,
		})
		if _, err := io.WriteString(pw, msgs[i]); nil != err {
			t.Fatalf("Error sending output line: %v", err)
		}
	}

	/* Did it get there? */
	tb.Expect(t.Context(), t, wantLogs...)
	opshell.ExpectShellMessages(t, och, wantCLines...)

	/* Disconnect the request and wait for everything to settle. */
	cancel()
	wg.Wait()

	/* Logs correct? */
	opshell.ExpectShellMessages(t, och,
		testReqCLine(t, errorColor, req, "Output connection closed"),
	)
	tb.Expect(t.Context(), t,
		m.
			Info(iobroker.LMConnectionClosed),
	)
}

// Can we hook up to both input and output simultaneously?
func TestServerInOutHandler(t *testing.T) {
	var (
		ctx, cancel           = context.WithCancel(t.Context())
		exitMsg               = tlog.S("exit")
		id                    = t.Name()
		nMsgs                 = 10
		pr, pw                = io.Pipe()
		shellSuffix           = tlog.S("shell-suffix")
		wg                    sync.WaitGroup
		tb, ich, och, _, c, s = newTestServer(
			t.Context(),
			t,
			&testServerConfig{noLogHI: true},
		)

		m = tlog.M.
			With(iobroker.LKID, id)
		mB = m.
			With(iobroker.LKDirection, iobroker.LVBidir)
	)
	defer cancel()
	defer pr.Close()
	defer pw.Close()

	/* Connect to the server. */
	addrCh := make(chan string, 1)
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GotConn: func(ci httptrace.GotConnInfo) {
			addrCh <- ci.Conn.LocalAddr().String()
		},
	})
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		fmt.Sprintf(
			"https://%s/%s/%s",
			s.l.Addr(),
			s.params.URLPaths.InOut,
			id,
		),
		pr,
	)
	if nil != err {
		t.Fatalf("Error rolling HTTP request: %v", err)
	}
	res, err := c.Do(req)
	if nil != err {
		t.Fatalf("Error sending request: %v", err)
	}
	defer res.Body.Close()

	/* Add the RemoteAddr (really, local address) in the request for
	checking logs. */
	req.RemoteAddr = <-addrCh

	/* Wait until the shell connects. */
	opshell.ExpectShellMessages(t, och,
		testReqCLine(
			t,
			iobroker.LogColor,
			res.Request,
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
		defer cancel()
		/* Read input lines. */
		scanner := bufio.NewScanner(res.Body)
		for scanner.Scan() {
			l := scanner.Text()
			/* exitMsg is like exit to a shell. */
			if exitMsg == l {
				return
			}
			/* Just proxy the line back to the opshell. */
			if _, err := fmt.Fprintf(
				pw,
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

// Do we add correct connection metadata?
func TestServerRequestLogger(t *testing.T) {
	var (
		addrCh              = make(chan string, 1)
		method              = http.MethodGet
		path                = "/" + tlog.S("path")
		uaCh                = make(chan string, 1)
		sni                 = tlog.S("sni")
		tb, _, och, _, c, s = newTestServer(
			t.Context(),
			t,
			&testServerConfig{makeFDir: true},
		)
	)

	/* Configure the client to send an SNI. */
	c.Transport.(*http.Transport).TLSClientConfig.ServerName = sni

	/* Connect to the server, request /, and get our local address and
	user-agent string in the process. */
	req, err := http.NewRequestWithContext(
		httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
			GotConn: func(ci httptrace.GotConnInfo) {
				addrCh <- ci.Conn.LocalAddr().String()
			},
			WroteHeaderField: func(key string, value []string) {
				if "User-Agent" ==
					http.CanonicalHeaderKey(key) {
					select {
					case uaCh <- strings.Join(value, "!"):
					default:
						t.Errorf(
							"Multiple User-Agent " +
								"strings",
						)
					}
				}
			},
		}),
		method,
		fmt.Sprintf("https://%s%s", s.l.Addr(), path),
		nil,
	)
	if nil != err {
		t.Fatalf("Error rolling HTTP request: %v", err)
	}
	res, err := c.Do(req)
	if nil != err {
		t.Fatalf("Error sending request: %v", err)
	}
	defer res.Body.Close()
	req.RemoteAddr = <-addrCh

	/* Logs ok? */
	ra := s.l.Addr().String()
	opshell.ExpectShellMessages(t, och, testReqCLine(
		t,
		fileColor,
		req,
		"File requested: %s", path,
	))
	tb.Expect(t.Context(), t,
		tlog.M.
			With(LKRequestInfo, map[string]any{
				"remote_addr": req.RemoteAddr,
				"method":      method,
				"request_uri": path,
				"protocol":    req.Proto,
				"host":        ra,
				"sni":         sni,
				"user_agent":  <-uaCh,
				"id":          "",
			}).
			With(LKRequestedFile, path).
			With(LKStaticFilesDir, s.params.StaticFilesDir).
			Info(LMFileRequested),
	)
}

// Can we start full duplex mode on a real network connection?
// Meant to be run 10k+ times.
func TestStartFullDuplex_RealNetwork(t *testing.T) {
	var (
		done   = make(chan struct{})
		pr, pw = io.Pipe()
	)
	defer pw.Close()
	defer pr.Close()

	/* Server which starts full duplex on one connection. */
	svr := httptest.NewServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		close(done)
		defer r.Body.Close()
		if err := StartFullDuplex(w, r); nil != err {
			t.Errorf("Error starting full duplex: %v", err)
		}
	}))
	defer svr.Close()

	pw.Close()
	res, err := svr.Client().Post(svr.URL, "", pr)
	if nil != err {
		t.Fatalf("Error making POST request: %v", err)
	}
	defer res.Body.Close()

	/* Give the handler time to report the error. */
	<-done
}

// Do we handle failures to enable duplex mode properly?
func TestStartFullDuplex_Error(t *testing.T) {
	var (
		_, _, och, _, _, s = newTestServer(
			t.Context(),
			t,
			&testServerConfig{noLogHI: true},
		)
		req = httptest.NewRequest(http.MethodPost, "/", nil)
		rr  = httptest.NewRecorder()
		buf = new(bytes.Buffer)
	)

	/* Try to handle in/out with a writer which doesn't support duplex. */
	rr.Body = buf
	s.inOutHandler(rr, req)

	/* Shouldn't have got an error or body. */
	if http.StatusOK != rr.Code {
		t.Errorf("Unexpect response code: %v", rr.Code)
	}
	if 0 != buf.Len() {
		t.Errorf("Unexpected response body: %q", buf.String())
	}

	/* But should be warned. */
	opshell.ExpectShellMessages(t, och, testReqCLine(
		t,
		errorColor,
		req,
		"Error starting duplex comms: enabling full duplex: %v",
		http.ErrNotSupported,
	))
}

// Can we add testNoLogHIKey to t.Context?
func TestTestCtxWithNoLogHI(t *testing.T) {
	ctx := testCtxWithNoLogHI(t)
	v := ctx.Value(testNoLogHIKey{})
	if nil == v {
		t.Fatalf("Value not set")
	}
	b, ok := v.(bool)
	if !ok {
		t.Fatalf("Value not a bool")
	}
	if !b {
		t.Fatalf("Value was false")
	}
}

// Can we make a tagged CLine from an httptest.NewRequest request?
func TestTestReqCLine_httptestNewRequest(t *testing.T) {
	var (
		req = httptest.NewRequest(http.MethodGet, "/", nil)
		n   = 123
		s   = tlog.S("s")
	)

	/* Work out the tag. */
	h, _, ok := strings.Cut(req.RemoteAddr, ":")
	if !ok {
		t.Fatalf("Remote address %q had no colon", req.RemoteAddr)
	}

	got := testReqCLine(t, opshell.ColorRed, req, "%d %s", 123, s)
	want := opshell.CLine{
		Color: opshell.ColorRed,
		Line:  "[" + h + "] " + strconv.Itoa(n) + " " + s,
	}

	if got != want {
		t.Errorf("CLine incorrect\n got: %v\nwant: %v", got, want)
	}
}

// testResponseRecoderFlushError is an http.ResponseWriter with a FlushError
// method that always returns the embedded error.
type testResponseRecorderErrFlush struct {
	*httptest.ResponseRecorder
	feErr error
}

// FlushError returns rr.feErr.
func (rr testResponseRecorderErrFlush) FlushError() error { return rr.feErr }

// EnableFullDuplex returns nil.
func (rr testResponseRecorderErrFlush) EnableFullDuplex() error { return nil }

// Do we handle an inability to flush properly?
func TestStartFullDuplex_FlushError(t *testing.T) {
	var (
		rr = &testResponseRecorderErrFlush{
			ResponseRecorder: httptest.NewRecorder(),
			feErr:            fmt.Errorf("%s", tlog.S("err")),
		}
		req = httptest.NewRequest(http.MethodGet, "/", nil)
	)
	if got, want := StartFullDuplex(
		rr,
		req,
	), rr.feErr; !errors.Is(got, want) {
		t.Errorf("Incorrect error\n got: %v\nwant: %v", got, want)
	}
}

// testCtxWithNoLogHI returns t.Context plus testNoLogHIKey set in the context.
func testCtxWithNoLogHI(t *testing.T) context.Context {
	return context.WithValue(t.Context(), testNoLogHIKey{}, true)
}

// testReqCLine returns a CLine tagged with req's RemoteAddr.
func testReqCLine(
	t *testing.T,
	color opshell.Color,
	req *http.Request, /* Tag from RemoteAddr. */
	f string, a ...any, /* Rest of the line. */
) opshell.CLine {
	t.Helper()
	h, _, err := net.SplitHostPort(req.RemoteAddr)
	if nil != err {
		t.Fatalf(
			"Error getting host from remote address %q: %v",
			req.RemoteAddr,
			err,
		)
	}
	return opshell.CLine{
		Color: color,
		Line:  fmt.Sprintf("[%s] %s", h, fmt.Sprintf(f, a...)),
	}
}
