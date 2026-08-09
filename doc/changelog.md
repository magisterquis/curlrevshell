Changelog
=========
This lists the feature creep present in each tagged version.

`adapters`
----------
The following changes are available on this branch and will (probably) be in
the next tagged version.  Get them with
```sh
go install github.com/magisterquis/curlrevshell@adapters
```
- Logfile locking should work better with non-file files.
- Updated dependencies.

`dev`
-----
- `go` un-`tool`'d staticcheck, because hypothetical supply chain
  vulnerability.
- Automate updates to [`README.md`](./README.md) and
  [`config.md`](./doc/config.md).
- Split the [Makefile](./Makefile) into [smaller parts](./src/mk).
- `-h` makes it that much easier to see the current version.
- [`config.md`](./doc/config.md): Add a quick copy/paste-friendly one-liner for
  a quick setup with quick defaults.
- [`crstemplate`](../lib/crstemplate): Turns out some targets don't like using
  `</dev/null` with `2>&0` so we'll open it twice.
- Get rid of tests' dependencies on [`jq`](https://jqlang.org).
- [`ctxerrgroup`](./lib/ctxerrgroup): Tags and tag lineage now available
  in GoTag'd goroutines.
- Make gunked up logfiles from multiple concurrent curlrevshell processes a bit
  less likely.
- [`sstls`](./lib/sstls) - Overwriting an archive now overwrites the whole
  archive and leaves no survivors.
- [`t/extract_templates.t`](./t/extract_templates.t) now reports how many TAP
  lines it'll emit.
- Tests are a little better.
- Updated dependencies.


`v0.0.1-beta.8` (2025-11-07)
----------------------------
- [`-template`](./flags.md#-template): Template (nearly) ALL the things!
- [`crstemplate`](../lib/crstemplate): New library to make it slightly easier
  to roll fancypants `-template` templates.
- Added compile-time i/o/io/c-changey variables.
- Added the aptly-named [`config.md`](./config.md) and
  [`template.md`](./template.md).
- Minor tweaks to the callback script.
- ~Asked an excitable cow to give a warning that `-callback-template` is
  deprecated in favor of `-callback`.~
- [`opshell`](../lib/opshell): Added `Shell.RedLogf` which does what it says on
  the tin.
- Renamed `{{.URL}}` to `{{.Host}}` and added `{{.Path}`, for overengineered
  templates to serve different `{{.script}}` sections based on URL.
- [`README.md`](../README.md): No more out-of-date references to `funcgen`.
- [`-ctrl-i`](./flags.md#-ctrl-i): Fewer stray newlines and send with `SIGUSR1`
  as well as `Ctrl+I`.
- [`-ctrl-i`](./flags.md#-ctrl-i): A friendly log message is now printed on
  startup with the PID, to make it easier to use
  [fwa](https://github.com/PeterHajdu/fwa) and `kill(1)` to send
  hot-off-the-press `-ctrl-i` functions to a connected shell.
- [`Makefile`](./Makefile): Make `make help` make help.  Make `make update`
  make updates.
- Somewhat less fragile tests in [`t/`](../t/).
- If [`-template`](./flags.md#-template) names a missing template file, the
  previously footgunful `/c` script will now not be generated.  Empty template
  files work just fine.
- [`-template`](./flags.md#-template): One-liners now configurable, plus way
  more available in [Params](../lib/crstemplate/params.go).
- [`-template`](./flags.md#-template): Function available in templates now
  (kinda) [documented](../lib/crstemplate/tmplfuncs/godoc.txt).
- [`-template`](./flags.md#-template): Added `matchre`, to check if a string
  might `match`[`re`](https://github.com/google/re2/wiki/Syntax).
- Added [staticcheck](https://staticcheck.dev) as a fancy new [Go tool
  dependency](https://go.dev/doc/modules/managing-dependencies#tools) which
  should save several seconds of copy/pasting a `go install` line.
- Better checks for stray `DEBUG`/`TODO`/`TAP_TODO` comments and outdated
  package versions.
- Added [`crsdialer`](./lib/crsdialer) to somewhat simplify
  curlrevshell-dialing.
- `-h`: Invisible changes to print help a bit more nicely in strange
  conditions.
- New and improved tests which now actually run curlrevshell.
- More new and improved tests which make sure the [README](../README.md) is
  up-to-dater.
- [`-tls-certificate-cache`](./flags.md#-tls-certificate-cache): Added a
  "simple" command to generate a new cert and key.
- New friendly welcome message with the current version.
- Leave the local tty un-raw'd if stdin isn't a terminal.
- [`shellfuncsfile`](../lib/shellfuncsfile): Prevent `tab_doc()` with
  `# TABDOC:NOTABLIST`.
- `make help` now prints out nifty targets in the [`Makefile`](../Makefile).
- [`simpleshell`](../lib/simpleshell): Less racy
- [`simpleshell`](../lib/simpleshell): Use the system `cat(1)` for less-faily
  tests when the build cache is empty.
- [`ctxerrgroup`](../lib/ctxerrgroup): Slightly less labor-intensive to run a
  bunch of things but know which sent back an error.
- Make it slightly harder to accidentally a shell by muscle-memorying Ctrl+C.
- New and improve tests which now actually run curlrevshell and make sure
  dependencies are up-to-date.
- [`-serve-files-from`](./flags.md#-serve-files-from): Note that `index.html`
  prevents directory listings.
- [`-debug`](./flags.md#-debug): No more pesky TLS EOF messages (by default),
  but `-debug` brings them back.
- Welcome message now lists the branch as well to avoid the version
  number-related strabismus.
- Tests are less jealous and no longer fail when something else is listening
  on port 4444.
- [`-ctrl-i`](./flags.md#-ctrl-i): Switch from `Ctrl+J` to `Ctrl+S`.
- [`-debug`](./flags.md#-debug): Debugged DEBUG messages with -debug.
- [`-log`](./flags.md#-log): Log the TLS fingerprint as well.
- Long invocations and excessive horizontal scrolling finally led to
  [compile-time defaults](./doc/config.md#linker-flags) for flags.
- [`tmplfuncs`](../lib/crstemplate/tmplfuncs): Go -> Perl, less test panic
- [`t/template.t`](./t/template.t): Quite a bit less flakey in slow (read:
  OpenBSD in a VM) situations.
- [`pledgeunveil`](../lib/pledgeunveil): Thin wrapper around OpenBSD's
  [`pledge(2)`](https://man.openbsd.org/pledge.2) and
  [`unveil(2)`](https://man.openbsd.org/unveil.2) which, aside from adding a
  fancy `pU` in [`ps awwwfux`](https://man.openbsd.org/ps.1) output, should
  mean quite a bit less can go sideways.  On OpenBSD, though.  No-op on other
  OSs.
- [`-ctrl-i`](./flags.md#-ctrl-i): Use `*.ctrl-i` as a pattern for a generic
  (read not-vim-default-syntax-highlighted) file, along with `*.subr` and
  `*.sh`.
- Finally removed `-callback-template`.
- Updated dependencies.


`v0.0.1-beta.7` (2024-10-22)
----------------------------
- `-callback-template`: Missing templates are probably not what you want.  Red
  text should help.
- Extracting Shell I/O from logs with [`jq`](https://jqlang.github.io/jq/) is
  quite a bit less painful.
- Tests are less racy.
- [`-ctrl-i`](./flags.md#-ctrl-i): In-memory, over-the-wire, on-demand module
  loading.  Or just sending the contents of a file (or directory or magically
  shellified Perl script) to the remote shell.  Trigger it with `Ctrl+I`, or
  use `Ctrl+J` to just see what `Ctrl+I` would send.
- [`shellfuncsfile`](../lib/shellfuncsfile): a nifty library to roll a
  file or directory into a single gob of shell functions; does a lot of
  `-ctrl-i`'s heavy lifting.
- Updated dependencies.
- [`shellfuncsfile` the tool](../lib/shellfuncsfile/cmd/shellfuncsfile): A
  wrapper around [`shellfuncsfile` the library](../lib/shellfuncsfile) to
  generate shell functions files like [`-ctrl-i`](./flags.md#-ctrl-i) does, but
  standalone.
- [`-print-ctrl-i`](./flags.md#-print-ctrl-i): Like [`shellfuncsfile`
  the tool](../lib/shellfuncsfile/cmd/shellfuncsfile), but
  without having to remember to build and run something else
- [`uu`](../lib/uu): Little library to do uuencoding/uudecoding, compatibly
  with perl's `pack("u", ...)`.
- Updated dependencies.
- Fewer half-disconnected shells which require a couple `return`s to get
  rid of, and less Ctrl+Cing shells which don't know they're dead.
- Better testing for half-dead shells.
- [`opshell`](../lib/opshell): Added a couple of functions to make testing
  a bit easier.
- [`chanlog`](../lib/chanlog): It's a
  [slog.Logger](https://pkg.go.dev/log/slog#Logger).  It's also a channel.
  It's also an easier way to test logging.
- [`/io`]: Added an HTTP endpoint for when you'd kinda prefer a single
  connection over two parallel connections.
- [`simpleshell`](./lib/simpleshell): A small library for connecting up a shell
  or other such thing with minimal effort.
- [`simpleshell` the program](./lib/simpleshell/cmd/simpleshell): A wrapper
  around [`simpleshell` the library](./lib/simpleshell/) which can be built as
  a standalone binary or an injectable shared object file.
  It's about the closest thing there is to a proper Curlrevshell implant.


`v0.0.1-beta.6` (2024-05-22)
----------------------------
- No more blank lines or repeated comamnds when up-arrowing.
- Option+Left/Right works, at least in iTerm2/Terminal.app on a Mac.
- Ctrl+O mutes output until it calms down, as requested by someone who found
  every logfile on target being printed to his screen.
- `-one-shell`: Stop listening after a shell connects, like a nicer `-k`-less
  netcat.
- No more pesky quotes around the cyan callback/file URL lines.


`v0.0.1-beta.5`
---------------
- Actually try to put the generated TLS certificate in `$HOME` if we can't find
  a cache directory.
- Change date on LICENSE, only four months late.
- Put the certificate in a directory called `sstls` and set the CN to `sstls`,
  to make it that much easier to reuse `lib/sstls` in other code and have the
  same copy/pastable fingerprint.  This will probably change again.
- The `-callback-template` callback template is now re-read every time it's
  needed, for on-the-fly ~debugging~ good ideas.
- Make sure input and output disconnect together, which means no hitting enter
  a couple of times before the next callback.
- Print the curl one-liners every time the shell dies, which means no more
  Ctrl+D after hitting enter a couple of times.
- Don't serve up `Eek!`s anymore, just in case someone piped one to a shell
  after Ctrl+D after hitting enter a coule of times.
- Tests are a little less crashy.


`v0.0.1-beta.4`
---------------
- `-icanhazip`: Guess callback address using [icanhazip.com](https://icanhazip.com).
- `-log`: JSON logging.  Do _you_ remember what you did a month ago?
- [`flags.md`](./flags.md): Do _I_ remember what all these flags do?
- Output uses one long-lived cURL process.  Way faster.  No left-justification.
- Updated dependencies.
