package tlog

/*
 * mockt.go
 * Mock testing.T
 * By J. Stuart McMurray
 * Created 20260214
 * Last Modified 20260327
 */

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// ter (testing.T-er) is an interface containing the parts of testing.T used by
// this library, primarily for this libraries own tests.
// Avoid directly using ter, as it may grow over time.
// [testing.T] and [mockT] satisfy ter.
type ter interface {
	Errorf(format string, args ...any)
	Error(...any)
	Fatalf(format string, args ...any)
	Helper()
}

// mockTMessage contains a message logged via mockT.
type mockTMessage struct {
	Fatal   bool /* Logged via mockT.Fatalf. */
	Message string
}

// newMockTMessage is a convience wrapper which passes a to fmt.Sprint to form
// the message.
func newMockTMessage(fatal bool, a ...any) mockTMessage {
	return mockTMessage{
		Fatal:   fatal,
		Message: fmt.Sprint(a...),
	}
}

// mockT is an implementation of T which mocks testing.T.  mockT's methods are
// all safe to call concurrently.  mockT satisfies [ter].
type mockT struct {
	mu   sync.Mutex
	msgs []mockTMessage
}

// NewmockT returns a new mockT, ready to go.
func newMockT() *mockT { return &mockT{msgs: make([]mockTMessage, 0)} }

// Errorf appends the given message to m's mesasge buffer.
func (m *mockT) Errorf(format string, args ...any) {
	m.appendMessage(false, format, args...)
}

// Error appends the given message, stringified with fmt.Sprint, to m's message
// buffer.
func (m *mockT) Error(a ...any) { m.Errorf("%s", fmt.Sprint(a...)) }

// Fatalf appends the given message to m's mesasge buffer
// with its Fatal field set.
func (m *mockT) Fatalf(format string, args ...any) {
	m.appendMessage(true, format, args...)
}

// Helper is a no-op.
func (m *mockT) Helper() {}

// appendMessage appends a message to m.msgs.
func (m *mockT) appendMessage(fatal bool, format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = append(m.msgs, mockTMessage{
		Fatal:   fatal,
		Message: fmt.Sprintf(format, args...),
	})
}

// Expect checks if m contains msgs, calling t.Errorf if not.
func (m *mockT) Expect(t *testing.T, want []mockTMessage) {
	t.Helper()
	m.expect(t.Errorf, want)
}

// expect is like expect, but takes a function used for loging errors.  This
// should normally be t.Errorf, but can be StringsBuffer.Printf for testing.
func (m *mockT) expect(f func(msg string, args ...any), want []mockTMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	/* Make sure the common prefix of the lists is correct. */
	for i := range min(len(m.msgs), len(want)) {
		if got, want := m.msgs[i], want[i]; got != want {
			f(
				"Mock test message %d incorrect\n"+
					"got:\n%+v\n"+
					"want:\n%+v",
				i,
				got,
				want,
			)
		}
	}
	/* ts returns the elements of ms joined by newlines. */
	ts := func(ms []mockTMessage) string {
		ss := make([]string, len(ms))
		for i, v := range ms {
			ss[i] = fmt.Sprintf("%+v", v)
		}
		return strings.Join(ss, "\n")
	}

	/* If we have leftovers, log them, too. */
	if len(m.msgs) > len(want) {
		f(
			"Leftover mock test message(s):\n%s",
			ts(m.msgs[len(want):]),
		)
	} else if len(m.msgs) < len(want) {
		f(
			"Missing mock test message(s):\n%s",
			ts(want[len(m.msgs):]),
		)
	}
}
