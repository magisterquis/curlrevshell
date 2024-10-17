// Program test_cat copies stdin to stdout.
package main

/*
 * test_cat.go
 * Copy stdin to stdout
 * By J. Stuart McMurray
 * Created 20241013
 * Last Modified 20241013
 */

import (
	"io"
	"log"
	"os"
)

func main() {
	if _, err := io.Copy(os.Stdout, os.Stdin); nil != err {
		log.Fatalf("Error: %s", err)
	}
}
