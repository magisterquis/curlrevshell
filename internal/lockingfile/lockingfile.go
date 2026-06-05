// Package lockingfile - os.File which locks during writes
package lockingfile

/*
 * lockingfile.go
 * os.File which locks during writes
 * By J. Stuart McMurray
 * Created 20251212
 * Last Modified 20251212
 */

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

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

// Write wraps [os.File.Write] but holds a lock with [syscall.Flock].  If the
// file does not support flock-style locks (i.e. returns [syscall.EOPNOTSUPP])
// the write will be attempted without the lock held.
func (f *File) Write(b []byte) (int, error) {
	/* Lock the file. */
	if err := lockFile(&f.File); nil != err {
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
	if uerr = unlockFile(&f.File); nil != uerr {
		uerr = fmt.Errorf("releasing lock: %w", uerr)
	}

	return n, errors.Join(werr, uerr)
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
	var serr, lerr error
	if serr = rc.Control(func(fd uintptr) {
		if lerr = syscall.Flock(int(fd), how); errors.Is(
			lerr,
			syscall.EOPNOTSUPP,
		) {
			lerr = nil
		} else if nil != lerr {
			lerr = fmt.Errorf("changing lock: %w", lerr)
		}
	}); nil != serr {
		serr = fmt.Errorf("calling flock: %w", serr)
	}

	return errors.Join(serr, lerr)
}
