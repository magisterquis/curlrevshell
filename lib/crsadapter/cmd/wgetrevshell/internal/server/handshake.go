package server

/*
 * handshake.go
 * Connect to curlrevshell
 * By J. Stuart McMurray
 * Created 20260812
 * Last Modified 20260812
 */

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/magisterquis/curlrevshell/lib/crsadapter"
)

// pidID returns our ID, crsPrefix plus our PID.
func pidID() string { return crsIDPrefix + strconv.Itoa(os.Getpid()) }

// handshake connects to curlrevshell and returns a stream of the given
// direction.
func handshake(
	ctx context.Context,
	aSock string,
	direction crsadapter.ShellStreamDirection,
) (*crsadapter.Stream, error) {
	s, err := (&net.Dialer{}).DialUnix(ctx, "unix", nil, &net.UnixAddr{
		Name: aSock,
		Net:  "unix",
	})
	if nil != err {
		return nil, fmt.Errorf("dial: %w", err)
	}
	crs, err := crsadapter.Handshake(
		s,
		crsadapter.ConnTypeShellStream,
		handshakeArgs(direction),
	)
	if nil != err {
		s.Close()
		return nil, fmt.Errorf("handshake: %w", err)
	}

	return crs, nil
}

// handshakeArgs returns arguments suitable for adding to a handshake.
func handshakeArgs(
	direction crsadapter.ShellStreamDirection,
) crsadapter.ConnTypeShellStreamArgs {
	return crsadapter.ConnTypeShellStreamArgs{
		Direction: direction,
		ID:        pidID(),
		Tag:       crsTag,
	}
}
