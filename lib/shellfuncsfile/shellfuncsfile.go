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
 * Last Modified 20240728
 */

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"golang.org/x/exp/maps"
)

// A Filter converts read bytes into one or more shell functions, as
// appropriate.
type Filter func(filename string, r io.Reader) ([]byte, error)

// defaultFilters are the filters built-in to the library.  They are used in
// NewDefaultConverter.  Don't forget to update NewDefaultConverter's comment
// when one is added here.
var defaultFilters = map[string]Filter{
	"*.pl":   FromPerl,
	"*.sh":   FromShell,
	"*.subr": FromShell,
}

// errNoConverter is returned by Converter.fromReader if the given filename
// didn't match any converter's pattern.
var errNoConverter = errors.New("no converter for file")

// A Converter converts files or directories into shell functions.  A zero
// converter is valid, but has no filters.
type Converter struct {
	// FS is the converter's underlying source of files.  If unset, the
	// filesystem (e.g. [os.Open]) will be used.
	// FS must not be modified during a call to Converter.From.
	FS interface {
		fs.FS
	}

	// AddListFunction adds an additional shell function to From's output
	// which lists other functions.  See the [GenFuncList] for more info.
	// AddListFunction must not be modified during a call to
	// Converter.From.
	AddListFunction bool

	// filters maps a glob to a filter.
	filtersL sync.RWMutex
	filters  map[string]Filter
}

// NewDefaultConverter returns a new converter with the default set of filters,
// which are the package-level From* functions.  The default filters and
// corresponding file name extensions are:
//   - FromPerl:  *.pl
//   - FromShell: *.sh *.subr
func NewDefaultConverter() *Converter {
	return &Converter{filters: maps.Clone(defaultFilters)}
}

// SetFilter sets the filter for a given file name glob pattern, which will be
// matched with [filepath.Match].
// Set a nil filter to disable handling that pattern.
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

// fromDirectory converts files in source which have a converter and
// concatenates the result.
func (c *Converter) fromDirectory(source string) ([]byte, error) {
	/* Got a directory.  We'll want to operate in just that. */
	var sfs fs.FS
	if nil == c.FS {
		sfs = os.DirFS(source)
	} else {
		var err error
		if sfs, err = fs.Sub(c.FS, source); nil != err {
			return nil, fmt.Errorf(
				"using subtree rooted at %s: %s",
				source,
				err,
			)
		}
	}

	/* Copy filters, to avoid races. */
	c.filtersL.RLock()
	filters := maps.Clone(c.filters)
	c.filtersL.RUnlock()

	/* Get the appropriate files in the directory. */
	var fileNames []string
	patterns := maps.Keys(filters)
	slices.Sort(patterns)
	for _, pattern := range patterns {
		ms, err := fs.Glob(sfs, pattern)
		if nil != err {
			return nil, fmt.Errorf(
				"finding files which match %q: %s",
				pattern,
				err,
			)
		}
		fileNames = append(fileNames, ms...)
	}
	slices.Sort(fileNames)
	fileNames = slices.Compact(fileNames)

	/* Convert each file, appending to one big buffer. */
	var buf bytes.Buffer
	for _, fileName := range fileNames {
		/* "Real" filename */
		fn := filepath.Join(source, fileName)
		/* Don't care about non-regular files. */
		if fi, err := fs.Stat(sfs, fileName); nil != err {
			return nil, fmt.Errorf(
				"getting info for %s: %w",
				fn,
				err,
			)
		} else if !fi.Mode().IsRegular() {
			continue
		}
		/* Convert the file. */
		if err := func() error {
			/* Slurp the file. */
			f, err := sfs.Open(fileName)
			if nil != err {
				return fmt.Errorf(
					"opening %s: %w",
					fn,
					err,
				)
			}
			defer f.Close()
			/* Convert it. */
			b, err := c.fromReader(f, fileName, filters)
			if nil != err {
				return fmt.Errorf(
					"converting %s: %w",
					fn,
					err,
				)
			}
			/* Save the result. */
			_, err = buf.Write(b)
			return err
		}(); nil != err {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// fromSingleFile converts a single file.
func (c *Converter) fromSingleFile(name string) ([]byte, error) {
	/* Slurp the file. */
	var (
		b   []byte
		err error
	)
	if nil == c.FS {
		b, err = os.ReadFile(name)
	} else {
		b, err = fs.ReadFile(c.FS, name)
	}
	if nil != err {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	/* Convert it. */
	c.filtersL.RLock()
	filters := maps.Clone(c.filters)
	c.filtersL.RUnlock()
	res, err := c.fromReader(bytes.NewReader(b), name, filters)
	if errors.Is(err, errNoConverter) { /* That's ok. */
		res = b
	} else if nil != err {
		return nil, fmt.Errorf("converting: %w", err)
	}

	return res, err
}

// fromReader converts b using the first filter which matches fn.
// The filters must be passed in explicitly, to avoid a race between AddFilter
// and FromDirectory.
func (c *Converter) fromReader(
	r io.Reader,
	fn string,
	filters map[string]Filter,
) ([]byte, error) {
	/* Get the right filter. */
	patterns := maps.Keys(filters)
	slices.Sort(patterns)
	var (
		f              Filter
		matchedPattern string
	)
	for _, pattern := range patterns {
		ok, err := filepath.Match(pattern, filepath.Base(fn))
		if nil != err {
			return nil, fmt.Errorf(
				"error matching %s against %s: %w",
				pattern,
				fn,
				err,
			)
		}
		if ok {
			f = filters[pattern]
			matchedPattern = pattern
			break
		}
	}
	if nil == f { /* No filter for this file, that's ok. */
		return nil, errNoConverter
	}

	/* Convert the file. */
	b, err := f(fn, r)
	if nil != err {
		return nil, fmt.Errorf(
			"running filter for %s: %w",
			matchedPattern,
			err,
		)
	}

	/* Make sure we have a newline. */
	if 0 != len(b) && '\n' != b[len(b)-1] {
		b = append(b, '\n')
	}
	return b, nil
}

// From takes as its source either a file or a directory and returns a slice of
// bytes containing shell functions.
//
// If the source is a file, From returns its contents, possibly converted
// should any [Filter]'s patterns match.
//
// If the source is a directory, From filters and concatenates all of the
// regular files in the directory, sorted lexicographically by name,  whose
// names do not start with a period, adding newlines as needed.
//
// Filter patterns will be tried in lexicographical order.
func (c *Converter) From(source string) ([]byte, error) {
	/* If this is a single file, life's easy. */
	var (
		fi  fs.FileInfo
		b   []byte
		err error
	)
	if nil == c.FS {
		fi, err = os.Stat(source)
	} else {
		fi, err = fs.Stat(c.FS, source)
	}
	if nil != err {
		return nil, fmt.Errorf("unable to get file info: %w", err)
	}
	switch {
	case fi.IsDir():
		if b, err = c.fromDirectory(source); nil != err {
			return nil, fmt.Errorf(
				"converting files in directory: %w",
				err,
			)
		}
	case fi.Mode().IsRegular():
		if b, err = c.fromSingleFile(source); nil != err {
			return nil, fmt.Errorf("converting file: %w", err)
		}
	default:
		return nil, InvalidTypeError{FI: fi}
	}

	/* Add a list if we're meant to. */
	if c.AddListFunction {
		lf, err := GenFuncList(string(b))
		if nil != err {
			return nil, fmt.Errorf(
				"generating function-listing function: %w",
				err,
			)
		}
		b = append(b, '\n')
		b = append(b, lf...)
	}

	return b, nil
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
