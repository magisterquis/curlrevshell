package pledgeunveil

/*
 * wrappers_test.go
 * Wrappers around the real pledge/unveil wrappers.
 * By J. Stuart McMurray
 * Created 20251011
 * Last Modified 20251011
 */

import (
	"maps"
	"testing"
)

func TestParseMultiUnveilArgs(t *testing.T) {
	for n, c := range map[string]struct {
		have [][2]string
		want map[string]string
	}{"different_files": {
		have: [][2]string{
			{"f_r", "r"},
			{"f_rw", "rw"},
			{"f_x", "x"},
			{"f_w", "w"},
			{"f_c", "c"},
			{"f_rwxc", "rwxc"},
		},
		want: map[string]string{
			"f_r":    "r",
			"f_rw":   "rw",
			"f_x":    "x",
			"f_w":    "w",
			"f_c":    "c",
			"f_rwxc": "crwx",
		},
	}, "repeated_paths": {
		have: [][2]string{
			{"kittens", "r"},
			{"kittens", "w"},
			{"kittens", "x"},
			{"kittens", "c"},
			{"moose", "rx"},
			{"moose", "wc"},
			{"f_rx", "rx"},
			{"f_xc", "x"},
			{"f_xc", "c"},
		},
		want: map[string]string{
			"f_rx":    "rx",
			"f_xc":    "cx",
			"kittens": "crwx",
			"moose":   "crwx",
		},
	}} {
		t.Run(n, func(t *testing.T) {
			if got := parseMultiUnveilArgs(c.have); !maps.Equal(
				got,
				c.want,
			) {
				t.Errorf(
					"Incorrect returned map\n"+
						"have: %v\n"+
						" got: %v\n"+
						"want: %v",
					c.have,
					got,
					c.want,
				)
			}
		})
	}

}
