package iobroker

/*
 * errors.go
 * Error types and values
 * By J. Stuart McMurray
 * Created 20260628
 * Last Modified 20260727
 */

import (
	"errors"
	"fmt"
)

// ErrIDEmpty is returned by [Broker.HandleInput] and [Broker.HandleOutput]
// when passed an empty stream ID.
var ErrIDEmpty = errors.New("stream ID empty")

// ErrIDIncorrect is returned by [Broker.HandleInput] and [Broker.HandleOutput]
// when a stream connects with an ID different than a connected stream of the
// opposite direction.
var ErrIDIncorrect = errors.New("stream ID incorrect")

// ErrInputClosed indicates that [Broker.Run] exited after the input channel
// it was passed was closed.
var ErrInputClosed = fmt.Errorf("input closed")

// ErrNotRunning is returned by [Broker.HandleInput] and [Broker.HandleOutput]
// when a stream connects to a not running broker.
var ErrNotRunning = errors.New("not running")

// ErrOneShell indicates one shell connected and disconnected, if oneShell
// was passed to [Broker.Run].
var ErrOneShell = errors.New("single shell finished")

// ErrStreamAlreadyConnected is returned by [Broker.HandleInput] and
// [Broker.HandleOutput] when a stream of the same direction is already
// connected.
var ErrStreamAlreadyConnected = errors.New("stream already connected")

var (
	errBrokerAlreadyRunning = errors.New("broker already running")
	errBidirOutputClosed    = errors.New("bidirectional output closed")
)
