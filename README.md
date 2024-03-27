cURL-Powered Reverse Shell
==========================
Somewhat kooky replacement for the typical `bash >/dev/tcp...` reverse shell,
but with the following "features":

- Underlying comms via HTTPS via cURL
- Self-signed TLS cert, plus certificate pinning
- One cURL execution per output line
- Lots and lots of fork and exec
- Like, lots
- Optionally serves static files

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
[script.go](internal/hsrv/script.go).
