Non-Flag Configuration
======================
Aside from using [command-line flags](./flags.md) at runtime, curlrevshell can
be configured at runtime with
[environment variable~s~](#environment-variables)
and at compile-time with [linker flags](#linker-flags)

Environment Variable~s~
-----------------------
There is currently one supported environment variable.  It may go away in a
future release.

Name               | Default Value                 | Description
-------------------|-------------------------------|------------
`CURLREVSHELL_LOG` | Default logfile (i.e. `-log`) | _None_

Linker Flags
------------
Compile-time config is done with Go's `-X` linker flag.  This usually look a
bit like
```sh
go install -ldflags '-X main.Foo=bar' github.com/magisterquis/curlrevshell@dev-merge
```

The available settings are as follows:

Setting              | Default | Description
---------------------|---------|------------
`main.URLPathIn`     | `i`     | URL path for shell input connection
`main.URLPathInOut`  | `io`    | URL path for bidirectional shell connection
`main.URLPathOut`    | `o`     | URL path for shell output connection
`main.URLPathScript` | `c`     | URL path for callback script generation
