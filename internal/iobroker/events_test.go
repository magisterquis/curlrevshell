package iobroker

/*
 * events_test.go
 * Tests for events.go
 * By J. Stuart McMurray
 * Created 20240919
 * Last Modified 20240926
 */

import (
	"sync"
	"testing"
)

// TestBrokerRemoveEventListener_Multiple is meant to check for race conditions
// removing event listeners.  This is inherently racy, and will probably need
// to be run with go test -count ....
func TestBrokerRemoveEventListener_Multiple(t *testing.T) {
	var (
		iob, _, _ = newTestBroker(t)
		nCh       = 128
		nEv       = EVChanLen
		wg        sync.WaitGroup
		chans     = make(map[chan Event]struct{}, nCh)
		start     = make(chan struct{})
	)
	/* Make lots of listeners. */
	for range nCh {
		ch := make(chan Event, nEv*2)
		chans[ch] = struct{}{}
		iob.AddEventListener(ch)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			iob.RemoveEventListener(ch)
			close(ch)
		}()
	}
	/* Tee up a bunch of sends. */
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for range nEv {
			iob.evCh <- Event{}
		}
	}()

	/* Let chaos ensue. */
	close(start)
	wg.Wait()
}
