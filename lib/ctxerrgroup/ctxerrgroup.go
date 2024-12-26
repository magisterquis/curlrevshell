// Package ctxerrgroup - Like errgroup, but with more contexts
package ctxerrgroup

/*
 * ctxerrgroup.go
 * Like errgroup, but with more contexts
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20241226
 */

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// Group wraps golang.org/x/sync/errgroup.Group but makes it slightly easier to
// add goroutines.  Group's undocumented methods directly wrap its embedded
// errgroup.Group.
type Group struct {
	eg *errgroup.Group
}

// WithContext returns a new Group, similar to errgroup.WithContext.
func WithContext(ctx context.Context) (*Group, context.Context) {
	eg, ectx := errgroup.WithContext(ctx)
	return &Group{eg: eg}, ectx
}

// GoContext is like errgroup.Group.Go, but passes ctx to the called function.
// The passed-in context is usually the context returned from WithContext.
func (g *Group) GoContext(ctx context.Context, f func(context.Context) error) {
	g.eg.Go(func() error { return f(ctx) })
}

// GoTag is like GoContext, but errors returned by f will be wrapped in a
// TaggedError..
func (g *Group) GoTag(ctx context.Context, tag string, f func(context.Context) error) {
	g.eg.Go(func() error {
		err := f(ctx)
		if nil != err {
			err = TaggedError{Tag: tag, Err: err}
		}
		return err
	})
}

func (g *Group) Go(f func() error)         { g.eg.Go(f) }
func (g *Group) SetLimit(n int)            { g.eg.SetLimit(n) }
func (g *Group) TryGo(f func() error) bool { return g.eg.TryGo(f) }
func (g *Group) Wait() error               { return g.eg.Wait() }

// TaggedError is the error type returned by [Group.GoTag].
type TaggedError struct {
	Tag string
	Err error
}

// Error implements the error interface.  The returned string consists of the
// tag, ": ", and the original error.
func (err TaggedError) Error() string {
	return fmt.Sprintf("%s: %s", err.Tag, err.Err)
}

// Unwrap return err.Err.
func (err TaggedError) Unwrap() error { return err.Err }
