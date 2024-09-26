package chanlog

/*
 * chanlog_test.go
 * Log to a channel, for testing
 * By J. Stuart McMurray
 * Created 20240925
 * Last Modified 20240925
 */

import (
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
}
