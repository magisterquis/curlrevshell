package shellfuncsfile

/*
 * filter_perl_test.go
 * Tests for filter_perl.go
 * By J. Stuart McMurray
 * Created 20240707
 * Last Modified 20240728
 */

import (
	"bytes"
	_ "embed"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

// fromPerlCasesTxtar are test cases for fromPerl, in txtar format.
// Each case has two files *_have and *_want.
//
//go:embed testdata/fromperl/from_perl.txtar
var fromPerlCasesTxtar []byte

func TestFromPerl(t *testing.T) {
	/* Roll a bunch of test cases.  We'll whine later if filenames aren't
	correct. */
	testCases := make(map[string]struct{})
	files := make(map[string][]byte)
	a := txtar.Parse(fromPerlCasesTxtar)
	for _, f := range a.Files {
		/* Get the part before the final _. */
		name, _, ok := strings.Cut(f.Name, ".")
		if !ok {
			t.Fatalf("Test case name %s missing dot", f.Name)
		}
		/* Note the test case name and make it easy to find the
		file. */
		testCases[name] = struct{}{}
		files[f.Name] = f.Data
	}

	/* Test ALL the cases. */
	for n := range testCases {
		t.Run(n, func(t *testing.T) {
			/* Grab the have/want files. */
			haveName := n + ".have"
			have, ok := files[n+".have"]
			if !ok {
				t.Fatalf("Missing have file")
			}
			want, ok := files[n+".want"]
			if !ok {
				t.Fatalf("Missing want file")
			}
			/* See what we get. */
			got, err := FromPerl(haveName, bytes.NewReader(have))
			if nil != err {
				t.Fatalf("Error: %s", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf(
					"Incorrect filter output:\n"+
						"have:\n%s\n"+
						"got:\n%s\n"+
						"want:\n%s\n",
					have,
					got,
					want,
				)
			}
		})
	}
}

// cleanPerlCasesTxtar are test cases for cleanPerl, in txtar format.  Each
// case has three files: *_have, *_leadComments, and *_perl, corresponding to
// the input and two outputs of cleanPerl.
//
//go:embed testdata/fromperl/clean_perl.txtar
var cleanPerlCasesTxtar []byte

func TestCleanPerl(t *testing.T) {
	/* Roll a bunch of test cases.  We'll whine later if filenames aren't
	correct. */
	testCases := make(map[string]struct{})
	files := make(map[string][]byte)
	a := txtar.Parse(cleanPerlCasesTxtar)
	for _, f := range a.Files {
		/* Get the part before the final _. */
		name, _, ok := strings.Cut(f.Name, ".")
		if !ok {
			t.Fatalf("Test case name %s missing dot", f.Name)
		}
		/* Note the test case name and make it easy to find the
		file. */
		testCases[name] = struct{}{}
		files[f.Name] = f.Data
	}

	/* Test ALL the cases. */
	for n := range testCases {
		t.Run(n, func(t *testing.T) {
			/* Grab the have/want files. */
			have, ok := files[n+".have"]
			if !ok {
				t.Fatalf("Missing have file")
			}
			wantLeadComments, ok := files[n+".leadComments"]
			if !ok {
				t.Fatalf("Missing leadComments file")
			}
			wantPerl, ok := files[n+".perl"]
			if !ok {
				t.Fatalf("Missing perl file")
			}
			/* See what we get. */
			gotLeadComments, gotPerl := cleanPerl(string(have))
			if want := string(wantLeadComments); gotLeadComments !=
				want {
				t.Errorf(
					"Incorrect leadComments:\n"+
						"got:\n%s\n"+
						"want:\n%s\n",
					gotLeadComments,
					want,
				)
			}
			if want := string(wantPerl); gotPerl !=
				want {
				t.Errorf(
					"Incorrect Perl:\n"+
						"got:\n%s\n"+
						"want:\n%s\n",
					gotPerl,
					want,
				)
			}
		})
	}
}
