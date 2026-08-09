// Package crsadapter - Protocol adapter library for curlrevshell
package crsadapter

/*
 * crsadapter.go
 * Protocol adapter library for curlrevshell
 * By J. Stuart McMurray
 * Created 20260809
 * Last Modified 20260809
 */

import (
	"context"
	"fmt"
	"net"
)

// Dial connects to the curlrevshell adapter socket path and requests a
// connection of type ct with the given arguments.
// Once dial has returned, the context will not affect the stream.
func Dial(ctx context.Context, path string, ct ConnType, args any) (*Stream, error) {
	/* Connect to curlrevshell. */
	u, err := (&net.Dialer{}).DialUnix(ctx, "unix", nil, &net.UnixAddr{
		Name: path,
		Net:  "unix",
	})
	if nil != err {
		return nil, fmt.Errorf("connecting to curlrevshell: %w", err)
	}

	/* Rest ef handshake, with a context. */
	return handshakeContext(ctx, u, ct, args)
}

// handshakeConetxt calls Handshake, but closes the connection if the context
// comes done.
func handshakeContext(
	ctx context.Context,
	u Streamer,
	ct ConnType,
	args any,
) (*Stream, error) {
	/* Close the connection if the context expires. */
	stop := context.AfterFunc(ctx, func() { u.Close() })
	defer stop() /* For just in case. */

	/* Does curlrevshell accept our connection? */
	s, err := Handshake(u, ct, args)
	if !stop() {
		u.Close() /* For just in case. */
		return nil, fmt.Errorf(
			"handshake ended early: %w",
			context.Cause(ctx),
		)
	} else if nil != err {
		return nil, err
	}

	return s, nil
}

// Handshake sends a ConnRequest and checks the ConnResponse.
func Handshake(s Streamer, ct ConnType, args any) (*Stream, error) {
	js := NewStream(s)

	/* Request to connection the connection. */
	if err := js.Send(ConnRequest{
		ConnType: ct,
		Args:     args,
	}); nil != err {
		return nil, fmt.Errorf("sending connection request: %w", err)
	}

	/* Did it work? */
	var res ConnResponse
	if err := js.DecodeNext(&res); nil != err {
		return nil, fmt.Errorf("reading connection response: %w", err)
	} else if "" != res.Error {
		return nil, ConnResponseError{res}
	}

	return js, nil
}
