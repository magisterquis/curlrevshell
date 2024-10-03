Test Data
=========
The files in this directory represent uuencoded/decode pairs.  Each is a txtar
archive containing two:

Name   | Description
-------|------------
enc    | uUencoded data
dec    | uNencoded data
decerr | The `err.Error()` returned from decoding `dec`

The archive comment may consist of the following directives, one per line

Directive   | Description
------------|------------
noencode    | Don't test encoding
nomaxenclen | Don't test MaxEncodedLen

Tests
-----
How the data is tested depends on which files are present.

Files              Description
-----------------|------------
`enc` / `dec`    | Ensures that `dec` decodes to `enc` and `enc` decodes to `dec`
`dec` / `encerr` | Ensures that encoding `dec` returns `encerr`
`enc` / `decerr` | Ensures that decoding `enc` returns `decerr`

Any other combination is an error.

Perl One-liners
---------------
Encoding:
```sh
perl -0777 -ne 'print pack "u", $_'
```
Decoding:
```sh
perl -0777 -ne 'print unpack "u", $_'
```
