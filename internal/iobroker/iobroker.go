// Package iobroker - Hook up io.Read/Writers to opshell channels
package iobroker

/*
 * iobroker.go
 * Hook up io.Read/Writers to opshell channels
 * By J. Stuart McMurray
 * Created 20260620
 * Last Modified 20260808
 */

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// streamDir is the direction of a stream connection, input or output.
type streamDir string

// Shell messages and Log messages, keys, and values.
const (
	LMAlreadyConnected = "Connection already established"
	LMConnectionClosed = "Connection closed"
	LMIDIncorrect      = "ID incorrect"
	LMIDMissing        = "ID missing"
	LMNewConnection    = "New connection"
	LMShellFinished    = "Shell finished"
	LMShellIO          = "Shell I/O"
	LMShellStarting    = "Shell starting"
	LMUnknownInputType = "Unknown input type"

	LKData      = "data"
	LKDirection = "direction"
	LKError     = "error"
	LKID        = "id"
	LKType      = "type"

	LVBidir  streamDir = "bidirectional"
	LVInput  streamDir = "input"
	LVOutput streamDir = "output"

	SMBufferingInput   = "Buffering input until a shell connects..."
	SMConnectionClosed = "Connection closed"
	SMShellIsGone      = "Shell is gone :("
	SMShellIsReady     = "Shell is ready to go!"
)

type (
	// testRunStartedKey is used to extract a channel from a context for
	// notifying tests run has started.
	testRunStartedKey struct{}
	// testHandleSingleShellStartedKey is used to extract a func() from
	// a context for notifying tests the first handleSingleShell is
	// starting.  The func() should probably be idempotent.
	handleSingleShellStartedKey struct{}
)

// Broker allows io.Read/Writers to be hooked up to opshell channels.  It
// ensures at most one input and one output stream are connected at the same
// time, and disconnects a connected input stream if the connected output
// stream disconnects.
// With the exception of Run, Broker's methods are safe for simultaneous use
// from multiple goroutines.
type Broker struct {
	/* output Channel. */
	och chan<- opshell.CLine

	/* Channels for sending stream info to/from handlers. */
	isiCh      chan inputStreamInfo
	osiCh      chan outputStreamInfo
	closeOSICh func() /* sync.OnceFunc. */
	closeISICh func() /* sync.OnceFunc. */

	/* Connected stream ID's and such. */
	streamIDMu   sync.Mutex
	inStreamID   string
	outStreamID  string
	shellStarted bool /* Logged that we got a shell. */

	/* Closed after ich is closed. */
	inputDone      <-chan struct{} /* ich is closed. */
	closeInputDone func()          /* sync.OnceFunc. */

	/* Help messages to be printed before accepting shells. */
	helpMessagesMu sync.Mutex
	helpMessages   []helpMessage

	runStarted atomic.Bool
}

// New returns a new Broker, ready for use.  Call its Run method to
// start it going.
func New(och chan<- opshell.CLine) *Broker {
	var (
		inputDone = make(chan struct{})
		isiCh     = make(chan inputStreamInfo)
		osiCh     = make(chan outputStreamInfo)
	)
	return &Broker{
		och:            och,
		isiCh:          isiCh,
		osiCh:          osiCh,
		closeOSICh:     sync.OnceFunc(func() { close(osiCh) }),
		closeISICh:     sync.OnceFunc(func() { close(isiCh) }),
		inputDone:      inputDone,
		closeInputDone: sync.OnceFunc(func() { close(inputDone) }),
	}
}

// Run starts the broker going.
// If oneShell is true, after the first time an input stream and output stream
// are simultaneously connected, no new connections will be accepted and
// Run will return when the streams disconnect.
func (b *Broker) Run(
	ctx context.Context,
	ich <-chan string,
	oneShell bool,
) error {
	/* Idempotency. */
	if !b.runStarted.CompareAndSwap(false, true) {
		panic(errBrokerAlreadyRunning)
	}

	/* Let future streams know we're done, when we're done. */
	defer func() {
		b.closeOSICh()
		b.closeISICh()
		b.closeInputDone()
	}()

	if testing.Testing() {
		if ch, ok := ctx.Value(
			testRunStartedKey{},
		).(chan struct{}); ok {
			close(ch)
		}
	}

	/* Guiding principle: We only do things when we own ich/och; no exiting
	on context done if we don't have ich/och. */
	for !b.isInputDone() && nil == ctx.Err() {
		/* Print help messages, e.g. To Get a Shell... */
		b.printHelpMessages(ctx)

		/* Accept and handle a single shell. */
		b.handleSingleShell(ctx, ich)

		/* Don't do this again if we're only handling one shell. */
		if oneShell && nil == ctx.Err() {
			return ErrOneShell
		}
	}

	/* If the input stream is closed, let everybody else know. */
	if b.isInputDone() {
		return ErrInputClosed
	}

	/* Anything else is just the context closing. */
	return nil
}

// handleSingleShell accepts and handles a single shell.
func (b *Broker) handleSingleShell(
	ctx context.Context,
	ich <-chan string,
) {
	/* Tell interested tests we're starting. */
	if testing.Testing() {
		if f, ok := ctx.Value(
			handleSingleShellStartedKey{},
		).(func()); ok {
			f()
		}
	}

	/* Wrangler of goroutines. */
	eg, ctx := ctxerrgroup.WithContext(ctx)
	shellDone := make(chan struct{})
	defer close(shellDone)

	/* Input stream.  We accept multiple input streams per output
	stream so as to disconnect input when output disconnects.  As a
	side-effect, beaconing for input is possible, albeit somewhat
	awkward. */
	eg.GoTag(ctx, "input", func(ctx context.Context) error {
		for !b.isInputDone() && nil == ctx.Err() {
			b.runInput(ctx, ich)
		}
		return nil
	})

	/* Output stream.  We pass och to an output stream and wait
	for it back. */
	eg.GoTag(ctx, "output", func(ctx context.Context) error {
		err := b.runOutput(ctx, b.och, shellDone)
		return err
	})

	/* Wait for this shell to be done.  Returned errors are fake. */
	eg.Wait()
}

// isInputDone indicates whether b.inputDone is closed.
func (b *Broker) isInputDone() bool {
	select {
	case <-b.inputDone:
		return true
	default:
		return false
	}
}
