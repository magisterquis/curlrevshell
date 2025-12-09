# Makefile
# Update the non-OpenBSD side
# By J. Stuart McMurray
# Created 20251010
# Last Modified 20251209

all: pledgeunveil_other.go
.PHONY: all

pledgeunveil_other.go: pledgeunveil_openbsd.go
	go generate

clean:
	rm -f pledgeunveil_other.go
.PHONY: clean
