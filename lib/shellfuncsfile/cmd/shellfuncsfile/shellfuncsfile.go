// Program shellfuncsfile - Command-line wrapper around the shellfuncsfile library.
package main

/*
 * shellfuncsfile.go
 * Command-line wrapper around the shellfuncsfile library.
 * By J. Stuart McMurray
 * Created 20240731
 * Last Modified 20240731
 */

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/magisterquis/curlrevshell/lib/shellfuncsfile"
)

func main() { os.Exit(rmain()) }

func rmain() int {
	/* Command-line flags. */
	var (
		noList = flag.Bool(
			"no-list-function",
			false,
			"Don't also generate a tab_list() function",
		)
	)
	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			`Usage: %s [options] source

Command-line wrapper around the shellfuncsfile library.

Options:
`,
			os.Args[0],
		)
		flag.PrintDefaults()
	}
	flag.Parse()

	/* Make sure we have something to convert. */
	if 0 == flag.NArg() {
		log.Fatalf("Need a source")
	}

	/* Convert ALL the things. */
	conv := shellfuncsfile.NewDefaultConverter()
	conv.AddListFunction = !*noList
	b, err := conv.From(flag.Args()...)
	if nil != err {
		log.Fatalf("Error: %s", err)
	}
	if _, err := os.Stdout.Write(b); nil != err {
		log.Fatalf("Output error: %s", err)
	}

	return 0
}
