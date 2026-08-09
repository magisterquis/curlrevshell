package crsadapter

/*
 * errors_test.go
 * Tests for errors.go
 * By J. Stuart McMurray
 * Created 20260809
 * Last Modified 20260809
 */

import (
	"testing"

	"github.com/magisterquis/curlrevshell/internal/tlog"
)

// Can we stringify properly?
func TestConnResponseError_Error(t *testing.T) {
	es := tlog.S("error-string")
	for n, c := range map[string]struct {
		have *ConnResponseError
		want string
	}{"empty string": {
		have: new(ConnResponseError),
		want: "",
	}, "error string": {
		have: &ConnResponseError{ConnResponse: ConnResponse{Error: es}},
		want: es,
	}} {
		t.Run(n, func(t *testing.T) {
			if got := c.have.Error(); got != c.want {
				t.Errorf(
					"Incorrect error string\n"+
						"have: %#v\n"+
						" got: %s\n"+
						"want: %s",
					c.have,
					got,
					c.want,
				)
			}
		})
	}
}

// Have we determined what the definition of Is is?
func TestConnResponseError_Is(t *testing.T) {
	var (
		errS    = tlog.S("error-string")
		targetS = tlog.S("target-string")
	)
	for n, c := range map[string]struct {
		err    ConnResponseError
		target error
		want   bool
	}{"same/string": {
		err:    ConnResponseError{ConnResponse{Error: errS}},
		target: ConnResponseError{ConnResponse{Error: errS}},
		want:   true,
	}, "different/string": {
		err:    ConnResponseError{ConnResponse{Error: errS}},
		target: ConnResponseError{ConnResponse{Error: targetS}},
		want:   false,
	}, "same/nil": {
		err:    ConnResponseError{ConnResponse{Error: ""}},
		target: nil,
		want:   true,
	}, "different/nil": {
		err:    ConnResponseError{ConnResponse{Error: errS}},
		target: nil,
		want:   false,
	}, "same/empty_string": {
		err:    ConnResponseError{ConnResponse{}},
		target: ConnResponseError{ConnResponse{}},
		want:   true,
	}, "different/empty_string": {
		err:    ConnResponseError{ConnResponse{Error: errS}},
		target: ConnResponseError{ConnResponse{}},
		want:   false,
	}} {
		t.Run(n, func(t *testing.T) {
			if got := c.err.Is(c.target); got != c.want {
				t.Errorf(
					"Is incorrect\n"+
						"   err: %v\n"+
						"target: %v\n"+
						"   got: %t\n"+
						"  want: %t",
					c.err,
					c.target,
					got,
					c.want,
				)
			}
		})
	}
}
