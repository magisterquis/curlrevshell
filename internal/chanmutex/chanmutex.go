// Package chanmutex - Chan-based, synctest-friendly mutex
package chanmutex

/*
 * mutex.go
 * Chan-based, synctest-friendly mutex
 * By Stuart McMurray
 * Created 20260816
 * Last Modified 20260816
 */

// Mutex is a mutex based on a channel to play nicely with
// testing/synctest.
// Mutex implements [sync.Locker]
type Mutex chan struct{}

func New() Mutex { return make(Mutex, 1) }

// Lock acquires an exclusive lock on the mutex.
func (mu Mutex) Lock() { mu <- struct{}{} }

// Unlock releases a lock acquired by Lock.
func (mu Mutex) Unlock() { <-mu }
