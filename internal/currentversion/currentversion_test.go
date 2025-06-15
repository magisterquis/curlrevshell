package currentversion

/*
 * currentversion_test.go
 * Tests for currentversion.go
 * By J. Stuart McMurray
 * Created 20250615
 * Last Modified 20250615
 */

import (
	"strings"
	"testing"
)

func TestBranch_String(t *testing.T) {
	if "" == currentBranch {
		t.Errorf("Missing branch name")
	}
	if strings.Contains(currentBranch, "\n") {
		t.Errorf("Branch %q should have no newlines", currentBranch)
	}
}
