package adsrv

/*
 * handle.go
 * Handle connected adapters
 * By J. Stuart McMurray
 * Created 20260808
 * Last Modified 20260808
 */

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/magisterquis/curlrevshell/internal/jsonstream"
)

// ConnRequestMaxWait is the maximum time [Server] will wait for a
// [ConnRequest] to show up on a new connection.
const ConnRequestMaxWait = time.Minute

// bidirIO contains the bits of net.UnixConn we use, to allow for using
// bidirpipe.Pipe during tests.
type bidirIO interface {
	io.ReadWriteCloser
	CloseRead() error
	CloseWrite() error
}

// handler Funchandles a connection.  If an error is returned, it will cause
// curlrevshell to exit.
type handlerFunc func(
	ctx context.Context,
	sl *slog.Logger,
	ct ConnType,
	jv jsontext.Value, /* Args, unparsed. */
	js *jsonstream.Stream,
) error

// Handle handles a connection from an adapter.  If an error is returned, it
// will cause curlrevshell to exit.
func (s *Server) handle(
	ctx context.Context,
	sl *slog.Logger,
	c bidirIO,
) error {
	sl.Debug(LMNewAdapterConn)
	defer c.Close()

	/* JSON stream reader. */
	js := jsonstream.New(c)

	/* Work out what this thing is. */
	toCtx, cancel := context.WithTimeoutCause(
		ctx,
		ConnRequestMaxWait,
		ErrConnRequestTimeout,
	)
	defer cancel()
	stop := context.AfterFunc(toCtx, func() { c.Close() })
	defer stop()
	args := new(jsontext.Value)
	cr := ConnRequest{Args: args}
	if err := js.DecodeNext(&cr); nil != err {
		if cause := context.Cause(toCtx); errors.Is(
			cause,
			ErrConnRequestTimeout,
		) {
			err = cause
		} else if nil != ctx.Err() {
			return nil
		}
		sl.Warn(
			LMConnRequestError,
			LKError, err,
		)
		return nil
	}
	stop()
	sl.Debug(
		LMConnRequestReceived,
		LKConnRequest, cr,
	)

	/* Work out how to handle this thing. */
	var (
		h   handlerFunc
		err error
	)
	switch cr.ConnType {
	case ConnTypeUnspecified:
		/* Empty */
		err = ErrUnspecifiedConnType
	case ConnTypeShellStream:
		/* Shell i/o. */
		h = s.handleStream
	default:
		/* Unknown. */
		err = UnknownConnTypeError{cr.ConnType}
	}
	if nil != err {
		sl.Warn(
			LMConnRequestError,
			LKError, err,
		)
		sendConnResponse(sl, js, err)
		return nil
	}

	/* Handle this thing. */
	return h(ctx, sl, cr.ConnType, *args, js)
}

// sendConnResponse sends a response to a ConnRequest.  It should be called
// before sending anything else on js.  If the request was accepted, rErr
// should be nil.
// On send error, a message is logged and Reply returns false.
func sendConnResponse(sl *slog.Logger, js *jsonstream.Stream, rErr error) bool {
	/* Roll a response. */
	cr := new(ConnResponse)
	if nil != rErr {
		cr.Error = rErr.Error()
	}
	sl = sl.With(LKConnResponse, cr)

	/* Send it back. */
	if err := js.Send(cr); nil != err {
		sl.Warn(
			LMConnResponseError,
			LKError, err,
		)
		return false
	}
	sl.Debug(LMConnResponseSent)
	return true
}
