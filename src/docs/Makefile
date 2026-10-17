# Makefile
# Put together a README with current help text
# By J. Stuart McMurray
# Created 20251209
# Last Modified 20261017

.SUFFIXES: .m4 .built

CURRENT_BRANCH_FILE  = ../../internal/currentversion/current_branch
PLEDGEUNVEIL_OTHER   = ../../lib/pledgeunveil/pledgeunveil_other.go
SRCS                != ls *.m4

# Our local current_branch is out of date if it's not the same as the real
# current_branch.
.BEGIN:
	@if [[ -f ${CURRENT_BRANCH_FILE} && -f current_branch ]] &&\
		! diff -q ${CURRENT_BRANCH_FILE} current_branch >/dev/null;\
	then\
		touch -t 197001010000 current_branch;\
	fi

all: ${SRCS:R:S/$/.built/}
.PHONY: all

# Make files available to m4.
M4_PASTABLES=downloading_deps usage current_branch
.for F in ${M4_PASTABLES}
M4_ARGS += -Dm4_$F="$$(<$F)"
.endfor

# Name conflict :|
.m4.built: ${M4_PASTABLES}
	m4 -PEE ${M4_ARGS} ${>:M*.m4} >$@.tmp
	mv $@.tmp $@

usage: ../../go.mod ../../go.sum ../../curlrevshell.go ${CURRENT_BRANCH_FILE}
usage: ${PLEDGEUNVEIL_OTHER}
	TD=$$(mktemp -d); TF=$$TD/curlrevshell;\
		go build -o "$$TF" ../../ &&\
		"$$TF" -h >$@.tmp 2>&1 &&\
		rm "$$TF" && rmdir "$$TD"
	mv $@.tmp $@

# Things made from other things.
${CURRENT_BRANCH_FILE}: ../../.git/HEAD call_make

${PLEDGEUNVEIL_OTHER}: call_make

current_branch: ${CURRENT_BRANCH_FILE}
	cp $> $@

downloading_deps: ../../go.mod ../../go.sum ${CURRENT_BRANCH_FILE}
	go list -deps -f '\
		{{- define "M"}}go: downloading {{.Path}} {{.Version}}{{end -}}\
		{{- with .Module -}}\
			{{- if not .Main -}}\
				{{- if .Replace -}}\
					{{template "M" .Replace}}\
				{{- else -}}\
					{{template "M" .}}\
				{{- end -}}\
			{{- end -}}\
		{{- end -}}\
	' ../../ | sort -u >$@.tmp
	mv $@.tmp $@

# Call a makefile in another directory
call_make: .USE
	${MAKE} -C ${@D} ${@F}


clean:
	rm -f *.built downloading_deps usage current_branch
distclean: clean
.PHONY: clean distclean
