package lockingfile

/*
 * lockingfile_test.go
 * Tests for lockingfile.go
 * By J. Stuart McMurray
 * Created 20251212
 * Last Modified 20260814
 */

import (
	"bytes"
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

	"github.com/magisterquis/curlrevshell/lib/tlog"
)

func TestFile_Smoketest(t *testing.T) {
	/* Make a new File. */
	fn := filepath.Join(t.TempDir(), "kittens")
	if _, err := OpenFile(fn, os.O_WRONLY|os.O_CREATE, 0600); nil != err {
		t.Fatalf("Error creating %s: %s", fn, err)
	}
}

// Do we get an error if we can't open a file?
func TestOpenFile_Error(t *testing.T) {
	var (
		fn   = filepath.Join(t.TempDir(), "doesnotexist")
		want = syscall.ENOENT
	)
	if _, got := OpenFile(fn, os.O_RDWR|os.O_EXCL, 0); !errors.Is(
		got,
		want,
	) {
		t.Errorf("Incorrect error\n got: %v\nwant: %v", got, want)
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

// Are write errors reported?
func TestFileWrite_Error(t *testing.T) {
	/* We'll try to write to a directory, which should fail. */
	f, err := OpenFile(t.TempDir(), os.O_RDONLY, 0)
	if nil != err {
		t.Fatalf("Error opening temporary directory: %v", err)
	}
	/* Should fail with EBADF, specifically. */
	var want error = syscall.EBADF
	/* Does it? */
	if _, got := f.Write([]byte(tlog.S("msg"))); !errors.Is(got, want) {
		t.Errorf(
			"Incorrect error writing to directory\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
}

// Can we write without interleaving?  Unfortunately, there's not a great way
// to make sure all writes interleave.  But we can try a bunch of times
// anyways.
func TestFlock_interleavedwrites(t *testing.T) {
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
func TestFileWrite_pipe(t *testing.T) {
	pr, pw, err := os.Pipe()
	if nil != err {
		t.Fatalf("Error allocating pipe: %s", err)
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
	if _, err := (&File{File: *pw}).Write([]byte(have)); nil != err {
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

// Does flock fail when expected?
func TestFlock_Errors(t *testing.T) {
	/* Pipe, which we can't lock. */
	p, _, err := os.Pipe()
	if nil != err {
		t.Fatalf("Error creating pipe: %v", err)
	}

	/* Pipe error, which is platform-dependent. */
	var pErr error
	switch runtime.GOOS {
	case "darwin":
		pErr = syscall.ENOTSUP
	default:
		pErr = syscall.EOPNOTSUPP
	}

	for n, c := range map[string]struct {
		f    *os.File
		want error
	}{"nil_file": {
		f:    nil,
		want: os.ErrInvalid,
	}, "pipe": {
		f:    p,
		want: pErr,
	}} {
		t.Run(n, func(t *testing.T) {
			got := flock(c.f, syscall.LOCK_EX)
			if !errors.Is(got, c.want) {
				t.Errorf(
					"Incorrect error\n"+
						" got: %v\n"+
						"want: %v",
					got,
					c.want,
				)
			}
		})
	}
}
