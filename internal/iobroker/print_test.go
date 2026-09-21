package iobroker

/*
 * print_test.go
 * Tests for print.go
 * By J. Stuart McMurray
 * Created 20260720
 * Last Modified 20260814
 */

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/opshell"
	"github.com/magisterquis/curlrevshell/lib/tlog"
)

// Can we make a string with a tag?
func TestTaggedString(t *testing.T) {
	var (
		a1  = tlog.S("string")
		a2  = rand.Int()
		tag = tlog.S("tag")
		f   = "%s %d"
	)
	if got, want := taggedString(tag, f, a1, a2), fmt.Sprintf(
		"[%s] %s",
		tag,
		fmt.Sprintf(f, a1, a2),
	); got != want {
		t.Errorf(
			"Incorrect tagged string\n got: %s\nwant: %v",
			got,
			want,
		)
	}
}

// Can we log a colored message?
func TestColorf(t *testing.T) {
	try := func(t *testing.T, tag string) {
		var (
			arg1               = tlog.S("arg")
			arg2               = rand.Uint64()
			b, _, och, _, _, _ = newTestBroker(t, t.Context(), nil)
			color              = opshell.ColorBlue
			f                  = "msg: %s %d"

			want = opshell.CLine{
				Color: color,
				Line:  fmt.Sprintf(f, arg1, arg2),
			}
		)
		/* Add in the tag if we have one. */
		if "" != tag {
			want.Line = fmt.Sprintf("[%s] %s", tag, want.Line)
		}
		/* Did it work? */
		b.Colorf(tag, color, f, arg1, arg2)
		opshell.ExpectShellMessages(t, och, want)
	}
	t.Run("with_tag", func(t *testing.T) { try(t, tlog.S("tag")) })
	t.Run("no_tag", func(t *testing.T) { try(t, "") })
}
