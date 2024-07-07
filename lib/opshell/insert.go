package opshell

/*
 * insert.go
 * Insert a callback's returned slice to the shell
 * By J. Stuart McMurray
 * Created 202406025
 * Last Modified 20240707
 */

import (
	"crypto/sha256"
	"io"
)

// insert inserts the data returned by s.insertGen, as on Ctrl+I.
// No error is returned; all errors are handled by sending messages to the
// user.
func (s *Shell) insert() {
	/* errf logs an error to the shell. */
	errf := func(format string, args ...any) {
		s.Logf(
			ColorRed,
			false,
			format,
			args...,
		)
	}
	/* Get the bytes to insert. */
	b, err := s.insertGen()
	if nil != err {
		errf("Error working out what to insert: %s", err)
		return
	} else if 0 == len(b) {
		errf("Lazily refusing to insert 0 bytes")
		return
	}

	/* Set up to make a hash. */
	her := sha256.New()

	/* Send it off for insertion. */
	s.Logf(ColorGreen, false, "Inserting %s...", s.insertName)
	n, err := io.MultiWriter(ChanWriter(s.ich), her).Write(b)
	if nil != err {
		/* This is actually pretty unpossible. */
		errf("Error inserting %s: %s", s.insertName, err)
		return
	}
	/* All done.  Tell the user what just happened. */
	if 0 == n {
		errf("Unsure why we inserted 0 bytes")
		return
	}
	s.Logf(ColorGreen, false, "Inserted %d bytes from %s", n, s.insertName)
	s.Logf(ColorGreen, false, "SHA256: %x", her.Sum(nil))
}
