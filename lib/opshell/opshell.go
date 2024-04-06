// Package opshell - Operator's interactive shell
package opshell

/*
 * opshell.go
 * Operator's interactive shell
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240406
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
	"golang.org/x/term"
)

const (
	// ttyPath is the path to our own TTY, which may or may not actually be
	// hooked up.
	ttyPath = "/dev/tty"
	// timeFormat formats the current time the same way as the log package
	// does by default.
	timeFormat = "15:04:05.000 "
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
// golang.org/x/term.Terminal.
type Shell struct {
	t            *term.Terminal
	ich          chan<- string
	och          <-chan CLine
	ttyF         *os.File
	noTimestamps bool
	wL           sync.Mutex /* Write lock. */
}

// New puts the controlly TTY in raw mode and returns a new Shell wrapping
// stdio.  Call Shell.Do to start processing lines and handle resizing and
// call the returned function to restore the TTY's state and clean up other
// resources.  ich will be closed before Shell.Do returns.  IF noTimestamps is
// true, no timestamps will be printed.
func New(
	ich chan<- string,
	och <-chan CLine,
	prompt string,
	noTimestamps bool,
) (*Shell, func(), error) {
	/* Shell to return. */
	s := Shell{
		t:            term.NewTerminal(stdioRW{}, prompt),
		ich:          ich,
		och:          och,
		noTimestamps: noTimestamps,
	}
	var err error
	if s.ttyF, err = os.Open(ttyPath); nil != err {
		return nil, nil, fmt.Errorf("opening controlling TTY: %w", err)
	}

	/* Cleanup things. */
	var oldState *term.State
	cleanup := sync.OnceFunc(func() {
		/* Restore the terminal state. */
		if nil != oldState {
			term.Restore(int(s.ttyF.Fd()), oldState)
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
	if oldState, err = term.MakeRaw(int(s.ttyF.Fd())); nil != err {
		cleanup()
		return nil, nil, fmt.Errorf(
			"putting terminal in raw mode: %w",
			err,
		)
	}

	return &s, cleanup, nil
}

// Do proxies between the channels with which the shell was made and stdio as
// well as watches for SIGWINCH to handle shell resizing.  Do closes the
// input channel passed to New.  Do returns ErrOutputClosed if the output
// channel passed to New is closed.
func (s *Shell) Do(ctx context.Context) error {
	/* Do ALL the things. */
	eg, ectx := ctxerrgroup.WithContext(ctx)

	/* Resize on SIGWINCH. */
	eg.GoContext(ectx, s.handleWINCH)

	/* Read lines from stdin, send them out.  It'd be nice to do this in
	the errgroup, but term.Terminal.ReadLine doesn't let us stop it. */
	var (
		ech  = make(chan error, 1)
		ich  = s.ich
		ichL sync.Mutex
	)
	go func() {
		for {
			/* Get a line from the input. */
			l, err := s.t.ReadLine()
			if nil != err {
				ech <- err
				return
			}
			/* Send it out. */
			ichL.Lock()
			if nil == ich {
				ichL.Unlock()
				return
			}
			s.ich <- l
			ichL.Unlock()
		}
	}()

	/* Watch for lines read from the shell, send them out.  We do this in
	two goroutines to kinda sorta stop ReadLine as well as to stop other
	goroutines and so on. */
	eg.GoContext(ectx, func(ctx context.Context) error {
		/* Stop sending to ich when we're done. */
		defer func() {
			ichL.Lock()
			defer ichL.Unlock()
			close(ich)
			ich = nil
		}()
		/* Wait for something to go wrong. */
		for {
			select {
			case <-ctx.Done():
				return context.Cause(ctx)
			case err := <-ech:
				return fmt.Errorf(
					"reading input line: %w",
					err,
				)
			}
		}
	})

	/* Send lines sent to us to the shell. */
	eg.GoContext(ectx, s.handleOutput)

	return eg.Wait()
}

// resize resizes t to the size of its underlying TTY.
func (s *Shell) resize() error {
	/* Get the current size. */
	w, h, err := term.GetSize(int(s.ttyF.Fd()))
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
			_, err = io.WriteString(s.t, cl.Line)
		} else {
			/* Print the line nicely. */
			_, err = s.Logf(cl.Color, cl.NoTimestamp, "%s", cl.Line)
		}
		if nil != err {
			return fmt.Errorf("writing to terminal: %w", err)
		}
	}
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
	escape *term.EscapeCodes,
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
