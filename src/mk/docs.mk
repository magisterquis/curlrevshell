# docs.mk
# Make makefile include to build documents
# By J. Stuart McMurray
# Created 20251209
# Last Modified 20260922

# Documents to generate.
GENDOCS = README.md doc/config.md

docs: ${GENDOCS} godocs .NOTMAIN ## Update documents

# Document-builders.
.for D in ${GENDOCS}
$D: src/docs/${@F}.built .NOTMAIN
	cp $> $@

src/docs/${D:T}.built! .NOTMAIN
	+${.MAKE} -C ${@D} -q ${@F} >/dev/null || ${.MAKE} -C ${@D} ${@F}
.endfor

# Offline-friendly go doc output.
GOSRCS != find * -mindepth 1 -name '*.go' \! -name '*_test.go'
GODIRS != for D in ${GOSRCS:H:QL}; do echo $$D; done | sort -u
GODOCS  = ${GODIRS:S,$,/godoc,}
godocs: ${GODOCS} .NOTMAIN

# Individual godoc files
.for F in ${GODOCS}
$F: ${GOSRCS:M${F:H}/*.go:N${F:H}/*/*} .NOTMAIN
	go doc -all ./${@D} >$@.tmp
	mv $@.tmp $@
.endfor
