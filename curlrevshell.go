// Program curlrevshell - Even worse reverse shell, powered by cURL
package main

/*
 * curlrevshell.go
 * Even worse reverse shell, powered by cURL
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20270809
 */

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/magisterquis/curlrevshell/internal/adsrv"
	"github.com/magisterquis/curlrevshell/internal/currentversion"
	"github.com/magisterquis/curlrevshell/internal/hsrv"
	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/internal/lockingfile"
	"github.com/magisterquis/curlrevshell/lib/crstemplate"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/ezicanhazip"
	"github.com/magisterquis/curlrevshell/lib/opshell"
	"github.com/magisterquis/curlrevshell/lib/pledgeunveil"
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

// URL Paths, which may be set at compile-time to change from /i and /o and
// so on.
var (
	URLPathIn     = crstemplate.DefaultURLPathIn
	URLPathInOut  = crstemplate.DefaultURLPathInOut
	URLPathOut    = crstemplate.DefaultURLPathOut
	URLPathScript = crstemplate.DefaultURLPathScript
)

// Default file paths.  ./crs/... is a reasonable choice.  They correspond to
// flags with similar names.
var (
	DefaultCtrlI          string
	DefaultLog            string
	DefaultServeFilesFrom string
	DefaultTemplate       string
)

// Log messages and keys.
const (
	LMStarting    = "Program starting"
	LMTerminating = "Program terminating"

	LKPID = "PID"
)

func main() { os.Exit(rmain()) }
func rmain() int {
	pledgeunveil.MustPledge(
		"cpath flock inet rpath stdio tty unix unveil wpath",
	)
	/* Command-line flags. */
	var cbAddrs []string
	var (
		addr = flag.String(
			"listen-address",
			"0.0.0.0:4444",
			"Listen `address`",
		)
		adapterPath = flag.String(
			"adapter-socket",
			"",
			"Unix socket `path` for adapters",
		)
		fdir = flag.String(
			"serve-files-from",
			DefaultServeFilesFrom,
			"Optional `directory` from which to serve "+
				"static files",
		)
		tmplf = flag.String(
			"template",
			DefaultTemplate,
			"Optional `template` file, used if it exists",
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
			cmp.Or(os.Getenv(LogEnvVar), DefaultLog),
			"Optional `file` to which to write JSON logs",
		)
		oneShell = flag.Bool(
			hsrv.OneShellFlag,
			false,
			"Close listening socket when first shell connects",
		)
		insertFile = flag.String(
			"ctrl-i",
			DefaultCtrlI,
			"Tab/Ctrl+I's insertion `source` file or directory",
		)
		printCtrlI = flag.Bool(
			"print-ctrl-i",
			false,
			"Print what would be sent with Tab/Ctrl+I and exit",
		)
		printDebug = flag.Bool(
			"debug",
			false,
			"Print debugging messages",
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
			flag.CommandLine.Output(),
			`Usage: %s [options]

Even worse reverse shell, powered by cURL.

Version %s

Keyboard Shortcuts:
Ctrl+I - Insert the file or directory specified with -ctrl-i
Ctrl+O - Mute output for a couple of seconds (for if you cat a huge file)
Ctrl+S - Print locally what Ctrl+I would send
Tab    - Same as Ctrl+I

Options:
`,
			filepath.Base(os.Args[0]),
			currentversion.VersionAndBranch(),
		)
		flag.PrintDefaults()
	}
	flag.Parse()

	/* If we're just printing the default template, life's easy. */
	if *printDefaultTemplate {
		pledgeunveil.MustPledge("stdio")
		if _, err := io.WriteString(
			os.Stdout,
			crstemplate.DefaultTemplate,
		); nil != err {
			log.Printf("Error printing template: %s", err)
			return 1
		}
		return 0
	}

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
		pledgeunveil.MustPledge("rpath stdio unveil")
		if err := pledgeunveil.MultiUnveil(
			[][2]string{{*insertFile, "r"}},
		); nil != err {
			log.Printf(
				"Error unveiling %s: %s",
				*insertFile,
				err,
			)
			return 6
		}
		pledgeunveil.MustPledge("rpath stdio")
		b, err := insertGen()
		if nil != err {
			log.Fatalf("Error generating Ctrl+I file: %s", err)
		}
		os.Stdout.Write(b)
		return 0
	}

	/* Restrict what files we can see, but make the certificate cache
	directory first, so we can unveil down to just the file. */
	/* Restrict what files we can see, but make the directories for files
	we may create first so we can unveil down to just the file. */
	for _, v := range [][2]string{
		{"certificate cache", *certFile},
		{"logfile", *logFile},
	} {
		/* Don't bother if we're not actually using one. */
		if "" == v[1] {
			continue
		}
		dn := filepath.Dir(v[1])
		if err := os.MkdirAll(dn, 0700); nil != err {
			log.Printf(
				"Error making %s directory %s: %s",
				v[0],
				dn,
				err,
			)
			return 5
		}
	}
	if err := pledgeunveil.MultiUnveil([][2]string{
		{*certFile, "crw"},
		{*fdir, "r"},
		{*insertFile, "r"},
		{*logFile, "cw"},
		{*tmplf, "r"},
		{*adapterPath, "cw"},
	}); errors.Is(err, syscall.ENOENT) {
		log.Printf("Unveil error: %s", err)
		return 4
	} else if nil != err {
		panic("unveil: " + err.Error())
	}
	pledges := "cpath flock inet rpath stdio tty wpath"
	if "" != *adapterPath {
		pledges += " unix"
	}
	pledgeunveil.MustPledge(pledges)

	/* Channels for comms between subsystems. */
	var (
		ich = make(chan string, 1024)
		och = make(chan opshell.CLine, 1024)
	)

	/* And an adapter betwen io.{Read,Writ}er and channels. */
	iob := iobroker.New(och)

	/* Set up logging.  If we're not writing to a logfile, we'll just kinda
	discard log messages.  Beats checking for nil, anyways. */
	lh := slog.DiscardHandler
	if "" != *logFile {
		/* Open the logfile. */
		f, err := lockingfile.OpenFile(
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
		/* Work out our log level. */
		var ho slog.HandlerOptions
		if *printDebug {
			ho.Level = slog.LevelDebug
		}
		lh = slog.NewJSONHandler(f, &ho)
	}
	sl := slog.New(lh)
	sl.Info(LMStarting, LKPID, os.Getpid())

	/* Fancypants shell. */
	shell, cleanup, err := opshell.New(
		ich,
		och,
		Prompt,
		*noTimestamps,
		insertGen,
		*insertFile,
	)
	if nil != err {
		log.Fatalf("Error setting up shell: %s", err)
	}
	och <- opshell.CLine{Prompt: shell.WrapInColor(
		Prompt,
		opshell.ColorCyan,
	)}
	defer cleanup()

	/* Print a welcome message with our version. */
	shell.Logf(
		opshell.ColorNone,
		false,
		"Welcome to curlrevshell version %s",
		currentversion.VersionAndBranch(),
	)

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
	hServer, err := hsrv.New(
		sl,
		*addr,
		*tmplf,
		iob,
		*certFile,
		cbAddrs,
		*printIPv6,
		*printDebug,
		crstemplate.Params{
			StaticFilesDir: *fdir,
			URLPaths: crstemplate.URLPaths{
				In:     URLPathIn,
				InOut:  URLPathInOut,
				Out:    URLPathOut,
				Script: URLPathScript,
			},
		},
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
	shell.Logf(
		opshell.ColorNone,
		false,
		"Listening on %s",
		hServer.Addr(),
	)

	/* Adapter server. */
	var adServer *adsrv.Server
	if "" != *adapterPath {
		var err error
		if adServer, err = adsrv.New(
			sl,
			*adapterPath,
			iob,
		); nil != err {
			shell.Logf(
				opshell.ColorRed,
				false,
				"Error setting Adapter service: %s",
				err,
			)
			return 7
		}
		shell.Logf(
			opshell.ColorNone,
			false,
			"Listening for adapters on %s",
			adServer.Addr(),
		)
	}

	/* Print helpful help messages. */
	hServer.RegisterOneLiners()

	/* Start ALL the things. */
	eg, ectx := ctxerrgroup.WithContext(context.Background())
	eg.GoTag(ectx, "shell", shell.Do)
	eg.GoTag(ectx, "server", hServer.Do)
	eg.GoTag(ectx, "i/o broker", func(ctx context.Context) error {
		return iob.Run(ctx, ich, *oneShell)
	})
	if nil != adServer {
		eg.GoTag(ectx, "adapter", adServer.Run)
	}

	/* SIGUSR1 is equivalent to Ctrl+I. */
	eg.GoContext(ectx, func(ctx context.Context) error {
		sch := make(chan os.Signal, 1)
		signal.Notify(sch, syscall.SIGUSR1)
		defer signal.Stop(sch)
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-sch:
				shell.Insert()
			}
		}
	})

	/* Wait for something to go wrong. */
	err = eg.Wait()
	shell.SetPrompt("")
	if nil != err &&
		!errors.Is(err, hsrv.ErrOneShellClosed) &&
		!errors.Is(err, io.EOF) &&
		!errors.Is(err, iobroker.ErrInputClosed) &&
		!errors.Is(err, opshell.ErrInputDone) {
		shell.Logf(opshell.ColorRed, false, "Fatal error: %s", err)
		sl.Info(LMTerminating, hsrv.LKError, err)
		return 1
	}
	shell.Logf(opshell.ColorGreen, false, "Goodbye.")
	sl.Info(LMTerminating)

	return 0
}
