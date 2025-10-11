package pledgeunveil

/*
 * wrappers.go
 * Wrappers around the real pledge/unveil wrappers.
 * By J. Stuart McMurray
 * Created 20251010
 * Last Modified 20251011
 */

import (
	"fmt"
	"slices"
)

// MustPledge Calls [Pledge](promises, "").  It panics on error.
func MustPledge(promises string) {
	if err := Pledge(promises, ""); nil != err {
		panic("pledge(" + promises + "): " + err.Error())
	}
}

// MultiUnveil wraps [Unveil] to unveil the {path, permissions} pairs in
// toUnveil.
// Paths may be the empty string; if so the path and corresponding perms are
// silently skipped.
// [UnveilBlock] is called if all of the calls to [Unveil] succeed.
func MultiUnveil(toUnveil [][2]string) error {
	/* Unveil ALL the files, deduping the list first as we may get multiple
	requests for the same file, e.g. /dev/null. */
	for path, perms := range parseMultiUnveilArgs(toUnveil) {
		if err := Unveil(path, perms); nil != err {
			return fmt.Errorf(
				"unveiling %s (%s): %w",
				path,
				perms,
				err,
			)
		}
	}

	/* Block further unveils. */
	if err := UnveilBlock(); nil != err {
		return fmt.Errorf("blocking further unveil calls: %w", err)
	}

	return nil
}

// parseMultiUnveilArgs parses the args passed to MultiUnveil
func parseMultiUnveilArgs(toUnveil [][2]string) map[string]string {
	/* Work out the permissions we'll need.  Entirely possible we'll get
	multiple requests for the same file, e.g. /dev/null. */
	m := make(map[string][]rune, len(toUnveil))
	for _, v := range toUnveil {
		path := v[0]
		if "" == path {
			continue
		}
		m[path] = append(m[path], []rune(v[1])...)
	}

	/* Sort/Uniq/String. */
	ret := make(map[string]string, len(m))
	for k, v := range m {
		slices.Sort(v)
		v = slices.Compact(v)
		ret[k] = string(v)
	}

	return ret
}
