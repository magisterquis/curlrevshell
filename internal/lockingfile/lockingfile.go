// Package lockingfile - os.File which locks during writes
package lockingfile

/*
 * lockingfile.go
 * os.File which locks during writes
 * By J. Stuart McMurray
 * Created 20251212
 * Last Modified 20260710
 */

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// errLock indicates an unknown error occurred trying to lock a file.
var errLock = errors.New("unknown flock(2) error")

// File wraps an os.File with a Write that flock(2)s the file during writes.
type File struct {
	os.File
}

// OpenFile wraps [os.OpenFile] to return a [File].
func OpenFile(name string, flag int, perm os.FileMode) (*File, error) {
	f, err := os.OpenFile(name, flag, perm)
	if nil != err {
		return nil, err
	}
	return &File{*f}, nil
}

// Write wraps [os.File.Write] but attempts to hold a lock with
// [syscall.Flock].  Write does not return an error if the file is unable to
// be locked.
func (f *File) Write(b []byte) (int, error) {
	/* Lock the file. */
	if err := lockFile(&f.File); nil == err {
		defer unlockFile(&f.File)
	}

	/* Write to it. */
	n, werr := f.File.Write(b)
	if nil != werr {
		werr = fmt.Errorf("writing: %w", werr)
	}
	f.File.Sync()

	return n, werr
}

// lockFile locks f for writing.  It blocks until the lock is held. */
func lockFile(f *os.File) error { return flock(f, syscall.LOCK_EX) }

// unlockFile unlocks f.
func unlockFile(f *os.File) error { return flock(f, syscall.LOCK_UN) }

// flock calls [syscall.Flock] on f.
func flock(f *os.File, how int) error {
	/* Raw file, because f.Fd() is slower. */
	rc, err := f.SyscallConn()
	if nil != err {
		return fmt.Errorf("getting raw file: %w", err)
	}

	/* f the lock(2). */
	err = errLock
	rc.Control(func(fd uintptr) {
		/* If we're here, control worked. */
		if err = syscall.Flock(int(fd), how); nil != err {
			err = fmt.Errorf("changing lock: %w", err)
		}
	})

	return err
}
