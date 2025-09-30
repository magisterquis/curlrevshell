// Package opshell - Operator's interactive shell
package opshell

/*
 * opshell.go
 * Operator's interactive shell
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20250929
 */

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/goxterm"
)

const (
	// timeFormat formats the current time the same way as the log package
	// does by default.
	timeFormat = "15:04:05.000 "
)

// TestingTTY may be set to an empty string to always treat Shell's i/o as a
// TTY but also disable colors.
// It is intended to be used in tests via
//
//	go build -ldflags '-X github.com/magisterquis/curlrevshell/lib/opshell.TestingTTY="yes"'
var TestingTTY string

const (
	// PlainWritePause is the amount of time a terminal must have no
	// plain writes (i.e. [CLine]'s with Plain set to true) after a user
	// sends Ctrl+O before plain writes are written again.
	// This is meant to give a user a fighting chance after he accidentally
	// cats a large file.
	PlainWritePause = 2 * time.Second

	// CtrlCWait is the amount of time after receiving a Ctrl+C which we
	// wait for a second one for stopping the Shell.
	CtrlCWait = time.Second

	// FirstCtrlCWarning is sent to the shell in red the first time a user
	// hits Ctrl+C to let him know a second is needed to kill the shell.
	FirstCtrlCWarning = "Caught Ctrl+C.  " +
		"One more in the next second to kill the shell."

	// SecondCtrlCWarning is sent to stdout in red to let the user know
	// the shell's going away.
	SecondCtrlCWarning = "Caught second Ctrl+C."
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
	isTTY        bool
	noTimestamps bool
	insertGen    func() ([]byte, error) /* Bytes-generator for ^I. */
	insertName   string                 /* Loggable name for insertGen. */
	wL           sync.Mutex             /* Write lock. */

	silenced       bool        /* Don't write plain messages for a bit. */
	silenceTimer   *time.Timer /* Unsilences after output's quiet. */
	lastPlainWrite time.Time   /* Last attempted write. */
}

// New returns returns Shell wrapping stdin and stdout.
// Stdin will be put in raw mode if it is a terminal.
// Call Shell.Do to start processing lines and handle resizing and
// call the returned function to restore the TTY's state and clean up other
// resources.  ich will be closed before Shell.Do returns.  If noTimestamps is
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
	return NewWrapping(
		ich,
		och,
		prompt,
		noTimestamps,
		insertGen,
		insertName,
		os.Stdin,
		os.Stdout,
	)
}

// NewWrapping is like New but requires explicit input and output instead of
// assuming stdio.
func NewWrapping(
	ich chan<- string,
	och <-chan CLine,
	prompt string,
	noTimestamps bool,
	insertGen func() ([]byte, error),
	insertName string,
	stdin io.Reader,
	stdout io.Writer,
) (*Shell, func(), error) {
	return newWrapping(
		ich,
		och,
		prompt,
		noTimestamps,
		insertGen,
		insertName,
		stdin,
		stdout,
		false, /* assumeTTY */
	)
}

// newWrapping does what NewWrapping says it does, but with more config, for
// testing.
func newWrapping(
	ich chan<- string,
	och <-chan CLine,
	prompt string,
	noTimestamps bool,
	insertGen func() ([]byte, error),
	insertName string,
	stdin io.Reader,
	stdout io.Writer,
	assumeTTY bool, /* Assume stdio is a TTY, even if it's not. */
) (*Shell, func(), error) {
	/* Shell to return. */
	s := Shell{
		t: goxterm.NewTerminal(goxterm.ReadWriter{
			Reader: stdin,
			Writer: stdout,
		}, prompt),
		ich:          ich,
		och:          och,
		noTimestamps: noTimestamps,
		insertGen:    insertGen,
		insertName:   insertName,
	}
	/* Work out the underlying terminal, which will be a lot simpler if
	we're not using a TTY. */
	s.isTTY = goxterm.IsTerminal(int(os.Stdin.Fd())) &&
		goxterm.IsTerminal(int(os.Stdout.Fd()))
	if !s.isTTY && "" == TestingTTY && !assumeTTY {
		s.t.Cooked()
	}
	/* If we're using a testing TTY, also disable colors. */
	if "" != TestingTTY {
		s.t.Escape = goxterm.CookedEscapeCodes()
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
			go s.Insert()
		case 0x13: /* ^S, like ^I but just locally. */
			go s.pretendInsert()
			/* This is left here but commented out to make it that
			much easier to add another Ctrl+Key. */
			// default:
			// 	go s.Logf(
			// 		ColorGreen,
			// 		false,
			// 		"Got key: ^%c 0x%02x %q",
			// 		key+'@', key, key,
			// 	)
		}
	}

	/* Cleanup things. */
	var oldState *goxterm.State
	cleanup := sync.OnceFunc(func() {
		/* Don't bother if we can't restore the state. */
		if nil == oldState {
			return
		}

		/* Restore the terminal state. */
		goxterm.Restore(int(os.Stdin.Fd()), oldState)
	})

	/* Put terminal in raw mode, if we're using a real TTY. */
	if s.isTTY {
		/* Put tty in raw mode. */
		var err error
		if oldState, err = goxterm.MakeRaw(
			int(os.Stdin.Fd()),
		); nil != err {
			cleanup()
			return nil, nil, fmt.Errorf(
				"putting terminal in raw mode: %w",
				err,
			)
		}
	}

	/* Set the initial size. */
	if err := s.resize(); nil != err {
		cleanup()
		return nil, nil, fmt.Errorf("setting initial size: %w", err)
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
	eg.GoTag(ectx, "handling SIGWINCH", s.handleWINCH)

	/* Note the last time we got a Ctrl+C, for shell-ending. */
	var lastCtrlC time.Time

	/* Read lines from stdin, send them out.  It'd be nice to do this in
	the errgroup, but goxterm.Terminal.ReadLine doesn't let us stop it. */
	eg.Go(func() error {
		for nil == ectx.Err() {
			/* Get a line from the input. */
			l, err := s.t.ReadLine()
			if errors.As(err, &goxterm.CtrlC{}) &&
				(lastCtrlC.IsZero() ||
					CtrlCWait < time.Since(lastCtrlC)) {
				/* If we've not got one before or it's been
				more than a second since the last one, just
				print a warning. */
				s.Logf(
					ColorRed,
					false,
					"%s",
					FirstCtrlCWarning,
				)
				lastCtrlC = time.Now()
				continue
			} else if errors.As(err, &goxterm.CtrlC{}) {
				s.Logf(
					ColorRed,
					false,
					"%s",
					SecondCtrlCWarning,
				)
				/* Second time we've got one. */
				return err
			}
			if nil != err {
				return fmt.Errorf("reading line: %w", err)
			}
			/* Send it out. */
			s.ich <- l
		}
		return context.Cause(ectx)
	})

	/* Send lines sent to us to the shell. */
	eg.GoTag(ectx, "sending output", s.handleOutput)

	/* Wait for something to go wrong. */
	return eg.Wait()

}

// resize resizes t to the size of its underlying TTY, if we have one. */
func (s *Shell) resize() error {
	/* Nothing to do here if we don't have a TTY. */
	if !s.isTTY {
		return nil
	}

	/* Get the current size. */
	w, h, err := goxterm.GetSize(int(os.Stdin.Fd()))
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
// multiple goroutines simultaneously.  noTS can be used to suppress logging
// the timestamp, even if s would normally log timestamps.
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

// RedLogf is a wrapper around Logf which always uses the color red and doesn't
// suppress timestamps.  This is handy for logging errors.
func (s *Shell) RedLogf(format string, v ...any) (int, error) {
	return s.Logf(ColorRed, false, format, v...)
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
	/* Roll the message, newlineless.  We'll add them back later. */
	rs := []rune(fmt.Sprintf(format, v...))
	if 0 == len(rs) { /* Nothing to do if no message. */
		return 0, nil
	}
	nnl := 0
	for i := len(rs) - 1; i >= 0 && '\n' == rs[i]; i-- {
		nnl++
	}

	/* Add a timestamp and colors, if we have them. */
	b := new(bytes.Buffer)
	if ColorNone != color {
		b.Write(ColorEC(escape, color))
	}
	if !noTS {
		b.WriteString(time.Now().Format(timeFormat))
	}
	b.WriteString(string(rs[:len(rs)-nnl]))
	if ColorNone != color {
		b.Write(ColorEC(escape, ColorReset))
	}

	/* Put newlines back. */
	if 0 == nnl {
		nnl = 1
	}
	for range nnl {
		b.WriteRune('\n')
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
