package crstemplate

/*
 * errors_test.go
 * Tests for errors.go
 * By J. Stuart McMurray
 * Created 20261021
 * Last Modified 20261022
 */

import (
	"errors"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/tlog"
)

// Does ExtraneousTemplateDataCheckError work as expected?
func TestExtraneousTemplateDataCheckError(t *testing.T) {
	have := ExtraneousTemplateDataCheckError{
		Err: errors.New(tlog.S("error")),
	}
	if got, want := errors.Unwrap(have), have.Err; got != want { /* Exact */
		t.Errorf(
			"Incorrect unwrapped error\n got: %v\nwant: %v",
			got,
			want,
		)
	}
	if got, want := have.Error(),
		"checking for extraneous template data: "+
			have.Err.Error(); got != want {
		t.Errorf(
			"Incorrect stringified error\n got: %s\nwant: %s",
			got,
			want,
		)
	}
}
