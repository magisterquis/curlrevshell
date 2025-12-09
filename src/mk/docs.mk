# docs.mk
# Make makefile include to build documents
# By J. Stuart McMurray
# Created 20251209
# Last Modified 20251209

# Documents to generate.
GENDOCS = README.md doc/config.md

docs: ${GENDOCS} .NOTMAIN ## Update documents

# Document-builders.
.for D in ${GENDOCS}
$D: src/docs/${@F}.built .NOTMAIN
	cp $> $@

src/docs/${D:T}.built! .NOTMAIN
	+${.MAKE} -C ${@D} ${@F}
.endfor
