package iobroker

/*
 * help.go
 * Print helpful text before accepting a shell
 * By J. Stuart McMurray
 * Created 20260620
 * Last Modified 20260801
 */

import (
	"context"
	"fmt"

	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// HelpMessage returns a help message to be shown to the user before waiting
// for a shell to connect.
type HelpMessage func() (string, error)

// helpMessage holds a HelpMessage and a name used for errors.
type helpMessage struct {
	name string
	hm   HelpMessage
}

// RegisterHelpMessage registers a function to print a help message before
// waiting for a shell to connect.
// Messages will be preceded with a timestamp and printed in the order
// registered.
// The name is only used in reporting errors.
func (b *Broker) RegisterHelpMessage(name string, hm HelpMessage) {
	b.helpMessagesMu.Lock()
	defer b.helpMessagesMu.Unlock()
	b.helpMessages = append(b.helpMessages, helpMessage{name: name, hm: hm})
}

// printHelp prints the help messages registered with RegisterHelpMessage.
// The context is only used for returning early during tests.
func (b *Broker) printHelpMessages(ctx context.Context) {
	b.helpMessagesMu.Lock()
	defer b.helpMessagesMu.Unlock()
	/* Print ALL the helps messages! */
	for _, hm := range b.helpMessages {
		/* Generate the message to print. */
		m, err := hm.hm()
		if nil != err {
			b.errorf(
				"",
				"%s",
				helpMessageErrorText(hm.name, err),
			)
			b.och <- opshell.CLine{Line: "\n", Plain: true}
			continue
		}
		/* Send forth. */
		b.Colorf("", opshell.ColorCyan, "%s", m)
		b.och <- opshell.CLine{Line: "\n", Plain: true}
	}
}

// helpMessageErrorText returns a message indicating a help message failed.
func helpMessageErrorText(name string, err error) string {
	return fmt.Sprintf("Error generating %s: %v", name, err)
}
