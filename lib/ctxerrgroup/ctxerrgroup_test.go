package ctxerrgroup

/*
 * ctxerrgroup_test.go
 * Tests for ctxerrgroup.go
 * By J. Stuart McMurray
 * Created 20241226
 * Last Modified 20260803
 */

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/magisterquis/curlrevshell/internal/tlog"
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

// Can we use nested tags properly?
func TestGroupGoTag_NestedTags(t *testing.T) {
	/* checkTags makes sure ctx has the right tag and tags, as returned by
	ContextTag and ContextTags.   Runs in a subtest named name. */
	checkTags := func(
		t *testing.T,
		name string,
		ctx context.Context,
		wantTag string,
		wantTags []string,
	) {
		t.Run(name, func(t *testing.T) {
			/* Is the current tag correct? */
			if got, want := ContextTag(ctx), wantTag; got != want {
				t.Errorf(
					"Incorrect context tag\n"+
						" got: %s\n"+
						"want: %s",
					got,
					want,
				)
			}
			/* Is the tag list correct? */
			if got, want := ContextTags(ctx), wantTags; !slices.Equal(
				got,
				want,
			) {
				t.Errorf(
					"Incorrect context tags\n"+
						" got: %s\n"+
						"want: %s",
					got,
					want,
				)
			}
		})
	}
	/* Nest some error groups, make sure tags are correct. */
	eg1, ctx := WithContext(t.Context())
	checkTags(t, "root/before", ctx, "", nil)
	eg1.GoTag(ctx, "eg1", func(ctx context.Context) error {
		checkTags(t, "eg1/before", ctx, "eg1", []string{"eg1"})
		eg2, ctx := WithContext(ctx)
		eg2.GoTag(ctx, "eg2", func(ctx context.Context) error {
			checkTags(t, "eg2/before", ctx, "eg2", []string{
				"eg1",
				"eg2",
			})
			eg3, ctx := WithContext(ctx)
			eg3.GoTag(ctx, "eg3", func(ctx context.Context) error {
				checkTags(t, "eg3", ctx, "eg3", []string{
					"eg1",
					"eg2",
					"eg3",
				})
				return nil
			})
			err := eg3.Wait()
			checkTags(t, "eg2/before", ctx, "eg2", []string{
				"eg1",
				"eg2",
			})
			return err
		})
		err := eg2.Wait()
		checkTags(t, "eg1/after", ctx, "eg1", []string{"eg1"})
		return err
	})
	eg1.Wait()
	checkTags(t, "root/after", ctx, "", nil)
}

// Do nested tags work when there's parallel calls to GoTag?
func TestGroupGoTag_NestedParallel(t *testing.T) {
	synctest.Test(t, testGroupGoTagNestedParallel)
}
func testGroupGoTagNestedParallel(t *testing.T) {
	var (
		eg, ctx             = WithContext(t.Context())
		greatGrandparentTag = tlog.S("great-grandparent")
		grandparentTag      = tlog.S("grandparent")
		parentTag           = tlog.S("parent")
		have1               = tlog.S("tag1")
		have2               = tlog.S("tag2")
		got1                string
		got2                string
		gots1               []string
		gots2               []string
	)
	/* We'll need a few layers of tags to get to where we have to worry
	about append overwriting things. */
	eg.GoTag(ctx, greatGrandparentTag, func(ctx context.Context) error {
		eg, ctx := WithContext(ctx)
		eg.GoTag(ctx, grandparentTag, func(ctx context.Context) error {
			eg, ctx := WithContext(ctx)
			eg.GoTag(ctx, parentTag, func(
				ctx context.Context,
			) error {
				eg, ctx := WithContext(ctx)
				var (
					startCh = make(chan struct{})
					doneCh  = make(chan struct{})
				)
				eg.GoTag(ctx, have1, func(
					ctx context.Context,
				) error {
					close(startCh)
					<-doneCh
					got1 = ContextTag(ctx)
					gots1 = ContextTags(ctx)
					return nil
				})
				<-startCh
				eg.GoTag(ctx, have2, func(
					ctx context.Context,
				) error {
					defer close(doneCh)
					got2 = ContextTag(ctx)
					gots2 = ContextTags(ctx)
					return nil
				})
				return eg.Wait()
			})
			return eg.Wait()
		})
		return eg.Wait()
	})
	if err := eg.Wait(); nil != err {
		t.Errorf("Errorgroup returned error: %v", err)
	}
	check := func(t *testing.T, have, got string, gots []string) {
		if want := have; got != want {
			t.Errorf(
				"Tag incorrect\nhave:%s\n got: %s\nwant: %s",
				have,
				got,
				want,
			)
		}
		if got, want := gots, []string{
			greatGrandparentTag,
			grandparentTag,
			parentTag,
			have,
		}; !slices.Equal(
			got,
			want,
		) {
			t.Errorf(
				"Tags incorrect\nhave: %s\n got: %s\nwant: %s",
				have,
				got,
				want,
			)
		}
	}
	check(t, have1, got1, gots1)
	check(t, have2, got2, gots2)
}

// Can we set a limit on the number of goroutines?
func TestGroupSetLimit(t *testing.T) {
	synctest.Test(t, testGroupSetLimit)
}
func testGroupSetLimit(t *testing.T) {
	var (
		eg        Group
		limit     = 10
		nGo       = limit * 100 /* Total number of calls to eg.Go. */
		mu        sync.Mutex
		active    int
		maxActive int
		wg        sync.WaitGroup
		done      = make(chan struct{}, nGo)
	)

	/* Only allow so many at once. */
	eg.SetLimit(limit)

	/* noteActive notes a goroutine's active and returns a func to note
	it's no longer active.  It should be called like
	defer noteActive()().  */
	noteActive := func() func() {
		/* Note we're running and how many we've peaked at
		running. */
		mu.Lock()
		active++
		maxActive = max(active, maxActive)
		mu.Unlock()
		/* Function to un-active us. */
		return func() {
			/* All done, note we're no longer running. */
			mu.Lock()
			active--
			maxActive = max(active, maxActive)
			mu.Unlock()
		}
	}

	/* Spawn one that'll keep the errgroup from exiting if we get lucky
	enough that everybody else exits before anybody else jumps in. */
	eg.Go(func() error {
		defer noteActive()()
		for range nGo {
			<-done
		}
		return nil
	})

	/* Spawn lots and lots of Goroutines and make sure we never have too
	many. */
	for range nGo {
		eg.Go(func() error {
			defer noteActive()()

			/* Work for a long time. */
			time.Sleep(time.Hour)

			/* Note we're done, so the keeper-aliver can eventually
			return as well. */
			done <- struct{}{}

			return nil
		})
	}

	/* Wait until all goroutines have been queued and finished. */
	wg.Wait()

	/* Shouldn't have got one of these. */
	if err := eg.Wait(); nil != err {
		t.Errorf("Wait returned error: %v", err)
	}

	/* Shouldn't have any more active goroutines. */
	if 0 != active {
		t.Errorf("Goroutines still active after Wait: %d", active)
	}

	/* Should have hit the max. */
	if maxActive != limit {
		t.Errorf(
			"Incorrect maximum number of running goroutines\n"+
				"     limit: %d\n"+
				"max active: %d",
			limit,
			maxActive,
		)
	}

}

// Are we told if we're over the limit?
func TestGroupTryGo(t *testing.T) {
	var (
		limit    = 10
		eg       Group
		over     = 2
		rejected int
		done     = make(chan struct{})
		started  atomic.Uint64
	)
	/* Don't start too many. */
	eg.SetLimit(limit)

	/* Start too many, we should have some rejected. */
	for range limit + over {
		if !eg.TryGo(func() error {
			started.Add(1)
			<-done
			return nil
		}) {
			rejected++
		}
	}

	/* All done. */
	close(done)
	if err := eg.Wait(); nil != err {
		t.Errorf("Wait returned error: %v", err)
	}

	/* Did we start enough? */
	if got, want := started.Load(), uint64(limit); got != want {
		t.Errorf(
			"Incorrect number of goroutines started\n"+
				" got: %d\n"+
				"want: %d",
			got,
			want,
		)
	}

	/* Did we not start enough? */
	if rejected != over {
		t.Errorf(
			"Incorrect number of goroutines not started\n"+
				" got: %d\n"+
				"want: %d",
			rejected,
			over,
		)
	}
}
