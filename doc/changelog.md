Changelog
=========
This lists the feature creep present in each tagged version.

`master`
========
The following changes are available on the master branch and will (probably) be
in the next tagged version.  Get them with
```sh
go install github.com/magisterquis/curlrevshell@master
```
- Actually try to put the generated TLS certificate in `$HOME` if we can't find
  a cache directory.
- Change date on LICENSE, only four months late.
- Put the certificate in a directory called `sstls` and set the CN to `sstls`,
  to make it that much easier to reuse `lib/sstls` in other code and have the
  same copy/pastable fingerprint.


`v0.0.1-beta.4`
===============
- `-icanhazip`: Guess callback address using [icanhazip.com](https://icanhazip.com).
- `-log`: JSON logging.  Do _you_ remember what you did a month ago?
- [`flags.md`](./flags.md): Do _I_ remember what all these flags do?
- Output uses one long-lived cURL process.  Way faster.  No left-justification.
- Updated dependencies.
