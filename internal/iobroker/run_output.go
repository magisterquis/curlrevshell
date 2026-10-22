package iobroker

/*
 * run_output.go
 * Output side of Broker.Run.
 * By J. Stuart McMurray
 * Created 20260627
 * Last Modified 20260803
 */

import (
	"context"
	"errors"

	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// outputStreamInfo is the output stream football.
// Either the broker or the input stream may have it, but not both.
type outputStreamInfo struct {
	och        chan<- opshell.CLine
	brokerDone <-chan struct{}
	shellDone  <-chan struct{}
}

// errOutputDone indicates that an output stream finished, and is used to
// stop an input stream.  It should not be returned outside of this package.
var errOutputDone = errors.New("output done")

// runOutput runs the input side of b.Run.
// runOutput will not return if an output stream has och.
func (b *Broker) runOutput(
	ctx context.Context,
	och chan<- opshell.CLine,
	shellDone <-chan struct{},
) error {
	/* Wait for an output stream to connect. */
	select {
	case <-b.inputDone: /* No point in a new shell. */
		return nil
	case <-ctx.Done(): /* Time to give up. */
		b.closeOSICh()
		return nil
	case b.osiCh <- outputStreamInfo{ /* Output stream connected. */
		och:        och,
		brokerDone: ctx.Done(),
		shellDone:  shellDone,
	}:
		/* Good. */
	}

	/* Wait to get och back. */
	<-b.osiCh

	/* All done, let the input side know to disconnect. */
	return errOutputDone
}
