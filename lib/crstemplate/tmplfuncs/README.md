Template Functions
==================
This module contains functions available in `-template` templates.

Documentation for each function is in
[doc/template.md](../../../doc/template.md).

GoDoc
------
```text
package tmplfuncs // import "github.com/magisterquis/curlrevshell/lib/crstemplate/tmplfuncs"

Package tmplfuncs contains the functions available to -template templates.

Template -> Go function name mappings are in TemplateFuncs.

CONSTANTS

const DefaultPort = "443"
    DefaultPort is used when we need a port and have no better options.


VARIABLES

var TemplateFuncs = template.FuncMap{
	"ensureport":    EnsurePort,
	"host":          Host,
	"map":           Map,
	"matchre":       MatchRE,
	"nodefaultport": NoDefaultPort,
	"port":          Port,
}
    TemplateFuncs are functions added to templates' function map.


FUNCTIONS

func EnsurePort(port, addr string) string
    EnsurePort takes an address and ensures it contains a port. If addr is the
    empty string, the empty string will be returned. If port is the empty string
    DefaultPort is used.

func Host(addr string) (string, error)
    Host takes an IP:port or host:port and returns just the IP or host. It is a
    wrapper around net.SplitHostPort but will return the host even if there is
    no port.

func Map(kvs ...any) (map[string]any, error)
    Map assembles its arguments, which must be key/value pairs into a map,
    for easier passing multiple values to subtemplates. Totally not a knockoff
    of Sprig's dict...

func MatchRE(re, s string) (bool, error)
    MatchRE returns true if the regular expression re matches the string s.

func NoDefaultPort(addr string) string
    NoDefaultPort takes a host:port or IP:port and returns it without the port
    if the port was DefaultPort. If addr has no port to begin with or can't be
    parsed into an address and port, it is returned as-is.

func Port(addr string) (string, error)
    Port takes an IP:port or host:port and returns just the port. If there was
    no port, Port returns DefaultPort.
```
