package main

/*
 * readmeversion.go
 * Print the compiled-in version and branch
 * By J. Stuart McMurray
 * Created 20251209
 * Last Modified 20251209
 */

import (
	"fmt"

	"github.com/magisterquis/curlrevshell/internal/currentversion"
)

func main() {
	fmt.Printf("%s\n", currentversion.VersionAndBranch())
}
