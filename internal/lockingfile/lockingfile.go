// Package lockingfile - os.File which locks during writes
package lockingfile

/*
 * lockingfile.go
 * os.File which locks during writes
 * By J. Stuart McMurray
 * Created 20251212
 * Last Modified 20261016
 */

import (
	"cmp"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"uuid"
)

var (
	// specialFDs map filenames to file descriptor numbers, for if OpenFile
	// is passed a filename that can't be opened normally.
	specialNames = map[string]uintptr{
		"/dev/stdin":  0,
		"/dev/stdout": 1,
		"/dev/stderr": 2,
	}

	// fdNameRE is used to extract a file descriptor number from a name
	// which indicates an already-opened file.
	fdNameRE = regexp.MustCompile(`^\/dev\/fd\/(\d+)$`)
)

// File wraps an os.File with a Write that flock(2)s the file during writes.
type File struct {
	*os.File
}

// OpenFile wraps [os.OpenFile] to return a [File].
//
// As a special case, if the named file is unable to be opened with
// [os.OpenFile], name is /dev/fd/N, where N is a number, or /dev/stdin,
// /dev/stdout, or /dev/stderr, and name refers to a socket, possibly via
// symlinks, an [os.File] wrapping that file descriptor number will be used.
func OpenFile(name string, flag int, perm os.FileMode) (*File, error) {
	/* Ideally, just an open(2)able file. */
	f, err := os.OpenFile(name, flag, perm)
	if nil == err {
		return &File{f}, nil
	}

	/* If we're passed /dev/fd/N or /dev/std*, assume the user knows what
	he's doing. */
	if f := openNamedFD(name); nil != f {
		return f, nil
	}

	return nil, err
}

// Write wraps [os.File.Write] but holds a lock with [syscall.Flock].  If the
// file does not support flock-style locks (i.e. returns [syscall.EOPNOTSUPP])
// the write will be attempted without the lock held.
func (f *File) Write(b []byte) (int, error) {
	/* Lock the file. */
	if err := lockFile(f.File); nil != err {
		return 0, fmt.Errorf("acquiring lock: %w", err)
	}

	/* Write to it. */
	n, werr := f.File.Write(b)
	if nil != werr {
		werr = fmt.Errorf("writing: %w", werr)
	}
	f.File.Sync()

	/* Unlock. */
	var uerr error
	if uerr = unlockFile(f.File); nil != uerr {
		uerr = fmt.Errorf("releasing lock: %w", uerr)
	}

	return n, cmp.Or(werr, uerr)
}

// lockFile locks f for writing.  It blocks until the lock is held. */
func lockFile(f *os.File) error { return flock(f, syscall.LOCK_EX) }

// unlockFile unlocks f.
func unlockFile(f *os.File) error { return flock(f, syscall.LOCK_UN) }

// flock calls [syscall.Flock] on f.
func flock(f *os.File, how int) error {
	/* If we're returning an unlock error, life's easy. */
	if testing.Testing() && syscall.LOCK_UN == how {
		if rest, ok := strings.CutPrefix(
			f.Name(),
			testUnlockErrorPrefix,
		); ok {
			return errors.New(rest)
		}
	}

	/* Raw file, because f.Fd() is slower. */
	rc, err := f.SyscallConn()
	if nil != err {
		return fmt.Errorf("getting raw file: %w", err)
	}

	/* f the lock(2). */
	var serr, ierr, lerr error
	if testing.Testing() {
		if rest, ok := strings.CutPrefix(
			f.Name(),
			testRawConnErrorPrefix,
		); ok {
			ierr = errors.New(rest)
		}
	}
	if serr = rc.Control(func(fd uintptr) {
		if lerr = syscall.Flock(int(fd), how); errors.Is(
			lerr,
			syscall.EOPNOTSUPP,
		) {
			lerr = nil
		} else if nil != lerr {
			lerr = fmt.Errorf("changing lock: %w", lerr)
		}
	}); nil != serr || nil != ierr {
		serr = fmt.Errorf("calling flock: %w", cmp.Or(serr, ierr))
	}

	return cmp.Or(serr, lerr)
}

// openNamedFD returns a new File with the file descriptor indicated by name,
// which should be /dev/fd/N or one of the keys in specialNames.
// If name is not suitable, openNamedFD returns nil.
func openNamedFD(name string) *File {
	/* newFile returns a new File wrapping a new os.File synthesized from
	fd and name. */
	newFile := func(fd uintptr) *File {
		return &File{os.NewFile(fd, name)}
	}

	/* Special names are easy. */
	if fd, ok := specialNames[name]; ok {
		return newFile(fd)
	}

	/* If we're testing, removed the contrived file prefix if present. */
	if testing.Testing() {
		name = strings.TrimPrefix(name, testUnopenableDevFDPrefix)
	}

	/* Try to extract a file descriptor number. */
	ms := fdNameRE.FindStringSubmatch(name)
	if 2 != len(ms) {
		return nil
	}
	fd, err := strconv.Atoi(ms[1])
	if nil != err || (testing.Testing() && testInvalidDevFDName == name) {
		/* We tried. */
		return nil
	}

	return newFile(uintptr(fd))
}

var (
	// testInvalidDevFDName is a filename that parses as a vaild /dev/fd/N
	// file but causes openNamedFD to return nil.
	testInvalidDevFDName = "/dev/fd/123456789123456" + strconv.Itoa(rand.Int())
	// testUnopenableDevFDPrefix is a file prefix that won't (likely) be
	// openable but which we'll translate into /dev/fd/N by removing the
	// prefix.
	testUnopenableDevFDPrefix = "devfd-" + uuid.NewV4().String() + "--"
	// testRawConnErrorPrefix is a filename prefix that causes the rest of
	// the filename to be wrapped in errors.New and returned in lieu of the
	// error returned from flock's call to rc.Control.
	testRawConnErrorPrefix = "rawconn-" + uuid.NewV4().String() + "--"
	// testUnlockErrorPrefix is a filename prefix that causes the rest of
	// the filename to be wrapped in errors.New and returned from flock if
	// the how is [syscall.LOCK_UN].
	testUnlockErrorPrefix = "unlock-" + uuid.NewV4().String() + "--"
)
