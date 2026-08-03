package iobroker

/*
 * print.go
 * Print messages to the user.
 * By J. Stuart McMurray
 * Created 20260628
 * Last Modified 20260801
 */

import (
	"fmt"
	"strings"

	"github.com/magisterquis/curlrevshell/lib/opshell"
)

const (
	// LogColor is used for happy logs.
	LogColor = opshell.ColorGreen
	// ErrColor is used for unhappy logs.
	ErrColor = opshell.ColorRed
	// WarnColor is used for warnings.
	WarnColor = opshell.ColorMagenta
)

// errorf logs a tagged message in red.
// The tag will be omitted if it is the empty string.
func (b *Broker) errorf(tag string, format string, v ...any) {
	b.Colorf(tag, ErrColor, format, v...)
}

// logf logs a tagged message in green.
// The tag will be omitted if it is the empty string.
func (b *Broker) logf(tag string, format string, v ...any) {
	b.Colorf(tag, LogColor, format, v...)
}

// warnf logs a tagged message in pink.
// The tag will be omitted if it is the empty string.
func (b *Broker) warnf(tag string, format string, v ...any) {
	b.Colorf(tag, WarnColor, format, v...)
}

// Colorf logs a tagged message in a given color.
// The tag will be omitted if it is the empty string.
func (b *Broker) Colorf(
	tag string,
	color opshell.Color,
	format string, v ...any,
) {
	b.och <- opshell.CLine{
		Color: color,
		Line:  taggedString(tag, format, v...),
	}
}

// taggedString is like fmt.Sprintf, but adds the tag to the beginning of the
// resultant string in square brackets if the tag isn't the empty string.
func taggedString(tag string, format string, v ...any) string {
	var sb strings.Builder
	if "" != tag {
		sb.WriteRune('[')
		sb.WriteString(tag)
		sb.WriteString("] ")
	}
	fmt.Fprintf(&sb, format, v...)
	return sb.String()
}
