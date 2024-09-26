Changelog
=========
This lists the feature creep present in each tagged version.

`betterdisconnects`
===================
The following changes are available on this branch and will (probably) be
in the next tagged version.  Get them with
```sh
go install github.com/magisterquis/curlrevshell@betterdisconnects
```
- Fewer half-disconnected shells which require a couple `return`s to get
  rid of, and less Ctrl+Cing shells which don't know they're dead.
- Better testing for half-dead shells.
- [`opshell`](../lib/opshell): Added a couple of functions to make testing
  a bit easier.
- [`chanlog`](../lib/chanlog): It's a
  [slog.Logger](https://pkg.go.dev/log/slog#Logger).  It's also a channel.
  It's also an easier way to test logging.

`v0.0.1-beta.6` (2024-05-22)
============================
- No more blank lines or repeated comamnds when up-arrowing.
- Option+Left/Right works, at least in iTerm2/Terminal.app on a Mac.
- Ctrl+O mutes output until it calms down, as requested by someone who found
  every logfile on target being printed to his screen.
- `-one-shell`: Stop listening after a shell connects, like a nicer `-k`-less
  netcat.
- No more pesky quotes around the cyan callback/file URL lines.


`v0.0.1-beta.5`
===============
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
===============
- `-icanhazip`: Guess callback address using [icanhazip.com](https://icanhazip.com).
- `-log`: JSON logging.  Do _you_ remember what you did a month ago?
- [`flags.md`](./flags.md): Do _I_ remember what all these flags do?
- Output uses one long-lived cURL process.  Way faster.  No left-justification.
- Updated dependencies.
