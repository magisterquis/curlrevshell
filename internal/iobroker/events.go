package iobroker

/*
 * events.go
 * Events and logging things
 * By J. Stuart McMurray
 * Created 20240919
 * Last Modified 20240926
 */

import (
	"context"
	"fmt"
	"maps"

	"github.com/magisterquis/curlrevshell/lib/opshell"
)

const (
	// ShellReadyMessage is what we print when both sides of the shell are
	// connected.
	ShellReadyMessage = "Shell is ready to go!"

	// ShellDisconnectedMessage is what we print when both sides of the
	// shell are gone.
	ShellDisconnectedMessage = "Shell is gone :("
)

const (
	// logColor is used for happy logs.
	logColor = opshell.ColorGreen
	// errColor is used for unhappy logs.
	errColor = opshell.ColorRed
)

// Log messages, keys, and values.
const (
	LMAlreadyConnected = "Connection already established"
	LMDisconnected     = "Disconnected"
	LMDisconnecting    = "Previous shell disconnecting"
	LMIncorrectKey     = "Incorrect key"
	LMKeyMissing       = "Key missing"
	LMNewConnection    = "New connection"
	LMShellIO          = "Shell I/O"
	LMShuttingDown     = "Shutting down"

	LKData         = "data"
	LKDirection    = "direction"
	LKError        = "error"
	LKIncorrectKey = "incorrect_key"
	LKKey          = "key"

	LVInput  sDirection = "input"
	LVOutput sDirection = "output"
)

const (
	// EVChanLen is number of unsent events we'll buffer before blocking
	// happens.  It may be used for other event channels' buffers as well.
	EVChanLen = 1024
)

// EventType speciifes a type of Event.
type EventType string

const (
	EventTypeConnected    EventType = "connected"
	EventTypeDisconnected EventType = "disconnected"
)

// Event is something which happens in this library.
type Event struct {
	Type EventType
}

// processEvents reads the events sen
func (b *Broker) processEvents(ctx context.Context) {
	for {
		/* Get an event. */
		var ev Event
		select {
		case <-ctx.Done():
			return
		case ev = <-b.evCh:
		}
		/* Send it out to all listeners. */
		b.evMu.Lock()
		for l := range maps.Keys(b.evListeners) {
			l <- ev
		}
		b.evMu.Unlock()
	}
}

// AddEventListener starts events being sent to ch.  ch should be buffered.
func (b *Broker) AddEventListener(ch chan<- Event) {
	b.evMu.Lock()
	defer b.evMu.Unlock()
	b.evListeners[ch] = struct{}{}
}

// RemoveEventListener removes ch from the set of channels to which events
// will be sent.  ch will not be closed, but no events will be sent to ch
// after RemoveEventListener returns.
func (b *Broker) RemoveEventListener(ch chan<- Event) {
	b.evMu.Lock()
	defer b.evMu.Unlock()
	delete(b.evListeners, ch)
}

// sendLine sends a line back to the shell.
func (b *Broker) sendLine(color opshell.Color, addr, format string, a ...any) {
	b.och <- opshell.CLine{
		Color: color,
		Line: fmt.Sprintf(
			"[%s] %s",
			addr,
			fmt.Sprintf(format, a...),
		),
	}
}

// Logf sends the message to the shell, in green.
func (b *Broker) Logf(addr, format string, a ...any) {
	b.sendLine(logColor, addr, format, a...)
}

// Errorf sends the message to the shell, in red.
func (b *Broker) Errorf(addr, format string, a ...any) {
	b.sendLine(errColor, addr, format, a...)
}
