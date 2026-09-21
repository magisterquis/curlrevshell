package bufferedconn

/*
 * addr.go
 * Conn addresses
 * By Stuart McMurray
 * Created 20260325
 * Last Modified 20260326
 */

import (
	"fmt"
	"sync/atomic"
)

// Network is returned by [Addr.Network].
const Network = "bufferedconn"

// defaultNumPairs is used by the package-level newAddrPair
var defaultNumPairs numPairs

// Side is a [Conn]'s side.  Each pair of Conns will have one SideLeft and one
// SideRight.
type Side string

const (
	SideLeft  Side = "left"
	SideRight Side = "right"
)

// Addr holds a [Conn]'s pair number, shared between Conns and unique per
// execution as well as whether it was the left or right returned conn.
type Addr struct {
	PairNum uint64
	Side    Side
}

// Network returns [Network].
func (a Addr) Network() string { return Network }

// String returns a's PairNum and Side joined with a hyphen.
func (a Addr) String() string {
	return fmt.Sprintf("%d-%s", a.PairNum, a.Side)
}

// numPairs keeps track of how many pairs have been made for use in
// [Addr.PairNum].
type numPairs struct{ atomic.Uint64 }

// newAddrPair returns a new pair of Addrs referring to a pair of connected
// Conns.
func (np *numPairs) newAddrPair() (leftAddr, rightAddr Addr) {
	pnum := np.Add(1)
	return Addr{PairNum: pnum, Side: SideLeft},
		Addr{PairNum: pnum, Side: SideRight}
}

// newAddrPair returns a new pair of Addrs using defaultNumPairs.
func newAddrPair() (leftAddr, rightAddr Addr) {
	return defaultNumPairs.newAddrPair()
}
