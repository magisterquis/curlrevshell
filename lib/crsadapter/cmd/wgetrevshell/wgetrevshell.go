// Program wgetrevshell - Wget to curlrevshell adapter
package main

/*
 * wgetrevshell.go
 * Wget to curlrevshell adapter
 * By J. Stuart McMurray
 * Created 20260810
 * Last Modified 20260812
 */

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/lib/crsadapter/cmd/wgetrevshell/internal/server"
	"github.com/magisterquis/curlrevshell/lib/pledgeunveil"
	"github.com/magisterquis/curlrevshell/lib/sstls"
)

// URL path bits.
var (
	// URLPathIn is the path suffix for the shell input HTTP handler.
	URLPathIn = "/i"
	// URLPathOut is the path suffix for the shell output HTTP handler.
	URLPathOut = "/o"
)

// Log messages and keys and exit statuses.
const (
	LMErrorStartingListener = "Error starting listener"
	LMFatalError            = "Fatal error"
	LMGoodbye               = "Goodbye."
	LMStarting              = "Starting"

	LKFingerprint = "tls_fingerprint"
	LKPID         = "pid"

	ExitListenError = 10
	ExitFatalError  = 11
	ExitUnveilError = 12
)

func main() {
	pledgeunveil.MustPledge("cpath inet rpath stdio unix unveil wpath")
	/* Command-line flags. */
	var (
		lAddr = flag.String(
			"listen-address",
			"0.0.0.0:5555",
			"Listen `address`",
		)
		adapterPath = flag.String(
			"adapter-socket",
			"",
			"Unix socket `path` for adapters",
		)
		certFile = flag.String(
			"tls-certificate-cache",
			sstls.DefaultCertFile(),
			"Optional `file` in which to cache generated "+
				"TLS certificate",
		)
		printDebug = flag.Bool(
			"debug",
			false,
			"Print debugging messages",
		)
		pathPrefix = flag.String(
			"prefix",
			"/wgetrevshell",
			"HTTP URL path `prefix`",
		)
	)
	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			`Usage: %s [options]

Wget to curlrevshell adapter

Options:
`,
			filepath.Base(os.Args[0]),
		)
		flag.PrintDefaults()
	}
	flag.Parse()

	if err := pledgeunveil.MultiUnveil([][2]string{
		{*certFile, "crw"},
	}); errors.Is(err, syscall.ENOENT) {
		log.Printf("Unveil error: %s", err)
		os.Exit(ExitUnveilError)
	} else if nil != err {
		panic("unveil: " + err.Error())
	}
	pledgeunveil.MustPledge("cpath inet rpath stdio unix wpath")

	/* Logging. */
	var ho slog.HandlerOptions
	if *printDebug {
		ho.Level = slog.LevelDebug
	}
	sl := slog.New(slog.NewJSONHandler(os.Stdout, &ho))

	/* TLS Listener. */
	l, err := sstls.Listen("tcp", *lAddr, "", 0, *certFile)
	if nil != err {
		sl.Error(
			LMErrorStartingListener,
			iobroker.LKError, err,
		)
		os.Exit(ExitListenError)
	}
	pledgeunveil.MustPledge("inet stdio unix")

	sl.Info(
		LMStarting,
		LKPID, os.Getpid(),
		LKFingerprint, l.Fingerprint,
	)

	/* Serve. */
	if err := server.Serve(
		context.Background(),
		sl,
		l,
		*adapterPath,
		*pathPrefix,
		URLPathIn,
		URLPathOut,
	); nil != err {
		sl.Error(
			LMFatalError,
			iobroker.LKError, err,
		)
		os.Exit(ExitFatalError)
	}

	sl.Info(LMGoodbye)
}
