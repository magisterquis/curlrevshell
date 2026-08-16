package chanmutex

/*
 * chanmutex_test.go
 * Tests for chanmutex.go
 * By Stuart McMurray
 * Created 20260816
 * Last Modified 20260816
 */

import (
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// Does it at least not blow up?  This is a bit nondeterministic :/
func TestChannelMutex(t *testing.T) {
	synctest.Test(t, testChannelMutex)
}
func testChannelMutex(t *testing.T) {
	var (
		start = make(chan struct{})
		mu    = New()
		nTry  = 1024
		nHold atomic.Int64
		sleep = time.Hour
		wg    sync.WaitGroup
	)

	for range nTry {
		wg.Go(func() {
			/* Wait to start, to get close to parallel. */
			<-start
			/* Hold the lock, to unparallel. */
			mu.Lock()
			defer func() { nHold.Add(-1); mu.Unlock() }()
			/* We should be the only ones running. */
			if got, want := nHold.Add(1), int64(1); got != want {
				/* A bug will be very obvious. */
				t.Errorf(
					"Lock holder count incorrect\n"+
						" got: %d\n"+
						"want: %d",
					got,
					want,
				)
			}
			/* Sleep a bit to give others a chance to try before
			we release. */
			time.Sleep(sleep)
		})
	}

	/* Did it work? */
	close(start)
	wg.Wait()
}
