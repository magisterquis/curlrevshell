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
go install -ldflags '-X main.Foo=bar' github.com/magisterquis/curlrevshell@dev
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

### Reasonably Ok Setup
Something like the following is a reasonably ok way to get set up quickly:
```sh
mkdir -p $HOME/crs/{ctrl-i,files}                        # Directories in which store curlrevshell things
go install -v -trimpath -ldflags "
    -w -s
    -X main.DefaultCtrlI=$HOME/crs/ctrl-i
    -X main.DefaultLog=$HOME/crs/log.json
    -X main.DefaultServeFilesFrom=$HOME/crs/files
    -X main.DefaultTemplate=$HOME/crs/crs.tmpl
" github.com/magisterquis/curlrevshell@dev
# ^ Install, setting defaults to $HOME/crs
touch $HOME/crs/crs.tmpl                                 # Default template, to prevent whining
curlrevshell -h                                          # For just in case
curlrevshell                                             # Ready to go :)
```
