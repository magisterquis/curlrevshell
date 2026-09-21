package iobroker

/*
 * run_input.go
 * Input side of Broker.Run.
 * By J. Stuart McMurray
 * Created 20260627
 * Last Modified 20260808
 */

import (
	"context"
)

// inputStreamInfo is the input stream football.
// Either the broker or the input stream may have it, but not both.
type inputStreamInfo struct {
	bufferedLine *string
	ich          <-chan string
	brokerDone   <-chan struct{}
}

// runInput runs the input side of b.Run.
// runInput will not return if an input stream has ich.
func (b *Broker) runInput(ctx context.Context, ich <-chan string) {
	/* Send ich to an input stream. */
	if !b.sendIch(ctx, ich) {
		/* Nope, we still have it, time to give up. */
		return
	}

	/* Input stream is doing input things, wait for it to return ich. */
	<-b.isiCh

	/* Drain ich before the next shell. */
	b.drainIch(ich)
}

// sendIch sends ich to an input stream, possibly buffering a line first, and
// returns false if we still have ich, or true if we sent it to an input
// stream.
func (b *Broker) sendIch(
	ctx context.Context,
	ich <-chan string,
) bool {
	/* We make a new football every time to avoid leftover
	gunk. */
	isi := inputStreamInfo{
		ich:        ich,
		brokerDone: ctx.Done(),
	}

	/* Wait for a line to buffer, an input stream to take isi, or time for
	giving up. */
	select {
	case l, ok := <-ich: /* Buffer a line or ich closed. */
		if !ok { /* ich closed. */
			b.closeInputDone()
			return false
		}
		isi.bufferedLine = &l
		b.warnf("", SMBufferingInput)
	case b.isiCh <- isi: /* Input stream connected, they have ich. */
		return true
	case <-ctx.Done(): /* Time to give up. */
		return false
	}

	/* We buffered a line, only thing left is to send it somewhere or
	to give up. */
	select {
	case b.isiCh <- isi: /* Input stream connected. */
		return true
	case <-ctx.Done(): /* Time to give up. */
		return false
	}
}

// drainIch best-effort drains ich.
func (b *Broker) drainIch(ich <-chan string) {
	/* Got ich back.  Drain anything buffered before the next shell. */
	for {
		select {
		case _, ok := <-ich: /* Drain a line. */
			if !ok {
				b.closeInputDone()
				return
			}
		default: /* No more line to drain. */
			return
		}
	}
}
