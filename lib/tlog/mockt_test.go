package tlog

/*
 * mockt_tests.go
 * Tests for mockt.go
 * By J. Stuart McMurray
 * Created 20260214
 * Last Modified 20260215
 */

import (
	"fmt"
	"slices"
	"testing"
)

func TestMockT_Smoketest(t *testing.T) { newMockT() }

// Does helper do nothing? (or at least not cause a panic)?
func TestMockTHelper(t *testing.T) {
	newMockT().Helper()
}

// Can we queue Messages?
func TestMockT_Messages(t *testing.T) {
	var (
		mt    = newMockT()
		haves = []struct {
			fmt   string
			args  []any
			fatal bool
		}{{
			"error string",
			[]any{},
			false,
		}, {
			"fatal string",
			[]any{},
			true,
		}, {
			"error message: %d %s %t",
			[]any{123, "kittens", true},
			false,
		}, {
			"fatal message: %d %s %t",
			[]any{456, "moose", true},
			true,
		}}
		wants = make([]mockTMessage, len(haves))
	)
	/* Queue messages and work out what we expect. */
	for i, v := range haves {
		/* Queue the message. */
		f := mt.Errorf
		if v.fatal {
			f = mt.Fatalf
		}
		f(v.fmt, v.args...)
		/* Work out what we expect. */
		wants[i] = mockTMessage{
			Fatal:   v.fatal,
			Message: fmt.Sprintf(v.fmt, v.args...),
		}
	}
	gots := mt.msgs

	/* Did it work? */
	for i := range min(len(gots), len(wants), len(haves)) {
		if got, want := gots[i], wants[i]; got != want {
			t.Errorf(
				"Message %d incorrect\n"+
					"have: %+v\n"+
					" got: %+v\n"+
					"want: %+v",
				i+1,
				haves[i],
				got,
				want,
			)
		}
	}

	/* Did we get enough messages? */
	if have, got, want := len(haves), len(gots), len(wants); got != want ||
		want != have {
		t.Errorf(
			"Message counts incorrect\n"+
				"have: %d\n"+
				" got: %d\n"+
				"want: %d",
			have,
			got,
			want,
		)
	}
}

// Do both testing.T and mockT satisfy T?
func TestT(t *testing.T) {
	var it ter
	/* Does testing.T work? */
	it = t
	if _, ok := it.(*testing.T); !ok {
		/* For just in case. */
		t.Errorf("T did not have a testing.T")
	}
	/* Does mockT work? */
	it = newMockT()
	if _, ok := it.(*mockT); !ok {
		/* For just in case. */
		t.Errorf("T did not have a mockT")
	}
}

// Can we make sure we got what we expect?
func TestMockTExpect(t *testing.T) {
	/* Work out what we have and expect. */
	type mSpec struct {
		fmt   string
		args  []any
		fatal bool
	}
	var (
		haves = []mSpec{{
			"error string",
			[]any{},
			false,
		}, {
			"fatal string",
			[]any{},
			true,
		}, {
			"error message: %d %s %t",
			[]any{123, "kittens", true},
			false,
		}, {
			"fatal message: %d %s %t",
			[]any{456, "moose", true},
			true,
		}}
		wants = make([]mockTMessage, len(haves))
	)
	for i, v := range haves {
		wants[i] = mockTMessage{
			Fatal:   v.fatal,
			Message: fmt.Sprintf(v.fmt, v.args...),
		}
	}

	/* Test cases. */
	for n, c := range map[string]struct {
		haves   []mSpec
		wants   []mockTMessage
		bufWant []string
	}{"empty": {}, "all_ok": {
		haves: haves,
		wants: wants,
	}, "extra_haves": {
		haves: haves,
		wants: wants[:len(wants)-2],
		bufWant: []string{
			"Leftover mock test message(s):\n" +
				fmt.Sprintf("%+v\n", wants[len(wants)-2]) +
				fmt.Sprintf("%+v", wants[len(wants)-1]),
		},
	}, "extra_wants": {
		haves: haves[:len(haves)-2],
		wants: wants,
		bufWant: []string{
			"Missing mock test message(s):\n" +
				fmt.Sprintf("%+v\n", wants[len(wants)-2]) +
				fmt.Sprintf("%+v", wants[len(wants)-1]),
		},
	}, "wrong_messages": {
		haves: []mSpec{haves[0], haves[1], haves[3]},
		wants: []mockTMessage{wants[0], wants[2], wants[3]},
		bufWant: []string{
			"Mock test message 1 incorrect\n" +
				"got:\n" +
				fmt.Sprintf("%+v\n", wants[1]) +
				"want:\n" +
				fmt.Sprintf("%+v", wants[2]),
		},
	}} {
		t.Run(n, func(t *testing.T) {
			mt := newMockT()
			/* Log the messages. */
			for _, v := range c.haves {
				f := mt.Errorf
				if v.fatal {
					f = mt.Fatalf
				}
				f(v.fmt, v.args...)
			}
			/* Did it work? */
			sb := new(stringsBuffer)
			mt.expect(sb.Printf, c.wants)
			if got, want := sb.Get(), c.bufWant; !slices.Equal(
				got,
				want,
			) {
				t.Errorf(
					"Log buffer incorrect\n"+
						"---got:\n%s\n"+
						"---want:\n%s",
					got,
					want,
				)
			}
		})
	}
}
