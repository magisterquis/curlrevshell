package uu

/*
 * uu_test.go
 * Tests for uu.go
 * By J. Stuart McMurray
 * Created 20240928
 * Last Modified 20240928
 */

import (
	"bytes"
	"embed"
	"io/fs"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

// testdata holds the test cases from the testdata directory
//
//go:embed testdata/*.txtar
var testdata embed.FS

// testC is a single test case.  It's fields correspond to the similarly-named
// files in the txtar archives.
type testC struct {
	Enc             []byte
	Dec             []byte
	DecErr          string
	NoEncode        bool
	NoMaxEncodedLen bool
}

func Test(t *testing.T) {
	/* Remove the directory prefix. */
	tdfs, err := fs.Sub(testdata, "testdata")
	if nil != err {
		t.Fatalf("Removing directory prefix: %s", err)
	}

	/* Work out our test cases. */
	testCs := make(map[string]testC)
	fs.WalkDir(tdfs, ".", func(
		fn string,
		de fs.DirEntry,
		err error,
	) error {
		/* Errors shouldn't happen. */
		if nil != err {
			return err
		}
		/* Don't care about directories. */
		if de.IsDir() {
			return nil
		}
		/* Make sure we don't have a case with this name already. */
		cn := strings.TrimSuffix(fn, ".txtar")
		if _, ok := testCs[cn]; ok {
			t.Errorf("Duplicate test case name: %s", cn)
			return nil
		}
		/* Extract the test data. */
		tb, err := fs.ReadFile(tdfs, fn)
		if nil != err {
			t.Errorf("Error reading test case %s: %s", fn, err)
			return nil
		}
		a := txtar.Parse(tb)
		var tc testC
		for _, f := range a.Files {
			/* Get the file bits. */
			switch f.Name {
			case "enc":
				tc.Enc = f.Data
			case "dec":
				tc.Dec = f.Data
			case "decerr":
				tc.DecErr = strings.TrimSpace(string(f.Data))
			default:
				t.Errorf("Unknown file %s in %s", f.Name, fn)
				return nil
			}
			/* Parse directives. */
			for _, d := range strings.Split(
				string(a.Comment),
				"\n",
			) {
				if "" == d {
					continue
				}
				switch d {
				case "noencode":
					tc.NoEncode = true
				case "nomaxenclen":
					tc.NoMaxEncodedLen = true
				default:
					t.Errorf(
						"Unknown directive %s in %s",
						d,
						fn,
					)
					return nil
				}
			}
		}
		testCs[cn] = tc
		return nil
	})

	/* Actually test the tests. */
	for n, tc := range testCs {
		switch {
		/* Normal encode/decode */
		case nil != tc.Enc && nil != tc.Dec && "" == tc.DecErr:
			t.Run(n, func(t *testing.T) { testEncDec(t, tc) })
		case nil != tc.Enc && nil == tc.Dec && "" != tc.DecErr:
			t.Run(n, func(t *testing.T) { testDecErr(t, tc) })
		default:
			t.Errorf("Invalid test case %s", n)
		}
	}
}

// testEncDec tests that tc.Enc and tc.Dec encode/decode to each other.
func testEncDec(t *testing.T, tc testC) {
	t.Run("encode", func(t *testing.T) {
		if tc.NoEncode {
			t.Skip("noencode set")
		}
		enc := AppendEncode(nil, tc.Dec)
		if !bytes.Equal(enc, tc.Enc) {
			t.Errorf(
				"Encoded data incorrect:\ngot:\n%s\nwant:\n%s",
				enc,
				tc.Enc,
			)
		}
	})

	dec, err := AppendDecode(nil, tc.Enc)
	if nil != err {
		t.Fatalf("Decode error: %s", err)
	}
	if !bytes.Equal(dec, tc.Dec) {
		t.Errorf(
			"Decoded data incorrect:\ngot:\n%q\nwant:\n%q",
			dec,
			tc.Dec,
		)
	}

	t.Run("MaxEncodedLen", func(t *testing.T) {
		if tc.NoMaxEncodedLen {
			t.Skip("nomaxenclen set")
		}
		if got := MaxEncodedLen(tc.Dec); got < len(tc.Enc) {
			t.Errorf(
				"MaxEncodedLen too small: got:%d real:%d",
				got,
				len(tc.Enc),
			)
		}
	})

	if got := MaxDecodedLen(tc.Enc); got < len(tc.Dec) {
		t.Errorf(
			"MaxDecodedLen too small: got:%d real:%d",
			got,
			len(tc.Dec),
		)
	}
}

// testDecErr tests that the error from decoding tc.Enc is correct.
func testDecErr(t *testing.T, tc testC) {
	dec, err := AppendDecode(nil, tc.Enc)
	if nil == err {
		t.Fatalf("Got unexpected success: %q", dec)
	}
	if got := err.Error(); got != tc.DecErr {
		t.Fatalf(
			"Incorrect error:\n got: %s\nwant: %s",
			got,
			tc.DecErr,
		)
	}
}
