// Package chanlog - log to a channel, fortesting
package chanlog

/*
 * chanlog.go
 * Log to a channel, for testing
 * By J. Stuart McMurray
 * Created 20240925
 * Last Modified 20240925
 */

import (
	"log/slog"
	"strings"
	"testing"
)

const BufLen = 1024

// ChanLog wraps a chan string as a blockingish logfile.  Writes are sent as
// strings less timestamps and surrounding whitespace to the wrapped chan.
// Don't send it anything which isn't a log line.
type ChanLog chan string

// New returns a new chanLog with a BufLen-sized buffer and slog.Logger which
// wraps it.
func New() (ChanLog, *slog.Logger) {
	cl := ChanLog(make(chan string, BufLen))
	sl := slog.New(slog.NewJSONHandler(
		cl,
		&slog.HandlerOptions{Level: slog.LevelDebug},
	))
	return cl, sl
}

// Write converts b to a string and sends it to cl.  It always returns
// len(b), nil.
func (cl ChanLog) Write(b []byte) (int, error) {
	line := string(b)

	/* Set the timestamp to the empty string. */
	parts := strings.Split(line, `"`)
	parts[3] = ""
	line = strings.Join(parts, `"`)

	/* Remove excess spaces. */
	line = strings.TrimSpace(line)

	/* Send it out. */
	cl <- string(line)
	return len(b), nil
}

// Expect expects the log entries on cl.  It calls t.Errorf for mismatches and
// blocks until as many log entries as supplied lines are read.
func (cl ChanLog) Expect(t *testing.T, lines ...string) {
	t.Helper()
	for _, want := range lines { /* Make sure we get each line. */
		got, ok := <-cl
		if !ok { /* Channel closed early. */
			t.Errorf(
				"Log channel closed while waiting for %q",
				got,
			)
			return
		} else if got != want { /* Wrong line. */
			t.Errorf(
				"Unexpected log line:\n got: %s\nwant: %s",
				got,
				want,
			)
		}
	}
}

// ExpectEmpty is like cl.Expect, except it checks that the log is empty and
// calls t.Errorf which each line if not.  This is inherently racy and should
// be called concurrently to calls to cl.Write.
func (cl ChanLog) ExpectEmpty(t *testing.T, lines ...string) {
	t.Helper()
	/* Check for lines we want. */
	cl.Expect(t, lines...)
	/* Anything left is an error. */
	var got []string
	for 0 != len(cl) {
		select { /* Check to see if we have anything.  */
		case l, ok := <-cl: /* We do, nuts. */
			if !ok { /* Closed.  This works. */
				return
			}
			got = append(got, l)
		default: /* Empty, good. */
			return
		}
	}
	/* If we got no leftovers, life's good. */
	if 0 == len(got) {
		return
	}
	/* Tell someone about the unexpected line(s). */
	var s string
	if 1 < len(got) {
		s = "s"
	}
	t.Errorf(
		"Unexpected leftover log line%s:\n%s",
		s,
		strings.Join(got, "\n"),
	)
}
