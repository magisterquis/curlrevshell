package tlog

/*
 * rtsr.go
 * Add a radom suffix to a string.
 * By J. Stuart McMurray
 * Created 20260327
 * Last Modified 20260327
 */

import (
	"fmt"
	"math/rand/v2"
	"strconv"
)

// S appends a hyphen and a random base36 uint64 number to s.
func S(s string) string {
	return fmt.Sprintf(
		"%s-%s",
		s,
		strconv.FormatUint(
			rand.Uint64(),
			36,
		),
	)
}
