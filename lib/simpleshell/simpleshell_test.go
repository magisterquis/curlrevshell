package simpleshell

/*
 * simpleshell_test.go
 * Tests for simpleshell.go
 * By J. Stuart McMurray
 * Created 20241013
 * Last Modified 20260807
 */

import (
	"cmp"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/hsrv"
	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/sstls"
)

func TestSplitArgs(t *testing.T) {
	for have, want := range map[string][]string{
		"|foo|bar|tridge|": {"foo", "bar", "tridge", ""},
		"":                 {},
		"|":                {},
		"|foo":             {"foo"},
		"|foo|bar":         {"foo", "bar"},
	} {
		t.Run(have, func(t *testing.T) {
			got := SplitArgs(have)
			if !slices.Equal(got, want) {
				t.Errorf(
					"Incorrect split:\n"+
						"have: %q\n"+
						" got: %#v\n"+
						"want: %#v",
					have,
					got,
					want,
				)
			}
		})
	}
}

func TestGo(t *testing.T) {
	var (
		errDone = errors.New("done")
		hErrCh  = make(chan error, 1)
		input   = tlog.S("kittens")
		urlCh   = make(chan string, 1)

		output = make([]byte, len(input))
	)
	/* Cheesy curlrevshell knockoff. */
	svr := httptest.NewUnstartedServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		defer close(hErrCh)
		defer r.Body.Close()
		if err := hsrv.StartFullDuplex(w, r); nil != err {
			hErrCh <- fmt.Errorf("starting duplex: %w", err)
		}
		eg, ctx := ctxerrgroup.WithContext(r.Context())
		/* Send input. */
		eg.GoTag(ctx, "input", func(ctx context.Context) error {
			if _, err := io.WriteString(w, input); nil != err {
				return err
			}
			if err := http.NewResponseController(
				w,
			).Flush(); nil != err {
				return err
			}

			return nil
		})
		/* Get output. */
		eg.GoTag(ctx, "output", func(ctx context.Context) error {
			n, err := io.ReadFull(r.Body, output)
			output = output[:n]
			if nil != err {
				t.Errorf("Error receiving output: %v", err)
			}
			return cmp.Or(err, errDone)
		})

		/* Wait until we're done. */
		hErrCh <- eg.Wait()
	}))
	cert, err := sstls.GetCertificate("", nil, nil, 0, "")
	if nil != err {
		t.Fatalf("Error getting certificate: %v", err)
	}
	svr.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	fp, err := sstls.PubkeyFingerprintTLS(cert)
	if nil != err {
		t.Fatalf("Error getting TLS fingerprint: %v", err)
	}

	/* Start the shell and the server. */
	eg, ctx := ctxerrgroup.WithContext(t.Context())
	eg.GoTag(ctx, "server", func(ctx context.Context) error {
		defer svr.Close()
		/* Run the server with a child context, more or less. */
		svr.Config.BaseContext = func(net.Listener) context.Context {
			return ctx
		}

		/* Start the server and send the URL to the shell. */
		svr.StartTLS()
		urlCh <- svr.URL

		/* Wait for the handler to finish or something to go wrong. */
		select {
		case <-ctx.Done():
			return nil
		case err := <-hErrCh:
			svr.CloseClientConnections()
			return err
		}
	})
	eg.GoTag(ctx, "shell", func(ctx context.Context) error {
		_, _, shell := NewEchoShell()
		return Go(ctx, ConnConfig{
			C2:          <-urlCh,
			Fingerprint: fp,
		}, shell)
	})

	/* Make sure it all went well. */
	if got, want := eg.Wait(), errDone; !errors.Is(got, want) {
		t.Fatalf("Incorrect error\n got: %v\nwant: %v", got, want)
	}
	if got, want := string(output), input; got != want {
		t.Errorf("Output incorrect:\n got: %s\nwant: %s", got, want)
	}
}
