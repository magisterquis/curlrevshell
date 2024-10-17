package simpleshell

/*
 * shell.go
 * Shell (or similar) subprocess
 * By J. Stuart McMurray
 * Created 20241013
 * Last Modified 20241013
 */

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"golang.org/x/sync/errgroup"
)

// Shell is connected to Curlrevshell by [Go].  It need not actually be a
// "real" shell (e.g. /bin/sh).
type Shell interface {
	// SetInput sets the io.Reader on which data from Curlrevshell will be
	// sent.
	SetInput(in io.Reader)

	// Output returns an io.Reader from which data to send to Curlrevshell
	// will be read.  Shells must close the returned io.Reader when no more
	// will be read, e.g. when an underlying subprocess terminates.
	// Output will always be called by Go.
	Output() io.ReadCloser

	// Go runs the underlying shell.  The io.ReadCloser returned by Output
	// should be closed before Go returns.
	// Go should not return an error if both all I/O completed successfully
	// and the underlying shell completed successfully, e.g. a subprocess
	// returned 0 on Unix.
	Go(ctx context.Context) error
}

// CmdShell turns an *[exec.Cmd] into a [Shell].
type CmdShell struct {
	cmd *exec.Cmd

	/* Output from cmd. */
	sout io.ReadCloser
	serr io.ReadCloser

	/* Output to Curlrevshell. */
	outr *io.PipeReader
	outw *io.PipeWriter
}

// NewCmdShell returns a new CmdShell which wraps cmd.
func NewCmdShell(cmd *exec.Cmd) (*CmdShell, error) {
	var (
		c   = CmdShell{cmd: cmd}
		err error
	)

	/* Work out pipes. */
	if c.sout, err = cmd.StdoutPipe(); nil != err {
		return nil, fmt.Errorf("getting stdout pipe: %w", err)
	}
	if c.serr, err = cmd.StderrPipe(); nil != err {
		return nil, fmt.Errorf("getting stderr pipe: %w", err)
	}
	pr, pw := io.Pipe()
	c.outr = pr
	c.outw = pw

	return &c, nil
}

// SetInput sets the [io.Reader] from which c reads input.
func (c *CmdShell) SetInput(in io.Reader) { c.cmd.Stdin = in }

// Output returns an [io.Reader] on which c sends output.
func (c *CmdShell) Output() io.ReadCloser { return c.outr }

// Go runs c's [exec.Cmd].  ctx is not used; use [exec.CommandContext] or cause
// an EOF on the [io.Reader] set via c.SetInPipe to stop Go.
func (c *CmdShell) Go(ctx context.Context) error {
	/* Start proxying output. */
	var peg errgroup.Group
	peg.Go(func() error { _, err := io.Copy(c.outw, c.sout); return err })
	peg.Go(func() error { _, err := io.Copy(c.outw, c.serr); return err })

	/* Start the process going. */
	var eg errgroup.Group
	eg.Go(func() error { return c.cmd.Run() })
	eg.Go(func() error { return c.outw.CloseWithError(peg.Wait()) })

	/* Wait until everything finishes. */
	return eg.Wait()
}

// String calls c's [exec.Cmd.String].
func (c *CmdShell) String() string { return c.cmd.String() }

// EchoShell is a [Shell] which just echos its input to its output, useful
// for testing.
type EchoShell struct {
	in   io.Reader
	out  *io.PipeWriter
	outr *io.PipeReader
}

// NewEchoShell returns a new EchoShell and pre-made i/o.
func NewEchoShell() (in *io.PipeWriter, out *io.PipeReader, shell *EchoShell) {
	ir, iw := io.Pipe()
	or, ow := io.Pipe()
	return iw, or, &EchoShell{in: ir, out: ow, outr: or}
}

// SetInput sets e's input; this is normally unnecessary.
func (e *EchoShell) SetInput(in io.Reader) { e.in = in }

// Output returns o's output.  This is the same *[io.PipeReader] returned by
// NewEchoShell.
func (e *EchoShell) Output() io.ReadCloser { return e.outr }

// Go copies between the returned i/o pipes.  ctx is ignored; close the input
// to stop Go.
func (e *EchoShell) Go(ctx context.Context) error {
	defer e.out.Close()
	_, err := io.Copy(e.out, e.in)
	return err
}
