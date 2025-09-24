// Package ctxerrgroup - Like errgroup, but with more contexts
package ctxerrgroup

/*
 * ctxerrgroup_test.go
 * Like errgroup, but with more contexts
 * By J. Stuart McMurray
 * Created 20250924
 * Last Modified 20250924
 */

import "testing"

// Do Groups made with New crash?
func TestGroup_New(t *testing.T) {
	eg := New()
	var ok bool
	eg.Go(func() error { ok = true; return nil })
	if err := eg.Wait(); nil != err {
		t.Errorf("Unexpected error: %s", err)
	}
	if !ok {
		t.Errorf("Group's one function did not run")
	}
}
