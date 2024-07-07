package shellfuncsfile

/*
 * shellfuncsfile_test.go
 * Turn a file or directory into shell functions
 * By J. Stuart McMurray
 * Created 20240706
 * Last Modified 20240707
 */

import (
	"bytes"
	"embed"
	"io/fs"
	"strings"
	"testing"
)

//go:embed testdata
var testdata embed.FS

func TestConverterFrom(t *testing.T) {
	/* Converter we're testing. */
	conv := NewDefaultConverter()

	/* Load with our test data. */
	var err error
	if conv.FS, err = fs.Sub(
		testdata,
		"testdata/converter/from",
	); nil != err {
		t.Fatalf("Getting test data directory: %s", err)
	}

	/* Work out our test cases. */
	wantSuffix := "_want"
	wantFNs, err := fs.Glob(conv.FS, "*"+wantSuffix)
	if nil != err {
		t.Fatalf("Getting test cases: %s", err)
	}

	/* Make sure each wanted shell functions file works. */
	for _, wantFN := range wantFNs {
		sourceFN := strings.TrimSuffix(wantFN, wantSuffix)
		t.Run(sourceFN, func(t *testing.T) {
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
