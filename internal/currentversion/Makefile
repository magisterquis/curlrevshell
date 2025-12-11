# Makefile
# Make the current branch
# By J. Stuart McMurray
# Created 20251209
# Last Modified 20251211

all: current_branch

current_branch: ../../.git/HEAD
	go generate -x

distclean:
	rm -f current_branch
clean:
.PHONY: clean distclean
