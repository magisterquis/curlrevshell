package adsrv

/*
 * handle_stream.go
 * Handle stream connections
 * By J. Stuart McMurray
 * Created 20260808
 * Last Modified 20260812
 */

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"io"
	"log/slog"
	"sync"

	"github.com/magisterquis/curlrevshell/lib/crsadapter"
)

// HandleStream handles a connection from an adapter requesting connection to
// a stream (i.e. /i, /io, or /o).
// cr.ConnType must be ConnTypeShellInput, ConnTypeShellInOut, or
// ConnTypeShellOutput.
func (s *Server) handleStream(
	ctx context.Context,
	sl *slog.Logger,
	ct crsadapter.ConnType,
	jv jsontext.Value, /* Args, unparsed. */
	js *crsadapter.Stream,
) error {
	/* Parse the info we'll need. */
	var ssa crsadapter.ConnTypeShellStreamArgs
	if err := json.Unmarshal(jv, &ssa); nil != err {
		sl.Warn(
			LMConnRequestArgsError,
			LKError, err,
		)
		sendConnResponse(sl, js, err)
		return nil
	}
	if nil != ssa.LogInfo {
		sl = sl.With(LKAdapterInfo, ssa.LogInfo)
	}

	/* Work out how to handle the request.  Weren't generics supposed to
	have this solved? */
	var handle func()
	switch ssa.Direction {
	case crsadapter.ShellStreamDirectionInput:
		/* Context'll need a bit of help to know when to stop. */
		ctx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		wg.Go(func() { io.Copy(io.Discard, js); cancel() })
		handle = func() {
			s.iob.HandleInput(ctx, sl, ssa.ID, ssa.Tag, js)
			js.Close() /* For just in case. */
			wg.Wait()
		}
	case crsadapter.ShellStreamDirectionInOut:
		handle = func() {
			s.iob.HandleBidirectional(ctx, sl, ssa.ID, ssa.Tag, js)
		}
	case crsadapter.ShellStreamDirectionOutput:
		handle = func() {
			s.iob.HandleOutput(ctx, sl, ssa.ID, ssa.Tag, js)
		}
	default:
		sendConnResponse(sl, js, UnknownShellStreamDirectionError{
			Direction: ssa.Direction,
		})
		return nil
	}

	/* Looks like we're all set. */
	if !sendConnResponse(sl, js, nil) {
		return nil
	}
	handle()

	return nil
}
