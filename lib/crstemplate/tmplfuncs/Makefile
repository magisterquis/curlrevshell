# Makefile
# Update tmplfuncs' documentation
# By J. Stuart McMurray
# Created 20250830
# Last Modified 20261017

.MAKE        ?= ${MAKE}

all: godoc.txt README.md
.PHONY: all

godoc.txt: tmplfuncs.go
	${.MAKE} -C ../../.. lib/crstemplate/tmplfuncs/godoc.txt

README.md: godoc.notl.txt README.md.m4
	m4 -PEE -Dm4_godoc=${>:M*.txt} ${>:M*.m4} >$@.tmp
	mv $@.tmp $@

# Go doc output with no trailing blank lines
godoc.notl.txt: godoc.txt
	perl -e '@a=<>;pop@a while("\n"eq$$a[-1]);print@a' $> >$@.tmp
	mv $@.tmp $@

distclean: clean
	rm -f godoc.txt README.md
clean:
	rm -f godoc.notl.txt
.PHONY: clean distclean
