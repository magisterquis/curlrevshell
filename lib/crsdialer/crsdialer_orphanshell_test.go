package crsdialer

/*
 * crsdialer_orphanshell_test.go
 * Make sure we don't leave a shell orphaned
 * By J. Stuart McMurray
 * Created 20250924
 * Last Modified 20250924
 */

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/hsrv"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/sstls"
)

// Make sure closing the connection doesn't leave a shell orphaned.
func TestDial_OrphanShell(t *testing.T) {
	var (
		c      *net.UnixConn
		hdone  = make(chan struct{})
		hready = make(chan struct{})
		sL     sync.Mutex
		serr   = make(chan error, 1)
		sr     *http.Request
		sw     http.ResponseWriter
	)

	/* Start a server listening. */
	l, err := sstls.Listen("tcp", "127.0.0.1:0", "", 0, "")
	if nil != err {
		t.Fatalf("Error listening: %s", err)
	}
	defer l.Close()
	svr := http.Server{
		BaseContext: func(net.Listener) context.Context {
			return t.Context()
		},
		Handler: http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			sL.Lock()
			defer sL.Unlock()
			if nil != sw {
				panic("duplicate ResponseWriter")
			} else if nil != sr {
				panic("duplicate Request")
			}
			if err := hsrv.StartFullDuplex(w); nil != err {
				panic("starting full duplex: " + err.Error())
			}
			sw, sr = w, r
			close(hready)
			<-hdone
		}),
	}
	go func() { serr <- svr.Serve(l) }()

	/* Get a connection. */
	eg, ctx := ctxerrgroup.WithContext(t.Context())
	eg.GoContext(ctx, func(ctx context.Context) error { /* Client. */
		var err error
		c, err = Dial(
			t.Context(),
			fmt.Sprintf("https://%s", l.Addr()),
			l.Fingerprint,
		)
		return err
	})
	eg.GoContext(ctx, func(ctx context.Context) error { /* Handler. */
		select {
		case <-hready: /* Good. */
		case <-ctx.Done(): /* Less good. */
		}
		return nil
	})
	if err := eg.Wait(); nil != err {
		t.Fatalf("Dial failed: %s", err)
	} else if nil == sr {
		t.Fatalf("Did not get Request from server")
	} else if nil == sw {
		t.Fatalf("Did not get ResponseWriter from server")
	}

	/* Hook client up to a shell. */
	var (
		sherr = make(chan error, 1)
		sh    = exec.Command("/bin/sh")
	)
	sh.Stdin = c
	sh.Stdout = c
	sh.Stderr = c
	if err := sh.Start(); nil != err {
		t.Fatalf("Error starting shell: %s", err)
	}
	go func() { sherr <- sh.Wait() }()

	/* Should be able to talk to the shell. */
	var (
		scanner = bufio.NewScanner(sr.Body)
		got     string
	)
	eg = new(ctxerrgroup.Group)
	eg.Go(func() error {
		cmd := "echo $$"
		if _, err := fmt.Fprintf(sw, "%s\n", cmd); nil != err {
			return fmt.Errorf("sending %s: %w", cmd, err)
		}
		if err := http.NewResponseController(sw).Flush(); nil != err {
			return fmt.Errorf("flushing: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		if scanner.Scan() {
			got = scanner.Text()
		}
		if err := scanner.Err(); nil != err {
			return fmt.Errorf("reading from shell: %w", err)
		}
		return nil
	})
	if err := eg.Wait(); nil != err {
		t.Errorf("Error talking to shell: %s", err)
	}
	if _, err := strconv.Atoi(got); nil != err {
		t.Errorf("Shell returned invalid PID %q: %s", got, err)
	}

	/* Finish the handler and close the server. */
	close(hdone)
	if err := svr.Shutdown(t.Context()); nil != err {
		t.Errorf("Error shutting down server: %s", err)
	}
	if err := <-serr; nil == err {
		t.Errorf("Server finished without expected error")
	} else if !errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Server finished unexpected error: %s", err)
	}

	/* Close the conn, shell should exit. */
	if err := c.Close(); nil != err {
		t.Errorf("Error closing connection to server: %s", err)
	}
	if err := <-sherr; nil != err {
		t.Errorf("Shell exited with error: %s", err)
	}
}
