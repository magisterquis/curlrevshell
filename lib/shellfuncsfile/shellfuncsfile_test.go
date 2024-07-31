package shellfuncsfile

/*
 * shellfuncsfile_test.go
 * Turn a file or directory into shell functions
 * By J. Stuart McMurray
 * Created 20240706
 * Last Modified 20240731
 */

import (
	"bytes"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//go:embed testdata
var testData embed.FS

// converterFromWantListFunc is a string which, if found in a want filename,
// indicates we want a list function as well.
const converterFromWantListFunc = "_list_"

func TestConverterFrom(t *testing.T) {
	tfs, err := fs.Sub(testData, "testdata/converter/from")
	if nil != err {
		t.Fatalf("Failed to get cases directory: %s", err)
	}
	/* Work out our test cases. */
	wantSuffix := "_want"
	wantFNs, err := fs.Glob(tfs, "*"+wantSuffix)
	if nil != err {
		t.Fatalf("Getting test cases: %s", err)
	}

	/* Make sure each wanted shell functions file works. */
	for _, wantFN := range wantFNs {
		sourceFN := strings.TrimSuffix(wantFN, wantSuffix)
		t.Run(sourceFN, func(t *testing.T) {
			/* Converter we're testing. */
			conv := NewDefaultConverter()
			conv.FS = tfs

			/* Switch on the list function if we're meant to. */
			conv.AddListFunction = strings.Contains(
				wantFN,
				converterFromWantListFunc,
			)

			/* Do the conversion. */
			want, err := fs.ReadFile(conv.FS, wantFN)
			if nil != err {
				t.Fatalf("Slurping %s: %s", wantFN, err)
			}
			got, err := conv.From(sourceFN)
			if nil != err {
				t.Fatalf("Converter.From failed: %s", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf(
					"Incorrect functions\n"+
						"source: %s\n"+
						"got:\n%s\n"+
						"want:\n%s\n",
					sourceFN,
					got,
					want,
				)
			}
		})
	}

}

// TestConverterFromReader_SubdirName makes sure we can handle files which
// aren't in the same directory.
func TestConverterFromReader_SubdirName(t *testing.T) {
	/* Roll our test file. */
	fn := filepath.Join(t.TempDir(), "foo.sh")
	have := "foo() { date; }"
	want := have + "\n"
	if err := os.WriteFile(fn, []byte(have), 0600); nil != err {
		t.Fatalf("Error writing to temporary file %s: %s", fn, err)
	}
	/* Try to convert it. */
	conv := NewDefaultConverter()
	b, err := conv.From(fn)
	if nil != err {
		t.Fatalf("From error: %s", err)
	}
	if got := string(b); got != want {
		t.Fatalf(
			"From failed:\nhave:\n%s\ngot:\n%s\nwant:\n%s",
			have,
			got,
			want,
		)
	}
}

// TestConverterFrom_Multiple tests converting multiple files.  In each
// subdirectory of testdata/converter/from_multiple, the files named have_* are
// passed to from, sorted lexicographically, and compared to the file named
// want.
func TestConverterFrom_Multiple(t *testing.T) {
	/* Get the test case directory. */
	td, err := fs.Sub(testData, "testdata/converter/from_multiple")
	if nil != err {
		t.Fatalf("Error getting test cases directory: %s", err)
	}
	/* And the test cases. */
	des, err := fs.ReadDir(td, ".")
	if nil != err {
		t.Fatalf("Error getting test cases: %s", err)
	}
	/* Test ALL the directories. */
	for _, de := range des {
		t.Run(de.Name(), func(t *testing.T) {
			sd, err := fs.Sub(td, de.Name())
			if nil != err {
				t.Fatalf(
					"Error getting test directory from "+
						"%s: %s",
					de.Name(),
					err,
				)
			}
			/* Get want. */
			want, err := fs.ReadFile(sd, "want")
			if nil != err {
				t.Fatalf("Error reading want: %s", err)
			}
			/* Convert the haves. */
			ns, err := fs.Glob(sd, "have_*")
			if nil != err {
				t.Fatalf("Getting list of want files: %s", err)
			}
			conv := NewDefaultConverter()
			conv.FS = sd
			conv.AddListFunction = true
			got, err := conv.From(ns...)
			if nil != err {
				t.Fatalf("From failed with error: %s", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf(
					"From failed:\nfiles: %v\n\n"+
						"got:\n%s\nwant:\n%s",
					ns,
					got,
					want,
				)
			}

		})
	}
}

// mustReadTestdataFile returns the bytes of the file fn in testdata.  It calls
// t.Fatalf on failure.
func mustReadTestdataFile(t *testing.T, fn string) []byte {
	b, err := fs.ReadFile(testData, fn)
	if nil != err {
		t.Fatalf("Failed to extract %s from testData: %s", fn, err)
	}
	return b
}
