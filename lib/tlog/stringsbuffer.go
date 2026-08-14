package tlog

/*
 * stringsbuffer.go
 * Buffer which buffers strings
 * By J. Stuart McMurray
 * Created 20260214
 * Last Modified 20260215
 */

import (
	"fmt"
	"sync"
)

// stringsBuffer buffers each call to Printf as a separate string.  The
// underlying storage is a simple string slice; do not use when performance is
// important.
// The zero value of stringsBuffer is an empty buffer.
// stringsBuffer's methods are safe for concurrent use.
type stringsBuffer struct {
	mu     sync.Mutex
	buffer []string
}

// Printf adds a string to the buffer.
func (s *stringsBuffer) Printf(format string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buffer = append(s.buffer, fmt.Sprintf(format, args...))
}

// Get returns the buffered strings and resets the internal buffer.  The
// returned slice will never be nil.
func (s *stringsBuffer) Get() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	/* Don't return nil or bother shuffling about empty buffers. */
	if 0 == len(s.buffer) {
		return make([]string, 0)
	}
	/* Reset and return the buffer. */
	b := s.buffer
	s.buffer = nil
	return b
}
