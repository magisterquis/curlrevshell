package testdatareader

/*
 * testdatareader_test.go
 * Tests for testdatareader.go
 * By J. Stuart McMurray
 * Created 20260815
 * Last Modified 20260815
 */

import (
	"embed"
	"errors"
	"io/fs"
	"path"
	"testing"

	"golang.org/x/tools/txtar"
)

//go:embed testdata
var testdataFS embed.FS

// txtarFS is an archive which will be unpacked into a filesystem, but isn't
// an embedded filesystem itself.
//
//go:embed testdata.txtar
var tdTxtar []byte

// testdata reads test-specific data from the testdaat directory.
var testdata = Reader{FS: testdataFS}

// mustTDFile gets a test-specific file from testdataFS.
var mustTDFile = Reader{FS: testdataFS}.MustReadFile

// Can we do reader things?
func TestReader(t *testing.T) {
	/* Embedded non-embedded test data. */
	taFS, err := txtar.FS(txtar.Parse(tdTxtar))
	if nil != err {
		t.Errorf("Error turning txtar data into fs.FS: %v", err)
	}

	var (
		tdR = Reader{FS: testdataFS}
		tdT = Reader{FS: taFS}
	)

	for n, c := range map[string]struct {
		r    Reader
		p    []string /* Path */
		want string
	}{"single_filename": {
		r:    tdR,
		p:    []string{"data"},
		want: "kittens",
	}, "path_with_slash": {
		r:    tdR,
		p:    []string{"foo/bar/tridge/file"},
		want: "moose",
	}, "multiple_path_elements": {
		r:    tdR,
		p:    []string{"p1", "p2a/p2b", "f"},
		want: "zoomies!",
	}, "txtar_archive_no_path": {
		r:    tdT,
		p:    []string{"fn"},
		want: "txtar, no path :)\n",
	}, "txtar_archive/path": {
		r:    tdT,
		p:    []string{"pfn"},
		want: "txtar, path :)\n",
	}, "base_dir/dot": {
		r:    Reader{BaseDir: ".", FS: taFS},
		p:    []string{"fn"},
		want: "Base directory is a dot. :)\n",
	}, "no_filename": {
		r:    tdR,
		p:    []string{},
		want: "But it is a file",
	}} {
		t.Run(n, func(t *testing.T) {
			if got := c.r.MustReadFile(t, c.p...); got != c.want {
				t.Errorf(
					"Incorrect file contents\n"+
						" got: %q\n"+
						"want: %q",
					got,
					c.want,
				)
			}
		})

	}
}

// Can we read from the test's directory with no subdirectory from t.Run?
func TestReader_NoSubdirectory(t *testing.T) {
	var (
		fn   = "fn"
		want = "It worked."
	)
	if got := (Reader{FS: testdataFS}).MustReadFile(t, fn); got != want {
		t.Errorf(
			"Incorrect file contents\n"+
				" got: %q\n"+
				"want: %q",
			got,
			want,
		)
	}
}

// Can we read a single file that's the test name?
func TestReader_TestNameFile(t *testing.T) {
	var want = "Yup."
	if got := (Reader{FS: testdataFS}).MustReadFile(t); got != want {
		t.Errorf(
			"Incorrect file contents\n"+
				" got: %q\n"+
				"want: %q",
			got,
			want,
		)
	}
}

// Can we read a file with a nil t?
func TestReader_NoTestName(t *testing.T) {
	var (
		fn   = "no_test_name"
		want = "nemo"
	)
	if got := mustTDFile(nil, fn); got != want {
		t.Errorf(
			"Incorrect file contents\n"+
				" got: %q\n"+
				"want: %q",
			got,
			want,
		)
	}
}

// Can we read using package-level readers, as in the doc for Reader?
func TestReader_PackageLevelVariables(t *testing.T) {
	/* ok checks if get() == want in a subtest named want.  get will only
	be passed t. */
	ok := func(get func(*testing.T, ...string) string, want string) {
		t.Run(want, func(t *testing.T) {
			if got := get(t); got != want {
				t.Errorf(
					"Incorrect file contents\n"+
						" got: %q\n"+
						"want: %q",
					got,
					want,
				)
			}
		})
	}
	ok(testdata.MustReadFile, "package_level_Reader")
	ok(mustTDFile, "package_level_Function")
}

// Do we panic on error?
func TestReaderMustReadFile_Panic(t *testing.T) {
	defer func() {
		/* Catch the panic. */
		v := recover()
		if nil == v {
			t.Fatalf("Did not panic")
		}
		/* Should get an error back. */
		err, ok := v.(ReadError)
		if !ok {
			t.Fatalf(
				"Recovered value type incorrect\n"+
					" got: %T\n"+
					"want: %T",
				v,
				err,
			)
		}
		/* And it should be the right sort of error. */
		if got, want := err, fs.ErrNotExist; !errors.Is(got, want) {
			t.Errorf(
				"Panic returned incorrect error\n"+
					" got: %v\n"+
					"want: %v",
				got,
				want,
			)
		}
		/* With the right path. */
		if got, want := err.Path,
			path.Join(DefaultBaseDir, t.Name()); got != want {
			t.Errorf(
				"Incorrect path in error\n got: %s\nwant: %s",
				got,
				want,
			)
		}
	}()
	mustTDFile(t)
}
