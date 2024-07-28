package shellfuncsfile

/*
 * funclist_test.go
 * Tests for funclist.go
 * By J. Stuart McMurray
 * Created 20240728
 * Last Modified 20240728
 */

import (
	"bytes"
	_ "embed"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

// genFuncListCasesTxtar are test cases for fromPerl, in txtar format.
// Each case has two files *_have and *_want.
//
//go:embed testdata/funclist/cases.txtar
var genFuncListCasesTxtar []byte

func TestGenFuncList(t *testing.T) {
	/* Roll a bunch of test cases.  We'll whine later if filenames aren't
	correct. */
	testCases := make(map[string]struct{})
	files := make(map[string][]byte)
	a := txtar.Parse(genFuncListCasesTxtar)
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
			want, ok := files[n+".want"]
			if !ok {
				t.Fatalf("Missing want file")
			}
			/* See what we get. */
			got, err := GenFuncList(string(have))
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
