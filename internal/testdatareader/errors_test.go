package testdatareader

/*
 * errors_test.go
 * Tests for errors.go
 * By J. Stuart McMurray
 * Created 20260815
 * Last Modified 20260815
 */

import (
	"errors"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/tlog"
)

// Do we error properly?
func TestReadError(t *testing.T) {
	var (
		havePath = tlog.S("path")
		haveErr  = errors.New(tlog.S("error"))

		have = ReadError{Path: havePath, Err: haveErr}
	)

	/* Do we stringify properly? */
	if got, want := have.Error(),
		"reading "+havePath+": "+haveErr.Error(); got != want {
		t.Errorf("Incorrect string form\n got: %s\nwant: %s", got, want)
	}

	/* Do we unwrap correctly? */
	if got, want := have.Unwrap(), haveErr; got != want {
		t.Errorf(
			"Incorrect unwrapped error\n got: %v\nwant: %v",
			got,
			want,
		)
	}
}
