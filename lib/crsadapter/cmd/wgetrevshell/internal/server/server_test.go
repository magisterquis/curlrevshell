package server

/*
 * server_test.go
 * Tests for server.go
 * By J. Stuart McMurray
 * Created 20260810
 * Last Modified 20260810
 */

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/internal/adsrv"
	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/crsadapter"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
	"golang.org/x/net/nettest"
)

// testBufLen is the size of channel buffers used for testing.
var testBufLen = 1024

// Does Serve work?
func TestServe(t *testing.T) { synctest.Test(t, testServe) }
func testServe(t *testing.T) {
	var (
		aech          = make(chan error, 1)
		bech          = make(chan error, 1)
		cName1        = adsrv.AdapterConnPrefix + "1"
		cName2        = adsrv.AdapterConnPrefix + "2"
		ctb, csl      = tlog.NewBuffer() /* curlrevshell. */
		ich           = make(chan string, testBufLen)
		och           = make(chan opshell.CLine, testBufLen)
		pathPrefix    = tlog.S("prefix")
		sctx, scancel = context.WithCancel(t.Context())
		sech          = make(chan error, 1)
		shellInput    = tlog.S("shell-input-longer-string")
		shellOutput   = tlog.S("shell-output-smlstr")
		urlPathIn     = tlog.S("url-path-in")
		urlPathOut    = tlog.S("url-path-out")
		wtb, wsl      = tlog.NewBuffer() /* wgetrevshell. */

		iob = iobroker.New(och)

		mOutName = tlog.M.
				With(adsrv.LKConnectionName, cName1)
		mOutIOB = mOutName.
			With(iobroker.LKDirection, iobroker.LVOutput).
			With(iobroker.LKID, pidID())
		mInName = tlog.M.
			With(adsrv.LKConnectionName, cName2)
		mInIOB = mInName.
			With(iobroker.LKDirection, iobroker.LVInput).
			With(iobroker.LKID, pidID())
		mIOB = tlog.M.
			With(iobroker.LKID, pidID())
	)
	defer scancel()

	/* Shouldn't have any logs or output left over. */
	t.Cleanup(func() {
		if err := <-aech; nil != err {
			t.Errorf("Adapter server returned error: %v", err)
		}
		if err := <-bech; nil != err {
			t.Errorf("I/O Broker returned error: %v", err)
		}
		if err := <-sech; nil != err {
			t.Errorf("Serve returned error: %v", err)
		}
		close(och)
		ctb.CloseExpectEmpty(context.Background(), t)
		wtb.CloseExpectEmpty(context.Background(), t)
		opshell.ExpectNoShellMessages(t, och)
	})

	/* Start the broker. */
	go func() { bech <- iob.Run(sctx, ich, false) }()

	/* Start an adapter server. */
	asrv, err := adsrv.New(csl, filepath.Join(t.TempDir(), "a"), iob)
	if nil != err {
		t.Fatalf("Error creating adapter server: %v", err)
	}
	go func() { aech <- asrv.Run(sctx) }()
	ctb.Expect(t.Context(), t,
		tlog.M.
			With(adsrv.LKSocketPath, asrv.Addr().String()).
			Info(adsrv.LMAdapterListenerStarted),
	)

	/* Start serving wget. */
	l, err := nettest.NewLocalListener("tcp")
	if nil != err {
		t.Fatalf("Error starting Serve listener: %v", err)
	}
	go func() {
		sech <- Serve(
			sctx,
			wsl,
			l,
			asrv.Addr().(*net.UnixAddr).String(),
			pathPrefix,
			urlPathIn,
			urlPathOut,
		)
	}()
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: iobroker.LogColor,
		Line: fmt.Sprintf(
			"[%s] Output connected: ID %s",
			crsTag,
			pidID(),
		),
	})
	ctb.Expect(t.Context(), t,
		mOutName.
			Debug(adsrv.LMNewAdapterConn),
		mOutName.
			With(adsrv.LKConnRequest, crsadapter.ConnRequest{
				ConnType: crsadapter.ConnTypeShellStream,
				Args: handshakeArgs(
					crsadapter.ShellStreamDirectionOutput,
				),
			}).
			Debug(adsrv.LMConnRequestReceived),
		mOutName.
			With(adsrv.LKConnResponse, crsadapter.ConnResponse{}).
			Debug(adsrv.LMConnResponseSent),
		mOutIOB.
			Info(iobroker.LMNewConnection),
	)

	/* Send some input. */
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	eg, ctx := ctxerrgroup.WithContext(ctx)
	eg.GoTag(ctx, "shell input", func(ctx context.Context) error {
		/* Long-lived input connection to the server. */
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			"http://"+path.Join(
				l.Addr().String(),
				pathPrefix,
				urlPathIn,
			),
			nil,
		)
		if nil != err {
			return fmt.Errorf("rolling request: %w", err)
		}
		res, err := http.DefaultClient.Do(req)
		if nil != err {
			return fmt.Errorf("GET: %w", err)
		}
		defer res.Body.Close()
		if http.StatusOK != res.StatusCode {
			return fmt.Errorf(
				"unexpected HTTP status: %v",
				res.Status,
			)
		}
		/* Get input. */
		b := make([]byte, len(shellInput)+1)
		n, err := io.ReadFull(res.Body, b)
		if nil != err {
			return fmt.Errorf("reading input: %w", err)
		}
		/* Was it correct? */
		if got, want := string(b[:n]), shellInput+"\n"; got != want {
			t.Errorf(
				"Shell input incorrect\n got: %q\nwant: %q",
				got,
				want,
			)
		}
		return nil
	})
	opshell.ExpectShellMessages(t, och, []opshell.CLine{{
		Color: iobroker.LogColor,
		Line: fmt.Sprintf(
			"[%s] Input connected: ID %s",
			crsTag,
			pidID(),
		),
	}, {
		Color: iobroker.LogColor,
		Line:  fmt.Sprintf("[%s] %s", crsTag, iobroker.SMShellIsReady),
	}}...)
	ctb.Expect(t.Context(), t,
		mInName.
			Debug(adsrv.LMNewAdapterConn),
		mInName.
			With(adsrv.LKConnRequest, crsadapter.ConnRequest{
				ConnType: crsadapter.ConnTypeShellStream,
				Args: handshakeArgs(
					crsadapter.ShellStreamDirectionInput,
				),
			}).
			Debug(adsrv.LMConnRequestReceived),
		mInName.
			With(adsrv.LKConnResponse, crsadapter.ConnResponse{}).
			Debug(adsrv.LMConnResponseSent),
		mInIOB.
			Info(iobroker.LMNewConnection),
		mIOB.
			With(adsrv.LKConnectionName, cName2).
			Info(iobroker.LMShellStarting),
	)
	ich <- shellInput
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: iobroker.ErrColor,
		Line:  fmt.Sprintf("[%s] Input connection closed", crsTag),
	})
	ctb.Expect(t.Context(), t,
		mInIOB.
			With(iobroker.LKData, shellInput+"\n").
			Info(iobroker.LMShellIO),
		mInIOB.
			Info(iobroker.LMConnectionClosed),
	)
	wtb.Expect(t.Context(), t,
		tlog.M.
			With(LKSize, len(shellInput)+1).
			With(iobroker.LKDirection, iobroker.LVInput).
			Info(iobroker.LMShellIO),
	)

	/* Send some output. */
	eg.GoTag(ctx, "shell output", func(ctx context.Context) error {
		/* Short-lived shell output connection to the server. */
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			"http://"+path.Join(
				l.Addr().String(),
				pathPrefix,
				urlPathOut,
			),
			bytes.NewReader([]byte(shellOutput)),
		)
		if nil != err {
			return fmt.Errorf("rolling request: %w", err)
		}
		res, err := http.DefaultClient.Do(req)
		if nil != err {
			return fmt.Errorf("POST: %w", err)
		}
		defer res.Body.Close()
		if http.StatusOK != res.StatusCode {
			t.Errorf(
				"Unexpected output HTTP status: %v",
				res.Status,
			)
		}
		return nil
	})
	ctb.Expect(t.Context(), t,
		mOutIOB.
			With(iobroker.LKData, shellOutput).
			Info(iobroker.LMShellIO),
	)
	wtb.Expect(t.Context(), t,
		tlog.M.
			With(LKSize, len(shellOutput)).
			With(iobroker.LKDirection, iobroker.LVOutput).
			Info(iobroker.LMShellIO),
	)
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Line:  shellOutput,
		Plain: true,
	})

	/* All done. */
	if err := eg.Wait(); nil != err {
		t.Fatalf("Shell i/o error: %v", err)
	}

	/* Logs look ok? */
	scancel()
	opshell.ExpectShellMessages(t, och, []opshell.CLine{{
		Color: iobroker.ErrColor,
		Line:  fmt.Sprintf("[%s] Output connection closed", crsTag),
	}, {
		Color: iobroker.ErrColor,
		Line:  fmt.Sprintf("[%s] %s", crsTag, iobroker.SMShellIsGone),
	}}...)
	ctb.Expect(t.Context(), t,
		mOutIOB.
			Info(iobroker.LMConnectionClosed),
		mIOB.
			With(adsrv.LKConnectionName, cName1).
			Info(iobroker.LMShellFinished),
	)
}

func TestServe_OutputConnectFail(t *testing.T) {
	td := t.TempDir()
	if got, want := Serve(
		t.Context(),
		nil,
		nil,
		td,
		"",
		"",
		"",
	), syscall.ENOTSOCK; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error connecting to directory\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
}
