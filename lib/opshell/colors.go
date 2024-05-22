package opshell

/*
 * colors.go
 * Colors for Shell.Logf
 * By J. Stuart McMurray
 * Created 20240323
 * Last Modified 20240507
 */

import (
	"bytes"
	"fmt"

	"github.com/magisterquis/goxterm"
)

// Color represents a specific color.
type Color int

// Options for colors in Terminal.ColorLogf
const (
	ColorNone Color = iota // Equivalent to Terminal.Logf
	ColorBlack
	ColorRed
	ColorGreen
	ColorYellow
	ColorBlue
	ColorMagenta
	ColorCyan
	ColorWhite
	ColorReset
)

// ColorEC returns the escape code for the given color from ec.  A non-nil
// slice will always be returned, even if ec is nil.  As a special case,
// ColorNone returns an empty slice, as do unknown colors.  The returned slice
// is a copy of the slice in ec or a newly-allocated slice; it may be modified
// at will.
func ColorEC(ec *goxterm.EscapeCodes, color Color) []byte {
	/* Figure out what color to return. */
	var b []byte
	switch color {
	case ColorBlack:
		b = ec.Black
	case ColorRed:
		b = ec.Red
	case ColorGreen:
		b = ec.Green
	case ColorYellow:
		b = ec.Yellow
	case ColorBlue:
		b = ec.Blue
	case ColorMagenta:
		b = ec.Magenta
	case ColorCyan:
		b = ec.Cyan
	case ColorWhite:
		b = ec.White
	case ColorReset:
		b = ec.Reset
	}

	/* No slice?  Probably ColorNone or something we've not heard of. */
	if nil == b {
		return make([]byte, 0)
	}

	/* Keep grubby mitts off our slice. */
	return bytes.Clone(b)
}

// WrapInColor returns s, wrapped on the left with the given color and on the
// right with ColorReset.  As a special case, if color is ColorNone, s is
// returned unchanged.
func (s *Shell) WrapInColor(st string, color Color) string {
	/* If we're not actually coloring things, nothing to do. */
	if ColorNone == color {
		return st
	}
	return fmt.Sprintf(
		"%s%s%s",
		ColorEC(s.t.Escape, color),
		st,
		ColorEC(s.t.Escape, ColorReset),
	)
}

//go:generate stringer -trimprefix Color -type Color
