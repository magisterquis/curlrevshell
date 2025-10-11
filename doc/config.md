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
go install -ldflags '-X main.Foo=bar' github.com/magisterquis/curlrevshell@pledgeunveil
```

The available settings are as follows:

Setting                      | Default | Description
-----------------------------|---------|------------
`main.DefaultCtrlI`          | _none_  | Default value for [`-ctrl-i`](./flags.md#-ctrl-i)
`main.DefaultLog`            | _none_  | Default value for [`-log`](./flags.md#-log)
`main.DefaultServeFilesFrom` | _none_  | Default value for [`-serve-files-from`](./flags.md#-serve-files-from)
`main.DefaultTemplate`       | _none_  | Default value for [`-template`](./flags.md#-template)
`main.URLPathInOut`          | `io`    | URL path for bidirectional shell connection
`main.URLPathIn`             | `i`     | URL path for shell input connection
`main.URLPathOut`            | `o`     | URL path for shell output connection
`main.URLPathScript`         | `c`     | URL path for callback script generation

Something like the following is reasonably ok:
```sh
go build -ldflags "
    -X main.DefaultCtrlI=$HOME/crs/ctrl-i
    -X main.DefaultLog=$HOME/crs/log.json
    -X main.ServeFilesFrom=$HOME/crs/files
    -X main.DefaultTemplate=$HOME/crs/crs.tmpl
"
```
