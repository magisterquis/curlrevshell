# subdirs.mk
# Build Go modules which take a bit of extra work
# By J. Stuart McMurray
# Created 20251209
# Last Modified 20251209

GOSUBMAKES   != find internal lib -mindepth 1 -name Makefile -type f

gosubdirs: .NOTMAIN
.for SUBDIR in ${GOSUBMAKES:H}
	+${.MAKE} -C ${SUBDIR}
.endfor
.PHONY: subdirs
