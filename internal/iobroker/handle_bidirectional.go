package iobroker

/*
 * handle_bidirectional.go
 * Hook up bidirectional shell streams to ich and och
 * By J. Stuart McMurray
 * Created 20260723
 * Last Modified 20260803
 */

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"sync"

	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
)

// HandleBidirectional proxies between the connected shell and ich/och.
// rw.Close may be called before HandleOutput returns.
// If rw has CloseRead or CloseWrite methods, they may be called.
// id may be the empty string, in which case a random ID will be generated.
func (b *Broker) HandleBidirectional(
	ctx context.Context,
	sl *slog.Logger,
	id string, /* Stream ID, to match input stream. */
	tag string, /* For messages to the user, shell's IP is good. */
	rw io.ReadWriteCloser, /* From connected shell. */
) error {
	/* Need an ID. */
	if "" == id {
		id = strconv.FormatUint(rand.Uint64(), 36)
	}
	sl = sl.With(LKID, id)

	/* Logger for logging non-direction things. */
	noDirSL := sl
	/* Logger for logging bidirection things. */
	sl = sl.With(LKDirection, LVBidir)

	/* Make sure we can be here. */
	isi, osi, err := b.connectStreamBidirectional(ctx, sl, id, tag)
	if nil != err || nil == isi || nil == osi {
		return err
	}

	/* Clean up when we're done. */
	var perr error
	defer func() {
		b.streamIDMu.Lock()
		defer b.streamIDMu.Unlock()
		/* Set IDs back to available. */
		b.inStreamID = ""
		b.outStreamID = ""
		/* Note we've disconnected. */
		b.errorf(tag, SMConnectionClosed)
		if nil != perr {
			sl = sl.With(LKError, perr.Error())
		}
		sl.Info(LMConnectionClosed)
		/* Note the shell's done. */
		b.shellStarted = false
		sl.Info(LMShellFinished)
		b.errorf(tag, SMShellIsGone)
		/* Send the channels back. */
		b.isiCh <- *isi
		b.osiCh <- *osi
		/* Wait for the shell to finish before we release the mutex,
		so we don't end up with another bidirectional stream trying
		to grab ich.  Prevents hard-to-trigger deadlock. */
		<-osi.shellDone
	}()

	/* Proxy. */
	eg, ctx := ctxerrgroup.WithContext(ctx)
	eg.GoTag(ctx, string(LVInput), func(ctx context.Context) error {
		defer func() {
			if cw, ok := rw.(interface{ CloseWrite() error }); ok {
				cw.CloseWrite()
			}
		}()
		return proxyInput(
			ctx,
			noDirSL.With(LKDirection, LVInput),
			*isi,
			rw,
		)
	})
	eg.GoTag(ctx, string(LVOutput), func(ctx context.Context) error {
		defer func() {
			if cr, ok := rw.(interface{ CloseRead() error }); ok {
				cr.CloseRead()
			}
		}()
		err := proxyOutput(
			ctx,
			noDirSL.With(LKDirection, LVOutput),
			*osi,
			rw,
		)
		/* If our output is closed, our input will also need to close
		so we return an error to cancel the context.  It's not a
		real error, though, so it will turn into nil later. */
		if nil == err {
			err = errBidirOutputClosed
		}
		return err
	})
	perr = eg.Wait()
	/* Remove the non-error we get when the output's closed, if that's
	the "error." */
	if errors.Is(perr, errBidirOutputClosed) {
		perr = nil
	}

	return perr
}

// connectStreamCommon holds b.streamIDMu and checks if we can accept the
// stream.  If so, it returns the streamInfoT from tsiCh, and any error
// received.
// If the context is done before a streamInfoT was received, the streamInfoT
// will be nil.
// connectStreamCommon logs as appropriate.
func (b *Broker) connectStreamBidirectional(
	ctx context.Context,
	sl *slog.Logger,
	id string, /* Stream ID, to match input stream. */
	tag string, /* For messages to the user, shell's IP is good. */
) (*inputStreamInfo, *outputStreamInfo, error) {
	b.streamIDMu.Lock()
	defer b.streamIDMu.Unlock()

	/* Can't already have a connected stream. */
	if "" != b.inStreamID || "" != b.outStreamID {
		b.errorf(
			tag,
			"Rejected unexpected connection with ID %q",
			id,
		)
		sl.Warn(LMAlreadyConnected)
		return nil, nil, ErrStreamAlreadyConnected
	}

	/* Get the channels. */
	var (
		isi *inputStreamInfo
		osi *outputStreamInfo
	)
	eg, ectx := ctxerrgroup.WithContext(ctx)
	eg.GoContext(ectx, func(ctx context.Context) error {
		var err error
		isi, err = recvStreamInfoT(ctx, b.isiCh)
		return err
	})
	eg.GoContext(ectx, func(ctx context.Context) error {
		var err error
		osi, err = recvStreamInfoT(ctx, b.osiCh)
		return err
	})
	err := eg.Wait()

	/* Put back the channels if things went wrong. */
	if nil != err || nil == isi || nil == osi || nil != ctx.Err() {
		var wg sync.WaitGroup
		wg.Go(func() {
			if nil != isi {
				b.isiCh <- *isi
				isi = nil
			}
		})
		wg.Go(func() {
			if nil != osi {
				b.osiCh <- *osi
				osi = nil
			}
		})
		wg.Wait()
		return isi, osi, err
	}

	/* Note we're here. */
	b.inStreamID = id
	b.outStreamID = id

	/* Note we've conected. */
	b.logf(tag, "Connected: ID %s", id)
	sl.Info(LMNewConnection)

	/* Tell the user if we're starting a shell. */
	b.logf(tag, SMShellIsReady)
	sl.Info(LMShellStarting)

	return isi, osi, err
}
