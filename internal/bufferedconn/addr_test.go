package bufferedconn

/*
 * addr.go
 * Conn addresses
 * By Stuart McMurray
 * Created 20260326
 * Last Modified 20260326
 */

import (
	"fmt"
	"testing"
)

// Do Addrs work as expected?
func TestAddr(t *testing.T) {
	var np numPairs
	al, ar := np.newAddrPair()

	/* Check both sides' side-specific things. */
	for _, c := range []struct {
		a Addr
		s Side
	}{
		{a: al, s: SideLeft},
		{a: ar, s: SideRight},
	} {
		t.Run(string(c.s), func(t *testing.T) {
			/* Network should always be Network. */
			if got, want := c.a.Network(), Network; got != want {
				t.Errorf(
					"Network incorrect\n"+
						" got: %s\n"+
						"want: %s",
					got,
					want,
				)
			}
			/* Side should be consistent. */
			if got, want := c.a.Side, c.s; got != want {
				t.Errorf(
					"Side incorrect\n"+
						"have: %+v\n"+
						" got: %s\n"+
						"want: %s",
					c.a,
					got,
					want,
				)
			}
			/* String should be predictable. */
			if got, want := c.a.String(), fmt.Sprintf(
				"%d-%s",
				np.Load(),
				c.s,
			); got != want {
				t.Errorf(
					"String incorrect\n"+
						"have: %+v"+
						" got: %s\n"+
						"want: %s",
					c.a,
					got,
					want,
				)
			}

		})
	}
}

// Do calls to the package-level newAddrPair get different pair numbers?
func TestAddr_DifferentNumbers(t *testing.T) {
	l1, r1 := newAddrPair()
	l2, r2 := newAddrPair()

	if l1.PairNum == l2.PairNum {
		t.Errorf("Left pair numbers the same: %d", l1.PairNum)
	}
	if r1.PairNum == r2.PairNum {
		t.Errorf("Right pair numbers the same: %d", r1.PairNum)
	}
}
