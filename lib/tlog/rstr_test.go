package tlog

/*
 * rtsr_test.go
 * Tests for rstr.go
 * By J. Stuart McMurray
 * Created 20260327
 * Last Modified 20260327
 */

import (
	"strconv"
	"strings"
	"testing"
)

// Do random-added strings work?
func TestS(t *testing.T) {
	for n, have := range map[string]string{
		"normal_string": "kittens",
		"empty_string":  "",
		"hyphen":        "-",
		"space":         " ",
		"newline":       "\n",
	} {
		t.Run(n, func(t *testing.T) {
			got := S(have)
			prefix := have + "-"
			if want := prefix; !strings.HasPrefix(got, want) {
				t.Errorf(
					"Incorrect prefix\n"+
						"have: %q\n"+
						" got: %q\n"+
						"want: %q",
					have,
					got,
					want,
				)
			}
			suffix := strings.TrimPrefix(got, prefix)
			if _, err := strconv.ParseUint(suffix, 36, 64); nil != err {
				t.Errorf(
					"Failed to parse suffix to a number\n"+
						"suffix: %q\n"+
						" error: %s",
					suffix,
					err,
				)
			}
		})
	}
}
