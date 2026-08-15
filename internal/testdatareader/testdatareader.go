// Package testdatareader - Reader of Test Data, and other assorted test things
//
// [Reader] reads data from an [fs.FS], normally an [embed.FS] containing a
// package's testdata directory, e.g.
//
//	//go:embed testdata
//	var testdataFS embed.FS
//
// Followed by package-level access to the Reader, e.g. one of the following:
//
//	// testdata reads test-specific data from the testdaat directory.
//	var testdata = testdatareader.Reader{FS: testdataFS}
//
//	// mustTDFile gets a test-specific file from testdataFS.
//	var mustTDFile = testdatareader.Reader{FS: testdataFS}.MustReadFile
package testdatareader

/*
 * testdatareader.go
 * Reader of Test Data, and other assorted test things
 * By J. Stuart McMurray
 * Created 20260815
 * Last Modified 20260815
 */

import (
	"cmp"
	"io/fs"
	"path"
	"testing"
)

// DefaultBaseDir is used by [Reader.MustReadFile] if [Reader.BaseDir] is
// unset.
const DefaultBaseDir = "testdata"

// Reader reads data from an [fs.FS], normally an [embed.FS] containing a
// package's testdata directory, e.g.
type Reader struct {
	// BaseDir is the directory under which MustReadFile looks for files.
	// By default this is DefaultBaseDir, but may be set to "." for
	// non-testdata use-cases. */
	BaseDir string

	// FS is the filesystem from which MustReadFile reads files.  This is
	// normally the testdata directory as an embed.FS but can be any
	// fs.FS.
	FS fs.FS
}

// MustReadFile returns the contents of a file in r.FS or panics on error.
// The file path is composed of r.BaseDir, the test's name if t is not nil, and
// elem... joined with [path.Join].  If r.BaseDir is the empty string,
// DefaultBaseDir is used.
//
// For example, if t refers to a test named TestFoo, and r.BaseDir is "mydir",
// r.MustReadFile(t, "bar/tridge", "baaz")
// would return the contents of
// mydir/TestFoo/bar/tridge/baaz.
func (r Reader) MustReadFile(t *testing.T, elem ...string) string {
	/* Work out the path to the file. */
	parts := make([]string, 0, 2+(len(elem)))
	parts = append(parts, cmp.Or(r.BaseDir, DefaultBaseDir))
	if nil != t {
		parts = append(parts, t.Name())
	}
	parts = append(parts, elem...)
	fn := path.Join(parts...)

	/* Grab the file or panic. */
	b, err := fs.ReadFile(r.FS, fn)
	if nil != err {
		panic(ReadError{Path: fn, Err: err})
	}
	return string(b)
}
