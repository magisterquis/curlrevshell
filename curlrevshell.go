// Program curlrevshell - Even worse reverse shell, powered by cURL
package main

/*
 * curlrevshell.go
 * Even worse reverse shell, powered by cURL
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240329
 */

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/magisterquis/curlrevshell/internal/hsrv"
	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

var (
	// CertFileDir is the base name of the cert cache file.
	CertCacheDir = "curlrevshell"
	// CertCacheFile is the file we stick in CertCacheDir.
	CertCacheFile = "cert.txtar"
	// Prompt is the shell prompt, settable at compile-time.  It will be
	// colored Cyan.
	Prompt = "> "
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
			"Optional callback `template` file",
		)
		printDefaultTemplate = flag.Bool(
			"print-default-template",
			false,
			"Write the default template to stdout and exit",
		)
		certFile = flag.String(
			"tls-certificate-cache",
			defaultCertFile(),
			"Optional `file` in which to cache generated "+
				"TLS certificate",
		)
		noTimestamps = flag.Bool(
			"no-timestamps",
			false,
			"Don't print timestamps",
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

Even worse reverse shell, powered by cURL

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
	/* Fancypants shell. */
	shell, cleanup, err := opshell.New(
		ich,
		och,
		Prompt,
		*noTimestamps,
	)
	och <- opshell.CLine{Prompt: shell.WrapInColor(Prompt, opshell.ColorCyan)}
	if nil != err {
		log.Printf("Error setting up shell: %s", err)
	}
	defer cleanup()

	/* HTTPS Server */
	svr, cleanup, err := hsrv.New(
		*addr,
		*fdir,
		*tmplf,
		ich,
		och,
		*certFile,
		cbAddrs,
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
	defer cleanup()

	/* Start ALL the things. */
	eg, ectx := ctxerrgroup.WithContext(context.Background())
	eg.GoContext(ectx, shell.Do)
	eg.GoContext(ectx, svr.Do)

	/* Wait for something to go wrong. */
	if err := eg.Wait(); errors.Is(err, io.EOF) {
		shell.SetPrompt("")
		shell.Logf(opshell.ColorGreen, false, "Goodbye.")
	} else if nil != err {
		shell.Logf(opshell.ColorRed, false, "Fatal error: %s", err)
		return 1
	}

	return 0
}

// defaultCertfile returns a path for the default cert file.  It tries the
// system-specific user-specific cache, and failing that $HOME/ and ..
func defaultCertFile() string {
	/* Come up with a directory, somewhere. */
	if dir, err := os.UserCacheDir(); nil != err {
		log.Printf("Unable to determine cache directory: %s", err)
	} else {
		return filepath.Join(dir, CertCacheDir, CertCacheFile)
	}

	/* In not the cache directory, we'll want a . directory. */
	p := "." + CertCacheDir

	/* Try HOME. */
	if dir, err := os.UserHomeDir(); nil != err {
		log.Printf("Unable to determine home directory: %s", err)
	} else {
		filepath.Join(dir, p, CertCacheFile)
	}

	/* Give up and use the local directory. */
	return filepath.Join(p, CertCacheFile)
}
