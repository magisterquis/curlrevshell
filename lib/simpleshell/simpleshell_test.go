package simpleshell

/*
 * simpleshell_test.go
 * Tests for simpleshell.go
 * By J. Stuart McMurray
 * Created 20241013
 * Last Modified 20241013
 */

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/magisterquis/curlrevshell/lib/sstls"
	"golang.org/x/sync/errgroup"
)

func TestTLSCertificateVerifier(t *testing.T) {
	/* TLS listener with known fingerprint. */
	l, err := sstls.Listen("tcp", "127.0.0.1:0", "", time.Hour, "")
	if nil != err {
		t.Fatalf("Error starting listener: %s", err)
	}
	defer l.Close()

	/* txrx sends and receives a byte on c, to make sure the handshake
	happens.  c is then closed. */
	txrx := func(c net.Conn) error {
		ech := make(chan error, 2)
		defer c.Close()
		go func() { _, err := c.Write(make([]byte, 1)); ech <- err }()
		go func() { _, err := c.Read(make([]byte, 1)); ech <- err }()
		for range 2 {
			if err := <-ech; nil != err {
				return err
			}
		}
		return nil
	}

	/* try makes a connection to t expecting the fingerprint fp.  It
	returns the errors from t.Accept and tls.Dial, in that order. */
	try := func(fp string) (lerr, derr error) {
		var wg sync.WaitGroup
		wg.Add(2)
		/* Make the connection. */
		go func() {
			defer wg.Done()
			tv, err := TLSFingerprintVerifier(fp)
			if nil != err {
				derr = fmt.Errorf(
					"generating verifier: %w",
					err,
				)
				return
			}
			tc := &tls.Config{
				InsecureSkipVerify: true,
				VerifyConnection:   tv,
			}
			var c net.Conn
			if c, derr = tls.Dial(
				"tcp",
				l.Addr().String(),
				tc,
			); nil != derr {
				return
			}
			derr = txrx(c)
		}()
		/* Accept the connection. */
		go func() {
			defer wg.Done()
			var c net.Conn
			if c, lerr = l.Accept(); nil != lerr {
				return
			}
			lerr = txrx(c)
		}()
		/* Wait for it all to happen. */
		wg.Wait()

		return lerr, derr
	}

	t.Run("correct_fingerprint", func(t *testing.T) {
		lerr, derr := try(l.Fingerprint)
		if nil != lerr {
			t.Errorf("Error from listener: %s", lerr)
		}
		if nil != derr {
			t.Errorf("Error from tls.Dial: %s", derr)
		}
	})

	t.Run("incorrect_fingerprint", func(t *testing.T) {
		lerr, derr := try(base64.StdEncoding.EncodeToString(
			make([]byte, 32),
		))
		if nil == lerr {
			t.Errorf("Accept succeeded unexpectedly")
		} else if "remote error: tls: bad certificate" != lerr.Error() {
			t.Errorf("Accept error: %s", lerr)
		}
		if nil == derr {
			t.Errorf("tls.Dial succeeded unexpectedly")
		} else if !errors.Is(derr, ErrNoMatchingCertificate) {
			t.Errorf("Unexpected error from tls.Dial : %s", derr)
		}
	})
}

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
	mux.HandleFunc(IOPath, func(w http.ResponseWriter, r *http.Request) {
		/* Don't double-handle */
		if 1 != handleCalled.Add(1) {
			return
		}
		defer cancel()
		rc := http.NewResponseController(w)
		if err := rc.EnableFullDuplex(); nil != err {
			handleErr = fmt.Errorf("enabling duplex: %w", err)
			return
		}
		if err := rc.Flush(); nil != err {
			handleErr = fmt.Errorf("initial flush: %w", err)
			return
		}
		var eg errgroup.Group
		eg.Go(func() error {
			if _, err := fmt.Fprintf(w, "%s", input); nil != err {
				return fmt.Errorf("sending input: %s", err)
			}
			if err := rc.Flush(); nil != err {
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
		ctx, cancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)
		defer cancel()
		if err := svr.Shutdown(ctx); nil != err {
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
