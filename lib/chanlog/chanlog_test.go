package chanlog

/*
 * chanlog_test.go
 * Log to a channel, for testing
 * By J. Stuart McMurray
 * Created 20240925
 * Last Modified 20240930
 */

import (
	"fmt"
	"slices"
	"testing"
)

func TestChanLog_Smoketest(t *testing.T) { New() }

func TestChanLog(t *testing.T) {
	cl, sl := New()
	have := "kittens"
	want := `{"time":"","level":"INFO","msg":"kittens"}`

	t.Run("Expect", func(t *testing.T) {
		sl.Info(have)
		cl.Expect(t, want)
	})

	t.Run("ExpectEmpty", func(t *testing.T) {
		sl.Info(have)
		cl.ExpectEmpty(t, want)
	})

	t.Run("ExpectUnsorted", func(t *testing.T) {
		/* Send a bunch of messages out of order. */
		have := []string{
			"kittens",
			"moose",
			"kittens",
			"foo",
			"bar",
			"foo",
			"kittens",
			"tridge",
		}
		for _, line := range have {
			sl.Info(line)
		}
		/* Expect them in a different order. */
		want := make([]string, len(have))
		for i, line := range have {
			want[i] = fmt.Sprintf(
				`{"time":"","level":"INFO","msg":"%s"}`,
				line,
			)
		}
		slices.Sort(want)
		cl.ExpectUnordered(t, want...)
	})
}
