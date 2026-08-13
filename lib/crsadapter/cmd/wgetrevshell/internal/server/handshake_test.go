package server

/*
 * handshake_test.go
 * Tests for handshake.go
 * By J. Stuart McMurray
 * Created 20260812
 * Last Modified 20260812
 */

import (
	"encoding/json/v2"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/crsadapter"
)

// Is our ID in the right format?
func TestID(t *testing.T) {
	var (
		id  = pidID()
		sep = "-"
	)
	prefix, pid, ok := strings.Cut(id, sep)
	if !ok {
		t.Errorf("No hyphen in ID %q", id)
	}
	if got, want := prefix+sep, crsIDPrefix; got != want {
		t.Errorf("Prefix incorrect\n got: %q\nwant: %q", got, want)
	}
	if _, err := strconv.Atoi(pid); nil != err {
		t.Errorf(
			"Error parsing suffix as a number\n"+
				"suffix: %q\n"+
				"   err: %v",
			pid,
			err,
		)
	}
}

// Does handshake report handshake errors?
func TestHandshake_HandshakeError(t *testing.T) {
	var response = tlog.S("Vote for me!")

	/* Curlrevshell, but the sort that doesn't shake hands well. */
	l, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: filepath.Join(t.TempDir(), "l"),
		Net:  "unix",
	})
	if nil != err {
		t.Fatalf("Listen error: %v", err)
	}
	defer l.Close()

	/* Offer to shake hands, get rejected. */
	ech := make(chan error, 1)
	go func() {
		defer close(ech)
		s, err := handshake(
			t.Context(),
			l.Addr().String(),
			crsadapter.ShellStreamDirectionInput,
		)
		if nil != s {
			t.Errorf("Non-nil stream returned")
		}
		ech <- err
	}()

	/* Accept the connection, ignore what it says, respond with something
	unrelated, ask for its vote. */
	var wg sync.WaitGroup
	c, err := l.Accept()
	if nil != err {
		t.Fatalf("Accept error: %v", err)
	}
	defer c.Close()
	wg.Go(func() {
		if _, err := io.Copy(io.Discard, c); nil != err {
			t.Errorf("Error reading request: %v", err)
		}
	})
	wg.Go(func() {
		if err := json.MarshalWrite(c, crsadapter.ConnResponse{
			Error: response,
		}); nil != err {
			t.Errorf("Error sending response: %v", err)
		}
	})
	wg.Wait()

	/* Should have gotten an error. */
	want := crsadapter.ConnResponseError{
		ConnResponse: crsadapter.ConnResponse{
			Error: response,
		},
	}
	if got := <-ech; !errors.Is(got, want) {
		t.Errorf(
			"Handshake returned incorrect error\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}

}
