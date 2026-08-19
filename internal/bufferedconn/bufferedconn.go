// Package bufferedconn - In-memory buffered net.Conn pair
package bufferedconn

/*
 * bufferedconn.go
 * In-memory buffered net.Conn pair
 * By Stuart McMurray
 * Created 20260325
 * Last Modified 20260502
 */

import (
	"bytes"
	"cmp"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

// BufLen is the number of buffers (not bytes) a Conn will queue.
const BufLen = 1024

// Conn is one end of a pair of buffered conns.  Its methods implement
// net.Conn.
// Methods return [io.ErrClosedPipe] when the local Conn closed
// the pipe and [io.EOF] when the peer Conn closed the pipe.
type Conn struct {
	localAddr  Addr
	remoteAddr Addr

	txCh  chan<- []byte
	rxCh  <-chan []byte
	rxMu  sync.Mutex
	rxBuf []byte

	done      chan struct{}
	closeDone func() /* Closes done, once. */
	peerDone  <-chan struct{}

	readDeadline  pipeDeadline
	writeDeadline pipeDeadline
}

// NewPair return a pair of conns connected to each other.
func NewPair() (*Conn, *Conn) {
	var (
		al, ar = newAddrPair()
		l2r    = make(chan []byte, BufLen)
		r2l    = make(chan []byte, BufLen)
		lDone  = make(chan struct{})
		rDone  = make(chan struct{})
	)
	return &Conn{
			localAddr:     al,
			remoteAddr:    ar,
			txCh:          l2r,
			rxCh:          r2l,
			done:          lDone,
			closeDone:     sync.OnceFunc(func() { close(lDone) }),
			peerDone:      rDone,
			readDeadline:  makePipeDeadline(),
			writeDeadline: makePipeDeadline(),
		}, &Conn{
			localAddr:     ar,
			remoteAddr:    al,
			txCh:          r2l,
			rxCh:          l2r,
			done:          rDone,
			closeDone:     sync.OnceFunc(func() { close(rDone) }),
			peerDone:      lDone,
			readDeadline:  makePipeDeadline(),
			writeDeadline: makePipeDeadline(),
		}
}

func (c *Conn) Read(b []byte) (n int, err error) {
	/* Don't even try to read if we're not meant to. */
	select {
	case <-c.done:
		return 0, io.ErrClosedPipe
	case <-c.readDeadline.wait():
		return 0, os.ErrDeadlineExceeded
	default:
	}

	c.rxMu.Lock()
	defer c.rxMu.Unlock()

	/* If we don't have a current buffer, grab one. */
	if 0 == len(c.rxBuf) {
		select {
		case c.rxBuf = <-c.rxCh:
		case <-c.done:
			return 0, io.ErrClosedPipe
		case <-c.peerDone:
			select { /* Drain channel. */
			case c.rxBuf = <-c.rxCh:
			default:
				return 0, io.EOF
			}
		case <-c.readDeadline.wait():
			return 0, os.ErrDeadlineExceeded
		}
	}

	/* Use what we can from the current buffer. */
	ret := copy(b, c.rxBuf)
	c.rxBuf = c.rxBuf[ret:]
	return ret, nil
}

func (c *Conn) Write(b []byte) (n int, err error) {
	/* Don't even try to queue the buffer if we're not meant to. */
	select {
	case <-c.done:
		return 0, io.ErrClosedPipe
	case <-c.peerDone:
		return 0, io.EOF
	case <-c.writeDeadline.wait():
		return 0, os.ErrDeadlineExceeded
	default:
	}

	/* Don't bother queuing a nothing. */
	if 0 == len(b) {
		return 0, nil
	}

	/* Queue a copy of the buffer. */
	select {
	case c.txCh <- bytes.Clone(b):
		return len(b), nil
	case <-c.peerDone:
		return 0, io.EOF
	case <-c.done:
		return 0, io.ErrClosedPipe
	case <-c.writeDeadline.wait():
		return 0, os.ErrDeadlineExceeded
	}
}

func (c *Conn) Close() error         { c.closeDone(); return nil }
func (c *Conn) LocalAddr() net.Addr  { return c.localAddr }
func (c *Conn) RemoteAddr() net.Addr { return c.remoteAddr }

func (c *Conn) SetDeadline(t time.Time) error {
	return cmp.Or(
		c.SetReadDeadline(t),
		c.SetWriteDeadline(t),
	)
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	select {
	case <-c.done:
		return io.ErrClosedPipe
	case <-c.peerDone:
		return io.EOF
	default:
	}
	c.readDeadline.set(t)
	return nil
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	select {
	case <-c.done:
		return io.ErrClosedPipe
	case <-c.peerDone:
		return io.EOF
	default:
	}
	c.writeDeadline.set(t)
	return nil
}
