package opshell

/*
 * opshell_test.go
 * Tests for opshell.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20241203
 */

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/goxterm"
)

// timestampRE matches a timestamp a the beginning of a line.
var timestampRE = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}.\d{3} `)

// testEscapeCodes are ANSI escape codes.
var testEscapeCodes = &goxterm.EscapeCodes{
	Black:   []byte("\x1b[30m"),
	Red:     []byte("\x1b[31m"),
	Green:   []byte("\x1b[32m"),
	Yellow:  []byte("\x1b[33m"),
	Blue:    []byte("\x1b[34m"),
	Magenta: []byte("\x1b[35m"),
	Cyan:    []byte("\x1b[36m"),
	White:   []byte("\x1b[37m"),
	Reset:   []byte("\x1b[0m"),
}

// newTestShell returns a new Shell, ready for use.  The net.Conn plays the
// role of stdio.
func newTestShell(t *testing.T) (
	net.Conn,
	chan<- string,
	<-chan CLine,
	*Shell,
) {
	var (
		rw, sc              = net.Pipe()
		ich                 = make(chan<- string, 1024)
		och                 = make(<-chan CLine, 1024)
		shell, cleanup, err = NewWrapping(
			ich,
			och,
			"",
			true,
			nil,
			"",
			sc,
			sc,
		)
	)
	t.Cleanup(cleanup)
	if nil != err {
		t.Fatalf("Could not make test shell: %s", err)
	}
	return rw, ich, och, shell
}

func TestLogf(t *testing.T) {
	for _, c := range []struct {
		color  Color
		noTS   bool
		format string
		args   []any
		want   string
	}{{
		color:  ColorRed,
		format: "kittens %d %t",
		args:   []any{123, true},
		want:   "\x1b[31mkittens 123 true\x1b[0m\n",
	}, {
		color:  ColorNone,
		format: "%s",
		args:   []any{"kittens"},
		want:   "kittens\n",
	}, {
		color:  ColorRed,
		format: "kittens %d %t\n\n\n\n",
		args:   []any{123, true},
		want:   "\x1b[31mkittens 123 true\x1b[0m\n\n\n\n",
	}, {
		color:  ColorNone,
		format: "%s\n\n\n\n",
		args:   []any{"kittens"},
		want:   "kittens\n\n\n\n",
	}, {
		color:  ColorNone,
		format: "%s\n",
		args:   []any{"kittens"},
		want:   "kittens\n",
	}, {
		color:  ColorRed,
		format: "kittens %d %t",
		args:   []any{123, true},
		want:   "\x1b[31mkittens 123 true\x1b[0m\n",
		noTS:   true,
	}, {
		color:  ColorNone,
		format: "%s",
		args:   []any{"kittens"},
		want:   "kittens\n",
		noTS:   true,
	}, {
		color:  ColorRed,
		format: "kittens",
		args:   nil,
		want:   "\x1b[31mkittens\x1b[0m\n",
	}, {
		color:  ColorNone,
		format: "kittens",
		args:   nil,
		want:   "kittens\n",
	}, {
		color:  ColorRed,
		format: "kittens\n",
		args:   nil,
		want:   "\x1b[31mkittens\x1b[0m\n",
	}, {
		color:  ColorNone,
		format: "kittens\n",
		args:   nil,
		want:   "kittens\n",
	}, {
		color:  ColorRed,
		format: "kittens\n\n",
		args:   nil,
		want:   "\x1b[31mkittens\x1b[0m\n\n",
	}, {
		color:  ColorNone,
		format: "kittens\n\n",
		args:   nil,
		want:   "kittens\n\n",
	}} {
		t.Run("", func(t *testing.T) {
			b := new(bytes.Buffer)
			if _, err := logf(
				b,
				testEscapeCodes,
				c.color,
				c.noTS,
				c.format,
				c.args...,
			); nil != err {
				t.Fatalf("Error: %s", err)
			}
			got := b.String()
			/* Only remove the timestamp if we expect one, to make
			sure no timestamp messages don't have one. */
			if !c.noTS {
				gotNoTS := removeTimestamp(got)
				/* Make sure we did actually get one. */
				if gotNoTS == got {
					t.Errorf(
						"Did not get timestamp in %q",
						got,
					)
				}
				got = gotNoTS
			}
			if got != c.want {
				t.Errorf(
					"Incorrect formatted line:\n"+
						"format: %q\n"+
						"  args: %#v\n"+
						" color: %s\n"+
						"  noTS: %t\n"+
						"   got: %q\n"+
						"  want: %q",
					c.format,
					c.args,
					c.color,
					c.noTS,
					got,
					c.want,
				)
			}
		})
	}
}

// removeTimestamp removes a timestamp from a logged message.  It works even
// if there is a color prefix.
func removeTimestamp(s string) string {
	/* Find the color, if we have one. */
	var c string
	if strings.HasPrefix(s, "\x1b[") {
		var ok bool
		c, s, ok = strings.Cut(s, "m")
		if !ok { /* Weird, but not a color. */
			return c
		}
		c += "m"
	}
	/* Next bit is maybe a timestamp.  If so, remove it. */
	if ts := timestampRE.FindString(s); "" != ts {
		s = strings.TrimPrefix(s, ts)
	}
	/* Return the timestampless string, plus the color. */
	return c + s
}

func TestRemoveTimestamp(t *testing.T) {
	for _, c := range []struct {
		have string
		want string
	}{{
		have: "kittens",
		want: "kittens",
	}, {
		have: "\x1b[32m02:03:05.100 kittens \"aa\\tbb\": 123\x1b[0m",
		want: "\x1b[32mkittens \"aa\\tbb\": 123\x1b[0m",
	}, {
		have: "02:03:05.123 kittens \"aa\\tbb\": 123",
		want: "kittens \"aa\\tbb\": 123",
	}, {
		have: "02:26:23.000 kittens\n",
		want: "kittens\n",
	}} {
		t.Run(c.have, func(t *testing.T) {
			got := removeTimestamp(c.have)
			if c.want != got {
				t.Errorf(
					"Incorrect removal:\n"+
						"have: %q\n"+
						" got: %q\n"+
						"want: %q",
					c.have,
					got,
					c.want,
				)
			}
		})
	}
}

func TestShell_Smoketest(t *testing.T) { newTestShell(t) }

func TestShell_CtrlC(t *testing.T) {
	/* Make a shell and work out the message we expect. */
	rw, _, _, shell := newTestShell(t)

	/* Start the shell going. */
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	eg, ectx := ctxerrgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := shell.Do(ectx); nil != err &&
			!errors.Is(err, io.EOF) {
			return err
		}
		cancel()
		return nil
	})
	obuf := make(chan string, 1024)
	eg.Go(func() error {
		scanner := bufio.NewScanner(rw)
		for scanner.Scan() {
			l := scanner.Text()
			obuf <- l
		}
		if err := scanner.Err(); nil != err &&
			!errors.Is(err, io.ErrClosedPipe) {
			return fmt.Errorf("reading shell output: %w", err)
		}
		return nil
	})
	/* Close our terminal pipe when the shell dies. */
	eg.Go(func() error {
		<-ectx.Done()
		return rw.Close()
	})

	/* Send a Ctrl+C. */
	sendCtrlC := func(s string) {
		eg.Go(func() error {
			if _, err := rw.Write([]byte{0x03}); nil != err {
				return fmt.Errorf(
					"writing %s Ctrl+C: %w",
					s,
					err,
				)
			}
			return nil
		})
	}
	sendCtrlC("first")

	/* Get what should be the warning. */
	want := shell.WrapInColor(FirstCtrlCWarning, ColorRed)
	if got := <-obuf; got != want {
		t.Errorf(
			"Incorrect warning from shell:\n got: %q\nwant: %q",
			got,
			want,
		)
	}

	/* Send a second, which should kill the shell. */
	sendCtrlC("second")
	want = shell.WrapInColor(SecondCtrlCWarning, ColorRed)
	if got := <-obuf; got != want {
		t.Errorf(
			"Incorrect warning from shell:\n got: %q\nwant: %q",
			got,
			want,
		)
	}

	/* Make sure everything worked. */
	if err := eg.Wait(); nil != err {
		t.Errorf("Shell error: %s", err)
	}
}
