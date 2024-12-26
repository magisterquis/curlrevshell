package ctxerrgroup

/*
 * ctxerrgroup_test.go
 * Tests for ctxerrgroup.go
 * By J. Stuart McMurray
 * Created 20241226
 * Last Modified 20241226
 */

import (
	"cmp"
	"context"
	"errors"
	"testing"
)

func TestGroupGoTag(t *testing.T) {
	tag := "kittens"
	wrappedErr := errors.New("a good error")
	f := func(ctx context.Context) error {
		return cmp.Or(ctx.Err(), wrappedErr)
	}
	eg, ectx := WithContext(context.Background())
	eg.Go(func() error { <-ectx.Done(); return ectx.Err() })
	eg.GoTag(ectx, tag, f)
	gotErr := eg.Wait()

	/* Make sure tagging works. */
	want := tag + ": " + wrappedErr.Error()
	if got := gotErr.Error(); want != got {
		t.Errorf(
			"Incorrect error string:\n got: %s\nwant: %s",
			got,
			want,
		)
	}

	/* Make sure wrapping works. */
	if !errors.Is(gotErr, wrappedErr) {
		t.Errorf(
			"Incorrect wrapping:\n got: %s\nwant: %s",
			gotErr,
			wrappedErr,
		)
	}
}
