// Package ctxerrgroup - Like errgroup, but with more contexts
package ctxerrgroup

/*
 * ctxerrgroup.go
 * Like errgroup, but with more contexts
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20260603
 */

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

type (
	// tagContextKey is used to by [Group.GoTag] to store the tag in the
	// context passed to the function it calls.
	// The tag may be retrieved with [ContextTag].
	tagContextKey struct{}

	// tagsContextKey is used by [Group.GoTag] to append the tag to
	// the list in the context passed to the function it calls.
	// The list may be retrieved with [ContextTags].
	tagsContextKey struct{}
)

// Group wraps golang.org/x/sync/errgroup.Group but makes it slightly easier to
// add goroutines.
// Group's undocumented methods directly wrap its internal [errgroup.Group].
// All of Group's methods are safe to be simultaneously called from multiple
// goroutines.
type Group struct {
	p  *errgroup.Group
	mu sync.Mutex
}

// WithContext returns a new Group, similar to errgroup.WithContext.
func WithContext(ctx context.Context) (*Group, context.Context) {
	eg, ectx := errgroup.WithContext(ctx)
	return &Group{p: eg}, ectx
}

// eg returns the wrapped errgroup.Group, safely allocating it if it doesn't
// exist.
func (g *Group) eg() *errgroup.Group {
	g.mu.Lock()
	if nil == g.p {
		g.p = new(errgroup.Group)
	}
	g.mu.Unlock()
	return g.p
}

// GoContext is like errgroup.Group.Go, but passes ctx to the called function.
// The passed-in context is usually the context returned from WithContext.
func (g *Group) GoContext(ctx context.Context, f func(context.Context) error) {
	g.eg().Go(func() error { return f(ctx) })
}

// GoTag is like GoContext, but errors returned by f will be wrapped in a
// TaggedError.
func (g *Group) GoTag(ctx context.Context, tag string, f func(context.Context) error) {
	/* Set the tag in the context, so goroutines have a chance at knowing
	their purpose in life. */
	ctx = context.WithValue(ctx, tagContextKey{}, tag)
	/* And save it in a list, so goroutines can know who their parents
	are. */
	ctx = context.WithValue(ctx, tagsContextKey{}, append(
		ContextTags(ctx),
		tag,
	))

	/* Run the goroutine itself, tag-wrapping errors. */
	g.eg().Go(func() error {
		err := f(ctx)
		if nil != err {
			err = TaggedError{Tag: tag, Err: err}
		}
		return err
	})
}

func (g *Group) Go(f func() error)         { g.eg().Go(f) }
func (g *Group) SetLimit(n int)            { g.eg().SetLimit(n) }
func (g *Group) TryGo(f func() error) bool { return g.eg().TryGo(f) }
func (g *Group) Wait() error               { return g.eg().Wait() }

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

// contextValue extracts a value from ctx.  If the value is not present, the
// zero value of the type is returned
func contextValue[T any](ctx context.Context, key any) T {
	v, _ := ctx.Value(key).(T)
	return v
}

// ContextTag returns the tag in ctx set by [Group.GoTag], if any.
// If no tag was set the empty string is returned.
func ContextTag(ctx context.Context) string {
	return contextValue[string](ctx, tagContextKey{})
}

// ContextTags returns the tags in ctx set by nested calls to [Group.GoTag],
// if any.
// If no tags were set nil is returned.
func ContextTags(ctx context.Context) []string {
	return contextValue[[]string](ctx, tagsContextKey{})
}
