package ctxerrgroup

/*
 * ctxerrgroup_test.go
 * Tests for ctxerrgroup.go
 * By J. Stuart McMurray
 * Created 20241226
 * Last Modified 20250105
 */

import (
	"cmp"
	"context"
	"errors"
	"testing"
)

// Make sure it works if we don't use WithConetxt.
func TestGroup_NoContext(t *testing.T) {
	/* ...if someone returns an error. */
	t.Run("with_error", func(t *testing.T) {
		eg := new(Group)
		testErr := errors.New("test error")
		eg.Go(func() error { return nil })
		eg.Go(func() error { return testErr })
		if got := eg.Wait(); nil == got {
			t.Errorf("Got nil error")
		} else if !errors.Is(got, testErr) {
			t.Errorf(
				"Incorrect error:\n got: %s\nwant: %s",
				got,
				testErr,
			)
		}

	})
	/* ...if no one returns an error. */
	t.Run("no_error", func(t *testing.T) {
		eg := new(Group)
		var (
			okGo, okGoContext, okGoTag bool
		)
		eg.Go(func() error { okGo = true; return nil })
		eg.GoContext(
			context.Background(),
			func(_ context.Context) error {
				okGoContext = true
				return nil
			},
		)
		eg.GoTag(
			context.Background(),
			"kittens",
			func(_ context.Context) error {
				okGoTag = true
				return nil
			},
		)
		if err := eg.Wait(); nil != err {
			t.Fatalf("Wait error: %s", err)
		}
		check := func(n string, b bool) {
			if !b {
				t.Errorf("%s not set", n)
			}
		}
		check("okGo", okGo)
		check("okGoContext", okGoContext)
		check("okGoTag", okGoTag)
	})
}

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
