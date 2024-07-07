package shellfuncsfile

/*
 * filter_perl_test.go
 * Tests for filter_perl.go
 * By J. Stuart McMurray
 * Created 20240707
 * Last Modified 20240707
 */

import (
	"bytes"
	_ "embed"
	"testing"

	"golang.org/x/tools/txtar"
)

func TestCleanPerl(t *testing.T) {
	for name, c := range map[string]struct {
		have string
		want string
	}{"simple": {
		have: `print "Kittens\n";`,
		want: `print "Kittens\n";`,
	}, "shebang_and_header_and_spaces": {
		have: `#!/usr/bin/env perl
#
# A header
# Another header

# A comment
print "Kittens\n"

`,
		want: `print "Kittens\n"`,
	}} {
		t.Run(name, func(t *testing.T) {
			if got := string(cleanPerl(
				[]byte(c.have),
			)); got != c.want {
				t.Errorf(
					"have:\n%s\ngot:\n%s\nwant:\n%s",
					c.have,
					got,
					c.want,
				)
			}
		})
	}
}

// fromPerlCaseTxtar are test cases for fromPerl, in txtar format.  First
// line of each file is the want, rest is the have.
//
//go:embed testdata/fromperl/cases.txtar
var fromPerlCaseTxtar []byte

func TestFromPerl(t *testing.T) {
	for _, f := range txtar.Parse(fromPerlCaseTxtar).Files {
		t.Run(f.Name, func(t *testing.T) {
			/* Extract the case from the file. */
			want, have, ok := bytes.Cut(f.Data, []byte("\n"))
			if !ok {
				t.Fatalf(
					"Case does not have at least one line",
				)
			}
			want = append(want, []byte("\n")...)

			/* Does it work? */
			if got, err := FromPerl(
				f.Name,
				bytes.NewReader(have),
			); nil != err {
				t.Fatalf("Error: %s", err)
			} else if !bytes.Equal(got, want) {
				t.Errorf(
					"have:\n%s\ngot:\n%s\nwant:\n%s",
					have,
					got,
					want,
				)
			}
		})
	}
}
