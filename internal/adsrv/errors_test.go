package adsrv

/*
 * errors_test.go
 * Tests for errors.go
 * By J. Stuart McMurray
 * Created 20260808
 * Last Modified 20260814
 */

import (
	"errors"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/crsadapter"
	"github.com/magisterquis/curlrevshell/lib/tlog"
)

// Can we whine about an unknown ConnType?
func TestUnknownConnTypeError(t *testing.T) {
	var (
		haveT = crsadapter.ConnType(tlog.S("unk"))
		haveE = UnknownConnTypeError{haveT}
		want  = "unknown connection type: " + string(haveT)
	)

	/* Stringification work? */
	if got := haveE.Error(); got != want {
		t.Errorf(
			"Incorrect string representation\n got: %v\nwant: %v",
			got,
			want,
		)
	}

	/* Can we compare easy enough? */
	haveN := UnknownConnTypeError{haveT}
	if !errors.Is(haveN, haveE) {
		t.Errorf("Do actually need an Is method")
	}
}

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
		have: &ConnResponseError{ConnResponse: crsadapter.ConnResponse{Error: es}},
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
		err:    ConnResponseError{crsadapter.ConnResponse{Error: errS}},
		target: ConnResponseError{crsadapter.ConnResponse{Error: errS}},
		want:   true,
	}, "different/string": {
		err:    ConnResponseError{crsadapter.ConnResponse{Error: errS}},
		target: ConnResponseError{crsadapter.ConnResponse{Error: targetS}},
		want:   false,
	}, "same/nil": {
		err:    ConnResponseError{crsadapter.ConnResponse{Error: ""}},
		target: nil,
		want:   true,
	}, "different/nil": {
		err:    ConnResponseError{crsadapter.ConnResponse{Error: errS}},
		target: nil,
		want:   false,
	}, "same/empty_string": {
		err:    ConnResponseError{crsadapter.ConnResponse{}},
		target: ConnResponseError{crsadapter.ConnResponse{}},
		want:   true,
	}, "different/empty_string": {
		err:    ConnResponseError{crsadapter.ConnResponse{Error: errS}},
		target: ConnResponseError{crsadapter.ConnResponse{}},
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
