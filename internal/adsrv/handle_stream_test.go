package adsrv

/*
 * handle_stream_test.go
 * Tests for handle_stream.go
 * By J. Stuart McMurray
 * Created 20260808
 * Last Modified 20260812
 */

import (
	"bufio"
	"context"
	"encoding/json/jsontext"
	"fmt"
	"io"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/internal/bidirpipe"
	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/crsadapter"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// Can we shell over a pair of unidirectional streams?
func TestServerHandleStream_SeparateInputAndOutput(t *testing.T) {
	synctest.Test(t, testServerHandleStreamSeparateInputAndOutput)
}
func testServerHandleStreamSeparateInputAndOutput(t *testing.T) {
	var (
		id, tag    = tlog.S("id"), tlog.S("tag")
		inLogInfo  = tlog.S("in-log-info")
		outLogInfo = tlog.S("out-log-info")
		pch        = make(chan bidirpipe.Pipe)
		pl, pr     = bidirpipe.New()

		im, tb, ich, och, c, _ = newTestServer(t, &testServerConfig{
			ctx: context.WithValue(
				t.Context(),
				testClientsChannelContextKey{},
				pch,
			),
		})
	)
	defer pl.Close()
	defer close(pch)

	/* Existing connection is shell input. */
	shellInput := crsadapter.NewStream(c)
	cReq := crsadapter.ConnRequest{
		ConnType: crsadapter.ConnTypeShellStream,
		Args: crsadapter.ConnTypeShellStreamArgs{
			Direction: crsadapter.ShellStreamDirectionInput,
			ID:        id,
			Tag:       tag,
			LogInfo:   inLogInfo,
		},
	}
	if err := shellInput.Send(cReq); nil != err {
		t.Fatalf("Error requesting input connection: %v", err)
	}
	var cRes crsadapter.ConnResponse
	if err := shellInput.DecodeNext(&cRes); nil != err {
		t.Fatalf("Error decoding response to input request: %v", err)
	} else if "" != cRes.Error {
		t.Fatalf("Unhappy response to input request: %v", cRes)
	}

	/* Logs and shell messages look ok? */
	ibm := im.
		With(LKAdapterInfo, inLogInfo).
		With(iobroker.LKID, id)
	ibmd := ibm.
		With(iobroker.LKDirection, iobroker.LVInput)

	tb.WithExpectEmpty().Expect(t.Context(), t,
		im.
			With(LKConnRequest, cReq).
			Debug(LMConnRequestReceived),
		im.
			With(LKAdapterInfo, inLogInfo).
			With(LKConnResponse, cRes).
			Debug(LMConnResponseSent),
		ibmd.
			Info(iobroker.LMNewConnection),
	)
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: iobroker.LogColor,
		Line:  fmt.Sprintf("[%s] Input connected: ID %s", tag, id),
	})

	/* New connection for shell output. */
	pch <- pr
	shellOutput := crsadapter.NewStream(pl)
	cReq = crsadapter.ConnRequest{
		ConnType: crsadapter.ConnTypeShellStream,
		Args: crsadapter.ConnTypeShellStreamArgs{
			Direction: crsadapter.ShellStreamDirectionOutput,
			ID:        id,
			Tag:       tag,
			LogInfo:   outLogInfo,
		},
	}
	if err := shellOutput.Send(cReq); nil != err {
		t.Fatalf("Error requesting input connection: %v", err)
	}
	cRes = crsadapter.ConnResponse{}
	if err := shellOutput.DecodeNext(&cRes); nil != err {
		t.Fatalf("Error decoding response to input request: %v", err)
	} else if "" != cRes.Error {
		t.Fatalf("Unhappy response to input request: %v", cRes)
	}

	/* Logs and shell messages look ok? */
	om := tlog.M.
		With(LKConnectionName, "adapter-conn-2")
	obm := om.
		With(LKAdapterInfo, outLogInfo).
		With(iobroker.LKID, id)
	obmd := obm.
		With(iobroker.LKDirection, iobroker.LVOutput)
	tb.WithExpectEmpty().Expect(t.Context(), t,
		om.
			Debug(LMNewAdapterConn),
		om.
			With(LKConnRequest, cReq).
			Debug(LMConnRequestReceived),
		om.
			With(LKAdapterInfo, outLogInfo).
			With(LKConnResponse, cRes).
			Debug(LMConnResponseSent),
		obmd.
			Info(iobroker.LMNewConnection),
		obm.
			Info(iobroker.LMShellStarting),
	)
	opshell.ExpectShellMessages(t, och, []opshell.CLine{{
		Color: iobroker.LogColor,
		Line:  fmt.Sprintf("[%s] Output connected: ID %s", tag, id),
	}, {
		Color: iobroker.LogColor,
		Line:  fmt.Sprintf("[%s] %s", tag, iobroker.SMShellIsReady),
	}}...)

	/* Send between the two. */
	var (
		eg, ctx     = ctxerrgroup.WithContext(t.Context())
		msg         = tlog.S("msg")
		shellPrefix = tlog.S("shell-prefix")

		want = shellPrefix + msg
	)
	eg.GoTag(ctx, "input", func(ctx context.Context) error {
		/* Send a line to the shell. */
		select {
		case ich <- msg:
		case <-ctx.Done():
			t.Errorf("Context finished before shell input sent")
		}
		return nil
	})
	eg.GoTag(ctx, "shell", func(ctx context.Context) error {
		var line string
		/* Get the input line. */
		scanner := bufio.NewScanner(shellInput)
		if scanner.Scan() {
			line = scanner.Text()
			/* Correct? */
			if got, want := line, msg; got != want {
				t.Errorf(
					"Shell read incorrect line\n"+
						" got: %q\n"+
						"want: %q",
					got,
					want,
				)
			}
		} else {
			t.Errorf("Shell got no line")
		}
		/* Or was there an error? */
		err := scanner.Err()
		if nil != err {
			t.Errorf("Error reading shell input: %v", err)
			return err
		}
		/* Send the output back. */
		if _, err := io.WriteString(
			shellOutput,
			shellPrefix+line,
		); nil != err {
			t.Errorf("Error sending shell output: %v", err)
			return err
		}
		return nil
	})
	eg.GoTag(ctx, "output", func(ctx context.Context) error {
		/* Get a line of output. */
		toch := make(chan opshell.CLine, 1)
		select {
		case got := <-och:
			toch <- got
		case <-ctx.Done():
			t.Errorf("Did not get shell output")
			return nil
		}
		/* Correct? */
		opshell.ExpectShellMessages(t, toch, opshell.CLine{
			Line:  want,
			Plain: true,
		})
		return nil
	})
	if err := eg.Wait(); nil != err {
		t.Errorf("Error sending to/from shell: %v", err)
	}

	/* Logs ok? */
	tb.WithExpectUnordered().Expect(t.Context(), t,
		ibmd.
			With(iobroker.LKData, msg+"\n").
			Info(iobroker.LMShellIO),
		obmd.
			With(iobroker.LKData, want).
			Info(iobroker.LMShellIO),
	)

	/* Close the connection. */
	if err := shellOutput.Close(); nil != err {
		t.Errorf("Error closing output stream: %v", err)
	}

	/* Logs ok? */
	opshell.ExpectShellMessages(t, och, []opshell.CLine{{
		Color: iobroker.ErrColor,
		Line:  fmt.Sprintf("[%s] Output connection closed", tag),
	}, {
		Color: iobroker.ErrColor,
		Line:  fmt.Sprintf("[%s] Input connection closed", tag),
	}, {
		Color: iobroker.ErrColor,
		Line:  fmt.Sprintf("[%s] %s", tag, iobroker.SMShellIsGone),
	}}...)
	tb.Expect(t.Context(), t,
		obmd.
			Info(iobroker.LMConnectionClosed),
		ibmd.
			Info(iobroker.LMConnectionClosed),
		ibm.
			Info(iobroker.LMShellFinished),
	)
}

// Can we shell over a bidirectional stream?
func TestServerHandleStream_Bidirectional(t *testing.T) {
	synctest.Test(t, testServerHandleStreamBidirectional)
}
func testServerHandleStreamBidirectional(t *testing.T) {
	var (
		id, tag = tlog.S("id"), tlog.S("tag")
		logInfo = tlog.S("log-info")

		m, tb, ich, och, c, _ = newTestServer(t, nil)
	)

	/* Upgrade to a shell. */
	js := crsadapter.NewStream(c)
	cReq := crsadapter.ConnRequest{
		ConnType: crsadapter.ConnTypeShellStream,
		Args: crsadapter.ConnTypeShellStreamArgs{
			Direction: crsadapter.ShellStreamDirectionInOut,
			ID:        id,
			Tag:       tag,
			LogInfo:   logInfo,
		},
	}
	if err := js.Send(cReq); nil != err {
		t.Fatalf("Error requesting in/out connection: %v", err)
	}
	var cRes crsadapter.ConnResponse
	if err := js.DecodeNext(&cRes); nil != err {
		t.Fatalf("Error decoding response to in/out request: %v", err)
	} else if "" != cRes.Error {
		t.Fatalf("Unhappy response to in/out request: %v", cRes)
	}

	/* Logs and shell messages look ok? */
	bm := m.
		With(LKAdapterInfo, logInfo).
		With(iobroker.LKID, id)
	bmd :=
		bm.
			With(iobroker.LKDirection, iobroker.LVBidir)
	tb.WithExpectEmpty().Expect(t.Context(), t,
		m.
			With(LKConnRequest, cReq).
			Debug(LMConnRequestReceived),
		m.
			With(LKAdapterInfo, logInfo).
			With(LKConnResponse, cRes).
			Debug(LMConnResponseSent),
		bmd.
			Info(iobroker.LMNewConnection),
		bmd.
			Info(iobroker.LMShellStarting),
	)
	opshell.ExpectShellMessages(t, och, []opshell.CLine{{
		Color: iobroker.LogColor,
		Line:  fmt.Sprintf("[%s] Connected: ID %s", tag, id),
	}, {
		Color: iobroker.LogColor,
		Line:  fmt.Sprintf("[%s] %s", tag, iobroker.SMShellIsReady),
	}}...)

	/* Send to/from a shell. */
	var (
		eg, ctx     = ctxerrgroup.WithContext(t.Context())
		msg         = tlog.S("msg")
		shellPrefix = tlog.S("shell-prefix")

		want = shellPrefix + msg
	)
	eg.GoTag(ctx, "input", func(ctx context.Context) error {
		/* Send a line to the shell. */
		select {
		case ich <- msg:
		case <-ctx.Done():
			t.Errorf("Context finished before shell input sent")
		}
		return nil
	})
	eg.GoTag(ctx, "shell", func(ctx context.Context) error {
		var line string
		/* Get the input line. */
		scanner := bufio.NewScanner(js)
		if scanner.Scan() {
			line = scanner.Text()
			/* Correct? */
			if got, want := line, msg; got != want {
				t.Errorf(
					"Shell read incorrect line\n"+
						" got: %q\n"+
						"want: %q",
					got,
					want,
				)
			}
		} else {
			t.Errorf("Shell got no line")
		}
		/* Or was there an error? */
		err := scanner.Err()
		if nil != err {
			t.Errorf("Error reading shell input: %v", err)
			return err
		}
		/* Send the output back. */
		if _, err := io.WriteString(js, shellPrefix+line); nil != err {
			t.Errorf("Error sending shell output: %v", err)
			return err
		}
		return nil
	})
	eg.GoTag(ctx, "output", func(ctx context.Context) error {
		/* Get a line of output. */
		toch := make(chan opshell.CLine, 1)
		select {
		case got := <-och:
			toch <- got
		case <-ctx.Done():
			t.Errorf("Did not get shell output")
			return nil
		}
		/* Correct? */
		opshell.ExpectShellMessages(t, toch, opshell.CLine{
			Line:  want,
			Plain: true,
		})
		return nil
	})
	if err := eg.Wait(); nil != err {
		t.Errorf("Error sending to/from shell: %v", err)
	}

	/* Logs ok? */
	tb.WithExpectUnordered().Expect(t.Context(), t,
		bm.
			With(iobroker.LKData, msg+"\n").
			With(iobroker.LKDirection, iobroker.LVInput).
			Info(iobroker.LMShellIO),
		bm.
			With(iobroker.LKData, want).
			With(iobroker.LKDirection, iobroker.LVOutput).
			Info(iobroker.LMShellIO),
	)

	/* Close the connection. */
	if err := js.Close(); nil != err {
		t.Errorf("Error closing output stream: %v", err)
	}

	/* Logs ok? */
	opshell.ExpectShellMessages(t, och, []opshell.CLine{{
		Color: iobroker.ErrColor,
		Line:  fmt.Sprintf("[%s] Connection closed", tag),
	}, {
		Color: iobroker.ErrColor,
		Line:  fmt.Sprintf("[%s] %s", tag, iobroker.SMShellIsGone),
	}}...)
	tb.Expect(t.Context(), t,
		bmd.
			Info(iobroker.LMConnectionClosed),
		bmd.
			Info(iobroker.LMShellFinished),
	)
}

// Can we handle unparseable arguments?
func TestServerHandleStream_ArgsParseError(t *testing.T) {
	synctest.Test(t, testServerHandleStreamArgsParseError)
}
func testServerHandleStreamArgsParseError(t *testing.T) {
	var (
		tb, sl  = tlog.NewBuffer()
		jv      = jsontext.Value(tlog.S("invalid"))
		sc, ss  = newTestJSONStreamPair(t)
		wantErr = "invalid character 'i' looking for beginning of value"
		wg      sync.WaitGroup
	)
	/* Attempt to parse invalid args. */
	wg.Go(func() {
		defer ss.Close()
		if err := new(Server).handleStream(
			t.Context(),
			sl,
			"",
			jv,
			ss,
		); nil != err {
			t.Errorf("Error from handleStream: %v", err)
		}
	})
	/* Error on the connection correct? */
	wg.Go(func() {
		var cRes crsadapter.ConnResponse
		if err := sc.DecodeNext(&cRes); nil != err {
			t.Errorf("Error decoding response: %v", err)
			return
		}
		if got, want := cRes.Error, wantErr; got != want {
			t.Errorf(
				"Incorrect ConnResponse error\n"+
					" got: %s\n"+
					"want: %s",
				got,
				want,
			)
		}
	})
	wg.Wait()

	/* Logs correct? */
	tb.WithExpectEmpty().Expect(t.Context(), t,
		tlog.M.
			With(LKError, wantErr).
			Warn(LMConnRequestArgsError),
		tlog.M.
			With(
				LKConnResponse,
				crsadapter.ConnResponse{Error: wantErr},
			).
			Debug(LMConnResponseSent),
	)
}

// Can we handle an unknown direction?
func TestServerHandleStream_UnknownDirection(t *testing.T) {
	synctest.Test(t, testServerHandleStreamUnknownDirection)
}
func testServerHandleStreamUnknownDirection(t *testing.T) {
	var (
		id, tag           = tlog.S("id"), tlog.S("tag")
		logInfo           = tlog.S("log-info")
		m, tb, _, _, c, _ = newTestServer(t, nil)
		dir               = crsadapter.ShellStreamDirection(
			tlog.S("invalid-direction"),
		)

		wantErr = UnknownShellStreamDirectionError{dir}
	)

	/* Upgrade to a shell. */
	js := crsadapter.NewStream(c)
	cReq := crsadapter.ConnRequest{
		ConnType: crsadapter.ConnTypeShellStream,
		Args: crsadapter.ConnTypeShellStreamArgs{
			Direction: crsadapter.ShellStreamDirection(dir),
			ID:        id,
			Tag:       tag,
			LogInfo:   logInfo,
		},
	}
	if err := js.Send(cReq); nil != err {
		t.Fatalf("Error requesting input connection: %v", err)
	}
	var cRes crsadapter.ConnResponse
	if err := js.DecodeNext(&cRes); nil != err {
		t.Fatalf("Error decoding response to input request: %v", err)
	}
	if got, want := cRes.Error, wantErr.Error(); got != want {
		t.Errorf(
			"Incorrect response error\n got: %s\nwant: %s",
			got,
			want,
		)
	}
	tb.WithExpectEmpty().Expect(t.Context(), t,
		m.
			With(LKConnRequest, cReq).
			Debug(LMConnRequestReceived),
		m.
			With(LKAdapterInfo, logInfo).
			With(LKConnResponse, cRes).
			Debug(LMConnResponseSent),
	)
}

// Can we handle not being able to reply properly?
func TestServerHandleStream_ReplyError(t *testing.T) {
	synctest.Test(t, testServerHandleStreamReplyError)
}
func testServerHandleStreamReplyError(t *testing.T) {
	var (
		id, tag           = tlog.S("id"), tlog.S("tag")
		logInfo           = tlog.S("log-info")
		m, tb, _, _, c, _ = newTestServer(t, nil)
	)

	/* Upgrade to a shell. */
	js := crsadapter.NewStream(c)
	cReq := crsadapter.ConnRequest{
		ConnType: crsadapter.ConnTypeShellStream,
		Args: crsadapter.ConnTypeShellStreamArgs{
			Direction: crsadapter.ShellStreamDirectionInput,
			ID:        id,
			Tag:       tag,
			LogInfo:   logInfo,
		},
	}
	if err := js.Send(cReq); nil != err {
		t.Fatalf("Error requesting input connection: %v", err)
	}
	/* Close the connection before we get a response, to cause an error. */
	if err := js.Close(); nil != err {
		t.Fatalf("Error closing JSON Stream: %v", err)
	}
	/* Get logs? */
	tb.WithExpectEmpty().Expect(t.Context(), t,
		m.
			With(LKConnRequest, cReq).
			Debug(LMConnRequestReceived),
		m.
			With(LKAdapterInfo, logInfo).
			With(LKError, fmt.Sprintf(
				"jsontext: write error: %v",
				io.ErrClosedPipe,
			)).
			Warn(LMConnResponseError),
	)
}

// Do we cancel the context passed to iobroker when an input stream
// disconnects?
func TestServerHandleStream_InputDisconnect(t *testing.T) {
	var (
		id, tag   = tlog.S("id"), tlog.S("tag")
		inLogInfo = tlog.S("in-log-info")

		m, tb, _, och, c, _ = newTestServer(t, nil)
	)

	/* Turn connection into shell input. */
	shellInput := crsadapter.NewStream(c)
	cReq := crsadapter.ConnRequest{
		ConnType: crsadapter.ConnTypeShellStream,
		Args: crsadapter.ConnTypeShellStreamArgs{
			Direction: crsadapter.ShellStreamDirectionInput,
			ID:        id,
			Tag:       tag,
			LogInfo:   inLogInfo,
		},
	}
	if err := shellInput.Send(cReq); nil != err {
		t.Fatalf("Error requesting input connection: %v", err)
	}
	var cRes crsadapter.ConnResponse
	if err := shellInput.DecodeNext(&cRes); nil != err {
		t.Fatalf("Error decoding response to input request: %v", err)
	} else if "" != cRes.Error {
		t.Fatalf("Unhappy response to input request: %v", cRes)
	}

	/* Logs and shell messages look ok? */
	md := m.
		With(LKAdapterInfo, inLogInfo).
		With(iobroker.LKDirection, iobroker.LVInput).
		With(iobroker.LKID, id)
	tb.WithExpectEmpty().Expect(t.Context(), t,
		m.
			With(LKConnRequest, cReq).
			Debug(LMConnRequestReceived),
		m.
			With(LKAdapterInfo, inLogInfo).
			With(LKConnResponse, cRes).
			Debug(LMConnResponseSent),
		md.
			Info(iobroker.LMNewConnection),
	)
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: iobroker.LogColor,
		Line:  fmt.Sprintf("[%s] Input connected: ID %s", tag, id),
	})

	/* Close our side of the connection.  Shell should tell the user that
	input closed. */
	if err := shellInput.Close(); nil != err {
		t.Fatalf("Error closing shell input: %v", err)
	}

	/* Close happily? */
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: iobroker.ErrColor,
		Line:  fmt.Sprintf("[%s] Input connection closed", tag),
	})
	tb.WithExpectEmpty().Expect(t.Context(), t,
		md.
			Info(iobroker.LMConnectionClosed),
	)
}
