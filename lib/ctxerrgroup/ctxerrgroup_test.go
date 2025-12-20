package ctxerrgroup

/*
 * ctxerrgroup_test.go
 * Tests for ctxerrgroup.go
 * By J. Stuart McMurray
 * Created 20241226
 * Last Modified 20251220
 */

import (
	"cmp"
	"context"
	"errors"
	"slices"
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

// Can we extract tags from [Group.WithTag]'s contexts?
func TestContextTag(t *testing.T) {
	// goTag returns a function which returns a Group.GoTag-created
	// context with the tag set.
	// The returned function is meant to be used in the below tests' haves.
	goTag := func(tag string) func(t *testing.T) context.Context {
		return func(t *testing.T) context.Context {
			var (
				eg, ctx = WithContext(t.Context())
				got     context.Context
			)
			eg.GoTag(ctx, tag, func(ctx context.Context) error {
				got = ctx
				return nil
			})
			if err := eg.Wait(); nil != err {
				t.Errorf("Wait returned error: %s", err)
			}
			return got
		}
	}

	for n, c := range map[string]struct {
		have func(t *testing.T) context.Context
		want string
	}{"background": {
		have: func(_ *testing.T) context.Context {
			return context.Background()
		},
		want: "",
	}, "context.TODO": {
		have: func(_ *testing.T) context.Context {
			return context.TODO()
		},
		want: "",
	}, "testing.T": {
		have: func(t *testing.T) context.Context {
			return t.Context()
		},
		want: "",
	}, "GoTag/not_empty": {
		have: goTag("kittens"),
		want: "kittens",
	}, "GoTag/empty": {
		have: goTag(""),
		want: "",
	}} {
		t.Run(n, func(t *testing.T) {
			if got := ContextTag(c.have(t)); got != c.want {
				t.Errorf(
					"Tag incorrect\n got: %s\nwant: %s",
					got,
					c.want,
				)
			}
		})
	}
}

// Can we get our tag ancestry?
func TestContextTags(t *testing.T) {
	var (
		want []string
		have = []string{
			"one",
			"two",
			"three",
			"",
			"five",
			"six",
		}
		haveOrig = slices.Clone(have)
	)

	/* Does it work if we recurse a handful of times? */
	var f func(*testing.T, context.Context) error
	f = func(t *testing.T, ctx context.Context) error {
		/* Make sure our tag is correct so far. */
		got := ContextTags(ctx)
		if (want == nil && got != nil) ||
			(want != nil && got == nil) ||
			!slices.Equal(got, want) {
			t.Errorf(
				"Incorrect tags\n got: %q\nwant: %q",
				got,
				want,
			)
		}

		/* Work out the next tag. */
		if 0 == len(have) {
			return nil
		}
		tag := have[0]
		have = have[1:]
		want = append(want, tag)

		/* Recurse and try again. */
		t.Run("tag"+tag, func(t *testing.T) {
			eg, ctx := WithContext(ctx)
			eg.GoTag(ctx, tag, func(ctx context.Context) error {
				return f(t, ctx)
			})
			if err := eg.Wait(); nil != err {
				t.Errorf("Wait returned error: %s", err)
			}
		})

		return nil
	}
	f(t, t.Context())

	/* At the end, our want and our have should be switched around. */
	if nil == have {
		t.Errorf("Got nil have")
	} else if 0 != len(have) {
		t.Errorf("Lefover have tags: %q", have)
	}
	if !slices.Equal(haveOrig, want) {
		t.Errorf(
			"May not have used all of the tags\n"+
				"original have: %q\n"+
				"   final want: %q",
			haveOrig,
			want,
		)
	}
}
