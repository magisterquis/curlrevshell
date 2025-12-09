# Makefile
# Update tmplfuncs' documentation
# By J. Stuart McMurray
# Created 20250830
# Last Modified 20251209

all: godoc.txt README.md
.PHONY: all

godoc.txt: tmplfuncs.go
	go doc -all | sed '$$d' >$@

README.md: godoc.txt README.md.m4
	m4 -PEE -Dm4_godoc=${>:M*.txt} ${>:M*.m4} >$@.tmp
	mv $@.tmp $@

clean:
	rm -f godoc.txt README.md
.PHONY: clean
