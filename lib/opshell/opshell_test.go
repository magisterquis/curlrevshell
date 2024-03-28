package opshell

/*
 * opshell_test.go
 * Tests for opshell.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240328
 */

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/term"
)

// timestampRE matches a timestamp a the beginning of a line.
var timestampRE = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}.\d{3} `)

// testEscapeCodes are ANSI escape codes.
var testEscapeCodes = &term.EscapeCodes{
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
		want:   "\x1b[31mkittens 123 true\n\x1b[0m",
	}, {
		color:  ColorNone,
		format: "%s",
		args:   []any{"kittens"},
		want:   "kittens\n",
	}, {
		color:  ColorRed,
		format: "kittens %d %t\n\n\n\n",
		args:   []any{123, true},
		want:   "\x1b[31mkittens 123 true\n\n\n\n\x1b[0m",
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
		want:   "\x1b[31mkittens 123 true\n\x1b[0m",
		noTS:   true,
	}, {
		color:  ColorNone,
		format: "%s",
		args:   []any{"kittens"},
		want:   "kittens\n",
		noTS:   true,
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
