package tlog

/*
 * errors_test.go
 * Tests for errors.go
 * By J. Stuart McMurray
 * Created 20260406
 * Last Modified 20260406
 */

import (
	"fmt"
	"slices"
	"testing"
)

// fakeGroupPathMissingError is similar to groupPathMissingError, but has the
// wrong type.
type fakeGroupPathMissingError struct{ Path []string }

// Error implements the error interface, via a real groupPathMissingError.
func (err fakeGroupPathMissingError) Error() string {
	return groupPathMissingError(err).Error()
}

// Does Is properly reject incorrect errors?
func TestGroupPathMissingErrorIs(t *testing.T) {
	gs := []string{S("g1"), S("g2")}
	e1 := groupPathMissingError{Path: slices.Clone(gs)}

	t.Run("equal", func(t *testing.T) {
		e2 := groupPathMissingError{Path: slices.Clone(gs)}
		if !e1.Is(e2) {
			t.Errorf(
				"Is returned false for equal errors\n"+
					"e1: %#v\n"+
					"e2: %#v",
				e1,
				e2,
			)
		}
	})

	t.Run("not_equal", func(t *testing.T) {
		e2 := groupPathMissingError{
			Path: append(slices.Clone(gs), S("g3")),
		}
		if e1.Is(e2) {
			t.Errorf(
				"Is returned true for unequal errors\n"+
					"e1: %#v\n"+
					"e2: %#v",
				e1,
				e2,
			)
		}
	})

	t.Run("incorrect_type", func(t *testing.T) {
		e2 := fakeGroupPathMissingError{Path: slices.Clone(e1.Path)}
		if e1.Is(e2) {
			t.Errorf(
				"Is returned true for a target "+
					"of the wrong type\n"+
					"e1: %T %#v\n"+
					"e2: %T %#v",
				e1, e1,
				e2, e2,
			)
		}
	})
}

// Do we get the right string form?
func TestGroupPathMissingErrorError(t *testing.T) {
	var (
		gs   = []string{S("g1"), S("g2")}
		have = groupPathMissingError{Path: gs}
	)
	if got, want := have.Error(), fmt.Sprintf(
		"group path %q missing",
		gs,
	); got != want {
		t.Errorf(
			"Error message incorrect\n"+
				"have: %#v\n"+
				" got: %s\n"+
				"want: %s",
			have,
			got,
			want,
		)
	}
}

// fakeGroupPathLeadsToNotGroupError is similar to groupPathLeadsToNotGroupError, but has the
// wrong type.
type fakeGroupPathLeadsToNotGroupError struct {
	Path []string
	Type string
}

// Error implements the error interface, via a real groupPathLeadsToNotGroupError.
func (err fakeGroupPathLeadsToNotGroupError) Error() string {
	return groupPathLeadsToNotGroupError(err).Error()
}

// Does Is properly reject incorrect errors?
func TestGroupPathLeadsToNotGroupErrorIs(t *testing.T) {
	var (
		gs = []string{S("g1"), S("g2")}
		v  = 1024
		e1 = newGroupPathLeadsToNotGroupError(gs, v)
	)

	t.Run("equal", func(t *testing.T) {
		e2 := newGroupPathLeadsToNotGroupError(gs, v)
		if !e1.Is(e2) {
			t.Errorf(
				"Is returned false for equal errors\n"+
					"e1: %#v\n"+
					"e2: %#v",
				e1,
				e2,
			)
		}
	})

	t.Run("not_equal/path", func(t *testing.T) {
		e2 := newGroupPathLeadsToNotGroupError(
			append(slices.Clone(gs), S("g3")),
			v,
		)
		if e1.Is(e2) {
			t.Errorf(
				"Is returned true for unequal errors\n"+
					"e1: %#v\n"+
					"e2: %#v",
				e1,
				e2,
			)
		}
	})
	t.Run("not_equal/value", func(t *testing.T) {
		e2 := newGroupPathLeadsToNotGroupError(gs, S("string"))
		if e1.Is(e2) {
			t.Errorf(
				"Is returned true for unequal errors\n"+
					"e1: %#v\n"+
					"e2: %#v",
				e1,
				e2,
			)
		}
	})

	t.Run("incorrect_type", func(t *testing.T) {
		e2 := fakeGroupPathLeadsToNotGroupError{
			Path: slices.Clone(e1.Path),
			Type: e1.Type,
		}
		if e1.Is(e2) {
			t.Errorf(
				"Is returned true for a target "+
					"of the wrong type\n"+
					"e1: %T %#v\n"+
					"e2: %T %#v",
				e1, e1,
				e2, e2,
			)
		}
	})
}

// Do we get the right string form?
func TestGroupPathLeadsToNotGroupErrorError(t *testing.T) {
	var (
		gs   = []string{S("g1"), S("g2")}
		v    = 1024
		have = newGroupPathLeadsToNotGroupError(gs, v)
	)
	if got, want := have.Error(), fmt.Sprintf(
		"group path %q leads to a value of type %T, not a group",
		gs,
		v,
	); got != want {
		t.Errorf(
			"Error message incorrect\n"+
				"have: %#v\n"+
				" got: %s\n"+
				"want: %s",
			have,
			got,
			want,
		)
	}
}
