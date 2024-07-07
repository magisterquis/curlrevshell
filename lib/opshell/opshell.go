// Package opshell - Operator's interactive shell
package opshell

/*
 * opshell.go
 * Operator's interactive shell
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240707
 */

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/goxterm"
)

const (
	// ttyPath is the path to our own TTY, which may or may not actually be
	// hooked up.
	ttyPath = "/dev/tty"
	// timeFormat formats the current time the same way as the log package
	// does by default.
	timeFormat = "15:04:05.000 "
)

const (
	// PlainWritePause is the amount of time a terminal must have no
	// plain writes (i.e. [CLine]'s with Plain set to true) after a user
	// sends Ctrl+O before plain writes are written again.
	// This is meant to give a user a fighting chance after he accidentally
	// cats a large file.
	PlainWritePause = 2 * time.Second
)

// ErrOutputClosed is returned by Shell.Do when it returns because someone
// closed the output channel.
var ErrOutputClosed = errors.New("output channel closed")

// CLine is a line and a color to print.  If Prompt is not empty, it is set as
// the prompt before printing the line.
type CLine struct {
	Color       Color
	Line        string
	Prompt      string
	NoTimestamp bool /* Don't print a timestamp. */
	Plain       bool /* No newline, color, timestamp, or anything else. */
}

// Shell is the shell used by an operator.  It's a wrapper around
// github.com/magisterquis/goxterm.Terminal
type Shell struct {
	t            *goxterm.Terminal
	ich          chan<- string
	och          <-chan CLine
	ttyF         *os.File
	noTimestamps bool
	insertGen    func() ([]byte, error) /* Bytes-generator for ^I. */
	insertName   string                 /* Loggable name for insertGen. */
	wL           sync.Mutex             /* Write lock. */

	silenced       bool        /* Don't write plain messages for a bit. */
	silenceTimer   *time.Timer /* Unsilences after output's quiet. */
	lastPlainWrite time.Time   /* Last attempted write. */
}

// New puts the controlly TTY in raw mode and returns a new Shell wrapping
// stdio.  Call Shell.Do to start processing lines and handle resizing and
// call the returned function to restore the TTY's state and clean up other
// resources.  ich will be closed before Shell.Do returns.  IF noTimestamps is
// true, no timestamps will be printed.
// insertGen will be called to generate bytes to send to the shell on Ctrl+I
// and will be logged as if it were inserting data from insertName.
func New(
	ich chan<- string,
	och <-chan CLine,
	prompt string,
	noTimestamps bool,
	insertGen func() ([]byte, error),
	insertName string,
) (*Shell, func(), error) {
	/* Shell to return. */
	s := Shell{
		t:            goxterm.NewTerminal(stdioRW{}, prompt),
		ich:          ich,
		och:          och,
		noTimestamps: noTimestamps,
		insertGen:    insertGen,
		insertName:   insertName,
	}
	/* Set up a timer to unsilence the shell after there's been a lull. */
	s.silenceTimer = time.AfterFunc(0, func() {
		s.wL.Lock()
		defer s.wL.Unlock()

		/* If we're called during init, don't actually do anything. */
		if s.lastPlainWrite.IsZero() {
			return
		}

		/* If we're not actually ready, try again later. */
		if PlainWritePause > time.Since(s.lastPlainWrite) {
			s.resetSilenceTimer(false)
			return
		}

		/* Note we're no longer silenced. */
		s.silenced = false
		go s.Logf(ColorGreen, false, "Unmuting")
	})
	/* Handle control characters. */
	s.t.ControlCharacterCallback = func(key rune) {
		switch key {
		case 0x0F: /* ^O, silence output for a bit. */
			s.wL.Lock()
			defer s.wL.Unlock()
			/* Don't double-pause. */
			if s.silenced {
				go s.Logf(ColorRed, false, "Already muted")
				return
			}
			/* Pause output for a bit. */
			s.silenced = true
			s.resetSilenceTimer(true)
			go s.Logf(
				ColorRed,
				false,
				"Muting until we get %s of calm",
				PlainWritePause,
			)
		case 0x09: /* ^I, paste from file. */
			go s.insert()
			/* This is left here but commented out to make it that
			much easier to add another Ctrl+Key. */
			//default:
			//	go s.Logf(
			//		ColorGreen,
			//		false,
			//		"Got key: ^%c 0x%02x %q",
			//		key+'@', key, key,
			//	)
		}
	}
	/* Open the controlling TTY, for raw mode and output. */
	var err error
	if s.ttyF, err = os.Open(ttyPath); nil != err {
		return nil, nil, fmt.Errorf("opening controlling TTY: %w", err)
	}

	/* Cleanup things. */
	var oldState *goxterm.State
	cleanup := sync.OnceFunc(func() {
		/* Restore the terminal state. */
		if nil != oldState {
			goxterm.Restore(int(s.ttyF.Fd()), oldState)
		}

		/* Close the underlying TTY. */
		s.ttyF.Close()
	})

	/* Set the initial size. */
	if err := s.resize(); nil != err {
		cleanup()
		return nil, nil, fmt.Errorf("setting initial size: %w", err)
	}

	/* Put the TTY in raw mode. */
	if oldState, err = goxterm.MakeRaw(int(s.ttyF.Fd())); nil != err {
		cleanup()
		return nil, nil, fmt.Errorf(
			"putting terminal in raw mode: %w",
			err,
		)
	}

	return &s, cleanup, nil
}

// Do proxies between the channels with which the shell was made and stdio as
// well as watches for SIGWINCH to handle shell resizing.
// Do returns ErrOutputClosed if the output channel passed to New is closed.
func (s *Shell) Do(ctx context.Context) error {
	/* Do ALL the things. */
	eg, ectx := ctxerrgroup.WithContext(ctx)

	/* Resize on SIGWINCH. */
	eg.GoContext(ectx, s.handleWINCH)

	/* Read lines from stdin, send them out.  It'd be nice to do this in
	the errgroup, but goxterm.Terminal.ReadLine doesn't let us stop it. */
	eg.Go(func() error {
		for nil == ectx.Err() {
			/* Get a line from the input. */
			l, err := s.t.ReadLine()
			if nil != err {
				return err
			}
			/* Send it out. */
			s.ich <- l
		}
		return context.Cause(ectx)
	})

	/* Send lines sent to us to the shell. */
	eg.GoContext(ectx, s.handleOutput)

	/* Wait for something to go wrong. */
	return eg.Wait()

}

// resize resizes t to the size of its underlying TTY.
func (s *Shell) resize() error {
	/* Get the current size. */
	w, h, err := goxterm.GetSize(int(s.ttyF.Fd()))
	if nil != err {
		return fmt.Errorf("getting tty size: %w", err)
	}

	/* And set it. */
	if err := s.t.SetSize(w, h); nil != err {
		return fmt.Errorf("setting terminal size: %w", err)
	}

	return nil
}

// handleWINCH watches for SIGWINCH and resizes the underlying terminal every
// time one is received.
func (s *Shell) handleWINCH(ctx context.Context) error {
	/* Watch for SIGWINCH. */
	winchch := make(chan os.Signal, 1)
	signal.Notify(winchch, syscall.SIGWINCH)
	defer signal.Stop(winchch)

	/* Every time we get it, resize. */
	for {
		select {
		case <-winchch:
			if err := s.resize(); nil != err {
				return err
			}
		case <-ctx.Done():
			return context.Cause(ctx)
		}
	}
}

/* handleOutput handles reading from s.och and writing to s.t */
func (s *Shell) handleOutput(ctx context.Context) error {
	var (
		cl CLine
		ok bool
	)
	for {
		/* Try to grab some output. */
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case cl, ok = <-s.och:
			if !ok {
				return ErrOutputClosed
			}
		}
		/* Set the prompt if we have one. */
		if p := cl.Prompt; "" != p {
			s.t.SetPrompt(p)
		}
		/* Send the line where it goes. */
		var err error
		if cl.Plain {
			/* Straight to the terminal. */
			err = s.writePlain(cl.Line)
		} else {
			/* Print the line nicely. */
			_, err = s.Logf(cl.Color, cl.NoTimestamp, "%s", cl.Line)
		}
		if nil != err {
			return fmt.Errorf("writing to terminal: %w", err)
		}
	}
}

// writePlain writes a plain message to the terminal, assuming the terminal's
// not being silenced.
func (s *Shell) writePlain(line string) error {
	s.wL.Lock()
	defer s.wL.Unlock()

	/* If we've been told to be quiet, make sure we're not
	doing this too fast. */
	if s.silenced {
		s.resetSilenceTimer(true)
		return nil
	}

	/* Actually do the write. */
	_, err := io.WriteString(s.t, line)
	return err
}

// Logf logs a line to the shell.  It is similar to log.Printf but includes
// a color and only logs the time, not the date.  Logf may be called from
// multiple goroutines simultaneously.
func (s *Shell) Logf(
	color Color,
	noTS bool, /* No timestamp. */
	format string,
	v ...any,
) (int, error) {
	s.wL.Lock()
	defer s.wL.Unlock()
	return logf(
		s.t,
		s.t.Escape,
		color,
		noTS || s.noTimestamps,
		format,
		v...,
	)
}

// logf does what Shell.Logf says it does, but without assuming a shell.
func logf(
	w io.Writer,
	escape *goxterm.EscapeCodes,
	color Color,
	noTS bool, /* No timestamp. */
	format string,
	v ...any,
) (int, error) {
	/* Roll a message, making sure we've a newline. */
	m := fmt.Sprintf(format, v...)
	if 0 != len(m) && !strings.HasSuffix(m, "\n") {
		m += "\n"
	}

	/* No point in fiddling with colors if we have no line to write. */
	if 0 == len(m) {
		return 0, nil
	}

	/* Add a timestamp and colors, if we have them. */
	b := new(bytes.Buffer)
	if ColorNone != color {
		b.Write(ColorEC(escape, color))
	}
	if !noTS {
		b.WriteString(time.Now().Format(timeFormat))
	}
	b.WriteString(m)
	if ColorNone != color {
		b.Write(ColorEC(escape, ColorReset))
	}

	/* Actually do the writing. */
	n, err := b.WriteTo(w)
	return int(n), err
}

// SetPrompt sets the shell's prompt.  This can also be done by sending it a
// CLine with CLine.Prompt set.  Don't forget a trailing space.
// Use s.WrapIncolor to color the prompt.
func (s *Shell) SetPrompt(prompt string) { s.t.SetPrompt(prompt) }

// resetSilenceTimer resets the silenceTimer to fire PlainWritePause after
// s.lastPlainWrite.
// resetSilenceTimer's caller must hold s.wL.
// If updateLast is true, s.lastPlainWrite will be set to the current time
// before resetting the timer.
func (s *Shell) resetSilenceTimer(updateLast bool) {
	if updateLast {
		s.lastPlainWrite = time.Now()
	}
	s.silenceTimer.Reset(time.Until(s.lastPlainWrite.Add(PlainWritePause)))
}
