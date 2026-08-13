package iobroker

/*
 * handle_common.go
 * Things common to handling input, output, and both at once
 * By J. Stuart McMurray
 * Created 20260630
 * Last Modified 20260812
 */

import (
	"context"
	"log/slog"
	"testing"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type (
	// testGetTSIKey extracts a channel from a context to tell us when to
	// get a streamInfo.
	testGetTSIKey[streamInfoT streamInfo] struct{}
	// testGotTSIKey extracts a channel from a context to tell tests when
	// we got a streamInfo.
	testGotTSIKey[streamInfoT streamInfo] struct{}
)

// streamInfo is is one of the *StreamInfo types.
type streamInfo interface {
	inputStreamInfo | outputStreamInfo
}

// handleCommon handles the bits common to HandleInput and HandleOutput,
// somewhat awkwardly.
func handleCommon[
	streamInfoT streamInfo,
	streamT any, /* io.Writer | io.ReadCloser */
](
	ctx context.Context,
	b *Broker, /* Because no generic methods. */
	sl *slog.Logger,
	id string, /* Stream ID, to match output stream. */
	tag string, /* For messages to the user, shell's IP is good. */
	tsiCh chan streamInfoT, /* b.isiCh/osiCh. */
	dir StreamDir, /* LVInput/Output. */
	ourID *string, /* b.in/outStreamID. */
	otherID *string, /* Ditto. */
	stream streamT, /* io.Writer/ReadCloser To/From shell. */
	proxy func( /* Shuffles between channel and stream. */
		ctx context.Context,
		sl *slog.Logger,
		tsi streamInfoT,
		stream streamT,
	) error,
) error {
	/* Need an ID. */
	if "" == id {
		sl.Warn(LMIDMissing)
		return ErrIDEmpty
	}
	sl = sl.With(LKID, id)

	/* Logger for logging non-direction things. */
	noDirSL := sl
	/* Logger for logging direction things. */
	sl = sl.With(LKDirection, dir)

	/* Make sure we're the correct stream to be here. */
	cDir := cases.Title(
		language.English,
	).String(string(dir)) /* Direction, capitalized. */
	tsi, err := connectStreamCommon(
		ctx,
		b,
		sl,
		noDirSL,
		id,
		tag,
		tsiCh,
		dir,
		cDir,
		ourID,
		otherID,
	)
	if nil != err || nil == tsi {
		return err
	}

	/* When we're all finished, clean up the ID we noted and send back the
	opshell channel if we got it. */
	var perr error
	defer func() {
		b.streamIDMu.Lock()
		defer b.streamIDMu.Unlock()
		/* We're done, unregister our ID. */
		*ourID = ""
		/* Note we've disconnected. */
		b.errorf(tag, "%s connection closed", cDir)
		if nil != perr {
			sl = sl.With(LKError, perr.Error())
		}
		sl.Info(LMConnectionClosed)
		/* Shell's done if we're the last to disconnect. */
		if b.shellStarted && "" == *otherID {
			/* Both sides are closed. */
			b.shellStarted = false
			noDirSL.Info(LMShellFinished)
			b.errorf(tag, SMShellIsGone)
		}
		/* Send the channel back. */
		tsiCh <- *tsi
	}()

	/* Proxy. */
	perr = proxy(ctx, sl, *tsi, stream)
	return perr
}

// connectStreamCommon holds b.streamIDMu and checks if we can accept the
// stream.  If so, it returns the streamInfoT from tsiCh, and any error
// received.
// If the context is done before a streamInfoT was received, the streamInfoT
// will be nil.
// connectStreamCommon logs as appropriate.
func connectStreamCommon[
	streamInfoT streamInfo,
](
	ctx context.Context,
	b *Broker,
	sl *slog.Logger,
	noDirSL *slog.Logger, /* sl, without LKDirection. */
	id string, /* Stream ID, to match output stream. */
	tag string, /* For messages to the user, shell's IP is good. */
	tsiCh chan streamInfoT, /* b.isiCh/osiCh. */
	dir StreamDir, /* LVInput/Output. */
	cDir string, /* dir, capitalized. */
	ourID *string, /* b.in/outStreamID. */
	otherID *string, /* Ditto. */
) (*streamInfoT, error) {
	b.streamIDMu.Lock()
	defer b.streamIDMu.Unlock()

	/* Make sure we're the correct stream to be here. */
	if "" != *ourID {
		/* Already have a connected stream. */
		b.errorf(
			tag,
			"Rejected unexpected %s connection with ID %q",
			dir,
			id,
		)
		sl.Warn(LMAlreadyConnected)
		return nil, ErrStreamAlreadyConnected
	} else if "" != *otherID && id != *otherID {
		/* Other stream has a different ID. */
		b.errorf(
			tag,
			"Rejected %s connection with incorrect ID %q, "+
				"expected %q",
			dir,
			id,
			b.outStreamID,
		)
		sl.Warn(LMIDIncorrect)
		return nil, ErrIDIncorrect
	}

	/* Get the opshell channel. */
	tsi, err := recvStreamInfoT(ctx, tsiCh)
	if nil != err {
		return nil, err
	} else if nil == tsi {
		return nil, nil
	}

	/* We're ok to be here. */
	*ourID = id /* Note our ID. */

	/* Note we've conected. */
	b.logf(tag, "%s connected: ID %s", cDir, id)
	sl.Info(LMNewConnection)

	/* Tell the user if we're starting a shell. */
	var logShellStarted bool
	if !b.shellStarted && (*otherID == id) {
		/* Got both sides. */
		b.shellStarted = true
		logShellStarted = true
	}
	if logShellStarted {
		b.logf(tag, SMShellIsReady)
		noDirSL.Info(LMShellStarting)
	}

	return tsi, nil
}

// recvStreamInfoT attempts to retrieve a streamInfoT from ch.
func recvStreamInfoT[streamInfoT streamInfo](
	ctx context.Context,
	tsiCh chan streamInfoT,
) (*streamInfoT, error) {
	/* Hooks for tests to track our progress. */
	if testing.Testing() {
		if ch, ok := ctx.Value(
			testGetTSIKey[streamInfoT]{},
		).(chan struct{}); ok {
			<-ch
		}
	}
	defer func() {
		if testing.Testing() {
			if ch, ok := ctx.Value(
				testGotTSIKey[streamInfoT]{},
			).(chan struct{}); ok {
				close(ch)
			}
		}
	}()
	/* Try to get the streamInfo. */
	select {
	case tsi, ok := <-tsiCh:
		if !ok {
			return nil, ErrNotRunning
		}
		return &tsi, nil
	case <-ctx.Done():
		return nil, nil
	}
}
