// Package shellfuncsfile takes a file or directory and turns it into shell
// functions.
//
// The general idea is to make a Converter, and every time a shell functions
// file is needed, call Converter.From to get a nice slice of bytes.  If a
// directory is passed, files in the directory are converted to shell functions
// which is handy for scripts written in other languages, e.g. Perl.
//
// Note that all of this is inherently racy.  Don't use this in situations
// where it's reasonably likely that files will change during calls to anything
// in this package.
package shellfuncsfile

/*
 * shellfuncsfile.go
 * Turn a file or directory into shell functions
 * By J. Stuart McMurray
 * Created 20240706
 * Last Modified 20240707
 */

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// A Filter converts read bytes into one or more shell functions, as
// appropriate.
type Filter func(filename string, r io.Reader) ([]byte, error)

// defaultFilters are the filters built-in to the library.  They are used in
// NewDefaultConverter.  Don't forget to update NewDefaultConverter's comment
// when one is added here.
var defaultFilters = map[string]Filter{
	"":     FromShell,
	"pl":   FromPerl,
	"sh":   FromShell,
	"subr": FromShell,
}

// A Converter converts files or directories into shell functions.  A zero
// converter is valid, but has no filters.  Converter's methods are safe for
// concurrent use by multiple goroutines.
type Converter struct {
	// FS is the converter's underlying source of files.  If unset, the
	// filesystem (e.g. [os.Open]) will be used.
	// FS must not be modified during a call to Converter.From.
	FS interface {
		fs.FS
	}

	filters  map[string]Filter
	filtersL sync.Mutex
}

// NewDefaultConverter returns a new converter with the default set of filters,
// which are the package-level From* functions.  The default filters and
// corresponding file name extensions are:
//   - FromPerl:  .pl
//   - FromShell: .sh .subr
func NewDefaultConverter() *Converter {
	return &Converter{filters: maps.Clone(defaultFilters)}
}

// SetFilter sets the filter for a given file name extension.  Set a nil filter
// to disable handling that extension.
func (c *Converter) SetFilter(ext string, filter Filter) {
	c.filtersL.Lock()
	defer c.filtersL.Unlock()
	/* If we don't have a filter, it's really a delete. */
	if nil == filter {
		delete(c.filters, ext)
		return
	}
	/* Set the filter. */
	c.filters[ext] = filter
}

// fromDirectory passes all of the files in the directory d to c.fromFile and
// concatenates the results, newline-separated.
// The original path to d should be passed as pd.
func (c *Converter) fromDirectory(pd string, d fs.ReadDirFile) ([]byte, error) {
	/* Work out what we have in the directory. */
	des, err := d.ReadDir(-1)
	if nil != err {
		return nil, fmt.Errorf("reading directory contents: %w", err)
	}

	/* Convert all the regular files. */
	var ret bytes.Buffer
	for _, de := range des {
		/* Skip hidden files. */
		if strings.HasPrefix(de.Name(), ".") {
			continue
		}
		fn := filepath.Join(pd, de.Name())
		b, err := c.fromFileName(fn)
		if nil != err {
			return nil, fmt.Errorf("converting %s: %w", fn, err)
		}
		if 0 == len(b) {
			continue
		}
		ret.Write(b)
		/* Make sure we have a newline. */
		if '\n' != b[len(b)-1] {
			ret.WriteByte('\n')
		}
	}

	/* Guess it worked? */
	return ret.Bytes(), nil
}

// fromFileName wraps opens the named file and wraps fromFile.  It returns
// (nil, nil) if the named file is not a regular file.
func (c *Converter) fromFileName(name string) ([]byte, error) {
	/* Open the file and make sure it's a regular file. */
	var (
		f   fs.File
		err error
	)
	if nil == c.FS {
		f, err = os.Open(name)
	} else {
		f, err = c.FS.Open(name)
	}
	if nil != err {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	fi, err := f.Stat()
	if nil != err {
		return nil, fmt.Errorf("describing file: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return nil, nil
	}

	/* Convert the file. */
	return c.fromFile(name, f)
}

// fromFile returns contents of the named file, possibly converted with another
// From* function according to the file name extension, as reported by
// [filepath.Ext].  The file must be a regular file.
func (c *Converter) fromFile(name string, f fs.File) ([]byte, error) {
	/* Work out what filter to use. */
	ext := strings.TrimPrefix(filepath.Ext(name), ".")
	c.filtersL.Lock()
	filter, ok := c.filters[ext]
	c.filtersL.Unlock()
	if !ok {
		return nil, fmt.Errorf("no filter for extension %q", ext)
	}

	return filter(name, f)
}

// From takes as its source either a file or a directory and returns a slice of
// bytes containing shell functions.
//
// If the source is a file, From returns its contents, possibly converted with
// A [Filter] function according to the source's file name extension as
// reported by [filepath.Ext].
//
// If the source is a directory, From filters and concatenates all of the
// regular files in the directory whose names do not start with a period,
// adding newlines as needed.
// Do not change the files in the source during a call to From.
func (c *Converter) From(source string) ([]byte, error) {
	/* Figure out what sort of file this is. */
	var (
		f   fs.File
		err error
	)
	if nil == c.FS {
		f, err = os.Open(source)
	} else {
		f, err = c.FS.Open(source)
	}
	if nil != err {
		return nil, fmt.Errorf("opening source: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if nil != err {
		return nil, fmt.Errorf("describing source: %w", err)
	}

	/* Handle as appropriate. */
	switch {
	case fi.Mode().IsDir():
		d, ok := f.(fs.ReadDirFile)
		if !ok {
			return nil, fmt.Errorf("directory unreadable")
		}
		return c.fromDirectory(source, d)
	case fi.Mode().IsRegular():
		return c.fromFile(source, f)
	default:
		return nil, InvalidTypeError{FI: fi}
	}
}

// InvalidTypeError is returned from Converter.From when the source is neither
// a regular file nor directory.
type InvalidTypeError struct {
	FI fs.FileInfo
}

// Error implements the error interface.
func (err InvalidTypeError) Error() string {
	return fmt.Sprintf("invalid type: %s", err.FI.Mode().Type())
}
