//go:build nettest

package bufferedconn

/*
 * bufferedconn_nettest_test.go
 * Tests for bufferedconn.go using golang.org/x/net/nettest
 * By Stuart McMurray
 * Created 20260501
 * Last Modified 20260327
 */

import (
	"net"
	"testing"

	"golang.org/x/net/nettest"
)

// Do our pairs of Conns do what pairs of conns should do?
func TestConn_Pair(t *testing.T) {
	nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
		c1, c2 = NewPair()
		stop = func() { c1.Close(); c2.Close() }
		return
	})
}
