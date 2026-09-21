package simpleshell

/*
 * simpleshell_test.go
 * Tests for simpleshell.go
 * By J. Stuart McMurray
 * Created 20241013
 * Last Modified 20260802
 */

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/magisterquis/curlrevshell/internal/hsrv"
	"github.com/magisterquis/curlrevshell/lib/sstls"
	"golang.org/x/sync/errgroup"
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
		handleErr    error
		handleCalled atomic.Uint64
		input        = "kittens"
		output       = new(bytes.Buffer)
		ctx, cancel  = context.WithCancel(context.Background())
	)
	defer cancel()

	/* Spawn something like a server, for testing. */
	var (
		mux = http.NewServeMux()
		svr = http.Server{Handler: mux}
	)
	defer svr.Close()
	mux.HandleFunc(IOPath, func(w http.ResponseWriter, r *http.Request) {
		/* Don't double-handle */
		if 1 != handleCalled.Add(1) {
			return
		}
		defer cancel()
		if err := hsrv.StartFullDuplex(w, r); nil != err {
			handleErr = fmt.Errorf("starting duplex: %w", err)
		}
		var eg errgroup.Group
		eg.Go(func() error {
			if _, err := fmt.Fprintf(w, "%s", input); nil != err {
				return fmt.Errorf("sending input: %s", err)
			}
			if err := http.NewResponseController(
				w,
			).Flush(); nil != err {
				return fmt.Errorf("flushing: %w", err)
			}
			return nil
		})
		eg.Go(func() error {
			if _, err := output.ReadFrom(io.LimitReader(
				r.Body,
				int64(len(input)),
			)); nil != err {
				return fmt.Errorf("reading body: %w", err)
			}
			return nil
		})
		handleErr = eg.Wait()
	})
	l, err := sstls.Listen("tcp", "127.0.0.1:0", "", time.Hour, "")
	if nil != err {
		t.Fatalf("Error starting listener: %s", err)
	}
	defer l.Close()

	/* Hook up a shell. */
	_, _, shell := NewEchoShell()
	eg, ectx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return Go(ectx, ConnConfig{
			C2:          "https://" + l.Addr().String() + IOPath,
			Fingerprint: l.Fingerprint,
		}, shell)
	})
	eg.Go(func() error {
		if err := svr.Serve(l); nil != err && !errors.Is(
			err,
			http.ErrServerClosed,
		) {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		<-ectx.Done()
		if err := svr.Shutdown(context.Background()); nil != err {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	})

	/* Make sure it all went well. */
	if err := eg.Wait(); nil != err {
		t.Errorf("Error: %s", err)
	}
	if got := output.String(); got != input {
		t.Errorf("Output incorrect:\n got: %s\nwant: %s", got, input)
	}
	if nil != handleErr {
		t.Errorf("Handler error: %s", err)
	}
	if got := handleCalled.Load(); 1 != got {
		t.Errorf("Handler called %d times, not once", got)
	}
}
