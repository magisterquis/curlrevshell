// Package currentversion - Get the current version and branch
package currentversion

/*
 * currentversion.go
 * Get the current version and branch
 * By J. Stuart McMurray
 * Created 20250615
 * Last Modified 20250615
 */

import (
	_ "embed"
	"runtime/debug"
	"strings"
)

// masterBranch is the name of the branch for which VersionAndBranch doesn't
// return the branch.
const masterBranch = "master"

//go:embed current_branch
var currentBranch string

// init removes the newline git adds to the branch name.
func init() {
	currentBranch = strings.TrimRight(currentBranch, "\n")
}

// VersionAndBranch returns the current main module build version plus the
// branch, if not master.
func VersionAndBranch() string {
	/* Get the main module's version. */
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "build info not available"
	}
	v := bi.Main.Version
	/* Add in the brancnh name, if we're not in the master branch. */
	if masterBranch != currentBranch {
		v += " (" + currentBranch + " branch)"
	}
	return v
}

//go:generate sh -c "git branch --show-current >./current_branch"
