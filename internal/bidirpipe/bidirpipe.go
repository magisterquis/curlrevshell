// Package bidirpipe - "Better" io.Pipe/net.Pipe
package bidirpipe

/*
 * bidirpipe.go
 * Like io.Pipe/net.Pipe, but bidirectional and with CloseRead/CloseWrite.
 * By J. Stuart McMurray
 * Created 20260626
 * Last Modified 20260801
 */

import (
	"cmp"
	"io"
)

// Pipe is like a combination of [io.Pipe] and [net.Pipe].  It's
// bidirectional but also allows closing a single direction.
// The underlying implementation is a pair of [io.PipeReader]/[io.PipeWriter]
// pairs.
type Pipe struct {
	pr *io.PipeReader
	pw *io.PipeWriter
}

// New returns a pair of connected Pipes.
func New() (Pipe, Pipe) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	return Pipe{pr: pr1, pw: pw2}, Pipe{pr: pr2, pw: pw1}
}

// Read implements io.Reader.
func (p Pipe) Read(b []byte) (n int, err error) { return p.pr.Read(b) }

// Write implements io.Writer.
func (p Pipe) Write(b []byte) (n int, err error) { return p.pw.Write(b) }

// CloseRead causes writes to the connected pipe to return net.ErrClosedPipe.
func (p Pipe) CloseRead() error { return p.pr.Close() }

// CloseWrite causes reads from the connected pipe to return EOF.
func (p Pipe) CloseWrite() error { return p.pw.Close() }

// Close closes the pipe.
func (p Pipe) Close() error { return cmp.Or(p.pr.Close(), p.pw.Close()) }
