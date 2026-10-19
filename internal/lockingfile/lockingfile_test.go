package lockingfile

/*
 * lockingfile_test.go
 * Tests for lockingfile.go
 * By J. Stuart McMurray
 * Created 20251212
 * Last Modified 20261016
 */

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
)

func TestFile_Smoketest(t *testing.T) {
	/* Make a new File. */
	fn := filepath.Join(t.TempDir(), "kittens")
	f, err := OpenFile(fn, os.O_WRONLY|os.O_CREATE, 0600)
	if nil != err {
		t.Fatalf("Error creating %s: %s", fn, err)
	}
	f.Close()
}

// Can we make a new File, even if it points to a socket?
func TestFile_Socket(t *testing.T) {
	/* Pair of socket file descriptors. */
	fds, err := syscall.Socketpair(
		syscall.AF_UNIX,
		syscall.SOCK_STREAM,
		0,
	)
	if nil != err {
		t.Fatalf("Error calling socketpair: %v", err)
	}

	/* Other end of the socket pair. */
	s := os.NewFile(uintptr(fds[0]), "testsocket")
	defer s.Close()

	/* Open the socket. */
	var (
		fd = fds[1]
		fn = fmt.Sprintf("/dev/fd/%d", fd)

		ofn = fn
	)
	/* On OpenBSD, we'll add a testing prefix so the open fails. */
	if "openbsd" == runtime.GOOS {
		ofn = testUnopenableDevFDPrefix + fn
	}
	lf, err := OpenFile(ofn, os.O_WRONLY|os.O_APPEND, 0600)
	if nil != err {
		syscall.Close(fd)
		t.Fatalf("Unexpected error from OpenFile: %v", err)
	}
	defer lf.Close()
	if got, want := lf.Name(), fn; got != want {
		t.Errorf("Incorrect filename\n got: %s\nwant: %s", got, want)
	}
	if got, want := lf.Fd(), uintptr(fd); got != want {
		t.Errorf(
			"Incorrect file descriptor number\n"+
				" got: %d\n"+
				"want: %d",
			got,
			want,
		)
	}

	/* Can we write to it? */
	var (
		buf     []byte
		eg, ctx = ctxerrgroup.WithContext(t.Context())
		msg     = tlog.S("message")
	)
	eg.GoTag(ctx, "read", func(ctx context.Context) error {
		var err error
		buf, err = io.ReadAll(s)
		return err
	})
	eg.GoTag(ctx, "write", func(ctx context.Context) error {
		defer lf.Close()
		_, err := io.WriteString(lf, msg)
		return err
	})
	if err := eg.Wait(); nil != err {
		t.Errorf("Error: %v", err)
	}
	if got, want := string(buf), msg; got != want {
		t.Errorf(
			"Incorrect bytes read from write\n"+
				" got: %q\n"+
				"want: %q",
			got,
			want,
		)
	}
}

// Does File.Write work?
func TestFileWrite(t *testing.T) {
	/* Make a new File. */
	f, err := OpenFile(
		filepath.Join(t.TempDir(), "kittens"),
		os.O_WRONLY|os.O_CREATE,
		0600,
	)
	if nil != err {
		t.Fatalf("Error creating file: %s", err)
	}
	defer f.Close()

	/* Can we write ok. */
	haves := []string{"moose", "zoomies!"}
	for _, have := range haves {
		if _, err := f.Write([]byte(have)); nil != err {
			t.Fatalf("Error writing %q to file: %s", have, err)
		}
	}

	/* Can we read it back? */
	b, err := os.ReadFile(f.Name())
	if nil != err {
		t.Fatalf("Error reading file: %s", err)
	}
	if got, want := string(b), strings.Join(haves, ""); got != want {
		t.Errorf(
			"Read after writes incorrect\n"+
				"have: %q\n"+
				" got: %q\n"+
				"want: %q",
			haves,
			got,
			want,
		)
	}
}

// Do we get an error if we can't open the file?
func TestOpenFile_OpenError(t *testing.T) {
	f, err := OpenFile(t.TempDir(), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0660)
	if got, want := err, os.ErrExist; !errors.Is(got, want) {
		t.Errorf(
			"Incorect error opening extant file with O_EXCL\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
	if nil == err {
		f.Close()
	}
}

// Can we write without interleaving?  Unfortunately, there's not a great way
// to make sure all writes interleave.  But we can try a bunch of times
// anyways.
func TestFileFlock_interleavedwrites(t *testing.T) {
	const (
		/* This one's kinda hard to test without knowing if interleaved
		writes would happen in the first place.  Set this to false to
		disable using lockFile and unlockFile, which should cause this
		test to fail.  If not, increase the numbers below. */
		useLock = true

		nParallel = 8 /* Number of simultaneous writes. */
		nBlocks   = 8 /* Number of I/O blocks to write. */
	)

	/* File to which we'll write things. */
	fn := filepath.Join(t.TempDir(), "kittens")

	/* Work out its block size, in the hopes that block-sized writes will
	encourage the kernel to not write multiple writes atomically. */
	f, err := OpenFile(fn, os.O_WRONLY|os.O_CREATE, 0600)
	if nil != err {
		t.Fatalf("Error creating file: %s", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if nil != err {
		t.Fatalf("Getting file info: %s", err)
	}
	f.Close()
	sb, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf(
			"Stat returned unexpected underlying type %T",
			fi.Sys(),
		)
	}
	blockSize := int(sb.Blksize)

	/* Make sure the block is a mulitple of a uint64.  Each write will be
	a block full of writer-specific uint64's. */
	uint64sz := binary.Size(uint64(0))
	if 0 != blockSize%uint64sz {
		/* Not gonna hold an exact number of uint64s. */
		t.Fatalf(
			"Block size %d is not a multple of %d",
			blockSize,
			uint64sz,
		)
	}

	/* Try a bunch of simultaneous writes. */
	var (
		ech     = make(chan error, nParallel)
		writeNs = make(map[uint64]struct{})
		ready   = make(chan error, nParallel)
		start   = make(chan struct{})
		uerr    error /* Unlock error. */
		werr    error /* Write error. */
	)
	try := func(n int) {
		/* Open the file. */
		f, err := os.OpenFile(
			fn,
			os.O_WRONLY|os.O_APPEND|os.O_SYNC,
			0600,
		)
		if nil != err {
			ready <- fmt.Errorf("open (%d): %w", n, err)
			return
		}
		defer f.Close()
		/* When we're done, report any errors. */
		defer func() { ech <- errors.Join(werr, uerr) }()
		/* Buffer containing lots of n's. */
		nbuf := make([]byte, uint64sz)
		binary.NativeEndian.PutUint64(nbuf, uint64(n))
		wbuf := slices.Repeat(nbuf, blockSize/uint64sz)

		/* Let the main test know we're ready. */
		ready <- nil

		/* Wait for everybody else to be ready. */
		<-start

		/* Lock the file. */
		if useLock {
			if err := lockFile(f); nil != err {
				ech <- fmt.Errorf("lock (%d): %w", n, err)
				return
			}
			defer func() {
				if uerr = unlockFile(f); nil != err {
					uerr = fmt.Errorf(
						"unlock (%d): %w",
						n,
						err,
					)
				}
			}()
		}

		/* Write the blocks, many times. */
		for range nBlocks {
			if _, werr = f.Write(wbuf); nil != err {
				werr = fmt.Errorf("write (%d): %w", n, err)
				return
			}
		}
	}
	for i := range nParallel {
		go try(i)
		writeNs[uint64(i)] = struct{}{}
	}

	/* Wait for everybody to be ready. */
	nErr := 0
	for range nParallel {
		if err := <-ready; nil != err {
			nErr++
			t.Errorf("Ready error: %s", err)
		}
	}
	close(start)

	/* Wait for them all to finish. */
	for range nParallel - nErr {
		if err := <-ech; nil != err {
			t.Errorf("Write error: %s", err)
		}
	}
	if t.Failed() {
		t.FailNow()
	}

	/* Read the file, checking for interleaves and duplicates. */
	var (
		buf     = make([]byte, blockSize*nBlocks)
		nbuf    = buf[:uint64sz]
		totRead = 0
	)
	rf, err := os.Open(f.Name())
	if nil != err {
		log.Fatalf("Error opening file for reading: %s", err)
	}
	for range nParallel {
		/* Get a try()'s write. */
		nr, err := io.ReadFull(rf, buf)
		if errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf(
				"Unexpected EOF after %d/%d bytes read: %s",
				totRead+nr,
				len(buf),
				err,
			)
		} else if nil != err {
			log.Fatalf("Read error: %s", err)
		}
		totRead += nr

		/* Make sure this block's number is one we wrote and haven't
		seen before. */
		n := binary.NativeEndian.Uint64(nbuf)
		if n >= nParallel {
			t.Fatalf("Unexpectedly large number written: %d", n)
		} else if _, ok := writeNs[n]; !ok {
			t.Fatalf("Duplicate number written: %d", n)
		}
		delete(writeNs, n)

		/* Make sure the whole write was the same number. */
		for start := 0; start < len(buf); start += len(nbuf) {
			got := buf[start : start+len(nbuf)]
			if 0 != bytes.Compare(nbuf, got) {
				t.Fatalf(
					"Interleaved write at offset %d\n"+
						" got: %02x\n"+
						"want: %02x",
					totRead-len(buf)+start,
					got,
					nbuf,
				)
			}
		}
	}

	/* Make sure all of the numbers were seen. */
	for k := range writeNs {
		t.Fatalf("Didn't see %d written", k)
	}
}

// Can we write to a pipe, which doesn't support locking?
func TestFileWrite_Pipe(t *testing.T) {
	pr, pw, err := os.Pipe()
	if nil != err {
		t.Fatalf("Error allocating pipe: %s", err)
	}

	/* Make sure locking a pipe isn't supported.  Test is kinda pointless
	otherwise. */
	rc, err := pw.SyscallConn()
	if nil != err {
		t.Fatalf("Error getting raw file: %s", err)
	}
	var lerr error
	if err := rc.Control(func(fd uintptr) {
		lerr = syscall.Flock(int(fd), syscall.LOCK_EX)
	}); nil != err {
		t.Fatalf("Error calling flock: %s", err)
	} else if nil == lerr {
		t.Skipf("Locking pipes is supported")
	} else if !errors.Is(lerr, syscall.EOPNOTSUPP) {
		t.Fatalf("Unexpected lock error: %s", lerr)
	} else if err := unlockFile(pw); nil != err {
		t.Fatalf("Unexpected unlock error: %s", err)
	}

	/* Make sure we can write happily. */
	var (
		buf  []byte
		have = "kittens"
		rerr error
		wg   sync.WaitGroup
	)
	wg.Go(func() {
		buf, rerr = io.ReadAll(pr)
	})
	if _, err := (&File{pw}).Write([]byte(have)); nil != err {
		t.Fatalf("Write error: %s", err)
	} else if err := pw.Close(); nil != err {
		t.Fatalf("Error closing pipe: %s", err)
	}

	/* Make sure we could read what we wrote. */
	wg.Wait()
	if nil != rerr {
		t.Errorf("Read error: %s", rerr)
	} else if got, want := string(buf), have; got != want {
		t.Errorf(
			"Incorrect read\nhave: %s\n got: %s\nwant: %s",
			have,
			got,
			want,
		)
	}
}

// Do we get an error if we can't lock before writing?
func TestFileWrite_LockError(t *testing.T) {
	_, err := (&File{}).Write(nil)
	if got, want := err, os.ErrInvalid; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error locking nil file\n got: %v\nwant: %v",
			got,
			want,
		)
	}
}

// Do we get an error if we can't write after locking?
func TestFileWrite_WriteError(t *testing.T) {
	f, err := OpenFile(t.TempDir(), os.O_RDONLY, 0)
	if nil != err {
		t.Fatalf("Error opening temporary directory: %v", err)
	}
	_, err = f.Write([]byte("kittens"))
	if got, want := err, syscall.EBADF; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error writing to directory\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
}

// Do we get an error if we can't unlock a file after writing?
func TestFileWrite_UnlockError(t *testing.T) {
	var es = tlog.S("error")
	f, err := os.OpenFile(
		filepath.Join(t.TempDir(), "f"),
		os.O_CREATE|os.O_RDWR,
		0,
	)
	if nil != err {
		t.Fatalf("Error opening temporary file: %v", err)
	}
	defer f.Close()
	_, err = (&File{
		os.NewFile(f.Fd(), testUnlockErrorPrefix+es),
	}).Write([]byte(""))
	if got, want := err, es; errors.Unwrap(err).Error() != es {
		t.Errorf(
			"Incorrect unlock error after write\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
	runtime.KeepAlive(f)
}

// Does flock reject a nil file?
func TestFlock_NilFile(t *testing.T) {
	if got, want := flock(nil, 0), os.ErrInvalid; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error locking a nil file\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
}

// Does flock return the right error on an impossibly large file descriptor
// number?
func TestFlock_ClosedFile(t *testing.T) {
	if got, want := flock(os.NewFile(0xFFFF, t.Name()), syscall.LOCK_EX),
		syscall.EBADF; !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error locking a closed file\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
}

// Do we return an error if we can't call code on the file's file descriptor?
func TestFlock_ControlError(t *testing.T) {
	var es = tlog.S("error")
	f, err := os.Create(filepath.Join(t.TempDir(), "f"))
	if nil != err {
		t.Fatalf("Error creating file: %v", err)
	}
	defer f.Close()
	if got, want := flock(
		os.NewFile(f.Fd(), testRawConnErrorPrefix+es),
		syscall.LOCK_EX,
	), es; errors.Unwrap(got).Error() != want {
		t.Errorf(
			"Incorrect error injecting an error into Control\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
}

// Can we roll a File from a string giving a file descriptor.
func TestOpenNamedFD(t *testing.T) {
	/* check calls openNamedFD and makes sure the returned File contains an
	os.File with the given name and file descriptor number.
	If fd is -1, try makes sure nil is retuned. */
	check := func(t *testing.T, name string, fd int) {
		gotF := openNamedFD(name)

		/* Errors are easy. */
		if -1 == fd {
			if nil != gotF {
				t.Fatalf(
					"Got name:%s fd:%d, not nil",
					gotF.Name(),
					gotF.Fd(),
				)
			}
			return
		}

		/* Should have got something. */
		if nil == gotF {
			t.Fatalf("Got nil File")
		}

		/* Name and FD correct? */
		if got, want := gotF.Name(), name; got != want {
			t.Errorf(
				"Incorrect name\n got: %s\nwant: %s",
				got,
				want,
			)
		}
		if got, want := gotF.Fd(), uintptr(fd); got != want {
			t.Errorf(
				"Incorrect file descriptor number\n"+
					" got: %d\n"+
					"want: %d",
				got,
				want,
			)
		}
	}
	try := func(name string, fd int) {
		t.Run(name, func(t *testing.T) { check(t, name, fd) })
	}

	/* Do all of the special names work? */
	for name, fd := range specialNames {
		try(name, int(fd))
	}

	/* Do normal names work? */
	for name, fd := range map[string]int{
		"/dev/fd/0":           0,
		"/dev/fd/1":           1,
		"/dev/fd/100":         100,
		"/dev/fd/100/kittens": -1,
		"/dev/fd/10kittens":   -1,
		"/dev/fd/kittens/":    -1,
		"/dev/fd/kittens":     -1,
		"/dev/fd/1/":          -1,
		"/dev/fd/":            -1,
		"/dev/fd":             -1,
		"/kittens":            -1,
		testInvalidDevFDName:  -1,
	} {
		try(name, fd)
	}
}
