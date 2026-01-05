# Makefile
# Put together a README with current help text
# By J. Stuart McMurray
# Created 20251209
# Last Modified 20260105

.SUFFIXES: .m4 .built

SRCS != ls *.m4

CURRENT_BRANCH_FILE=../../internal/currentversion/current_branch

# Our local current_branch is out of date if it's not the same as the real
# current_branch
.BEGIN:
	@if [[ -f ${CURRENT_BRANCH_FILE} && -f current_branch ]] &&\
		! diff -q ${CURRENT_BRANCH_FILE} current_branch >/dev/null;\
	then\
		touch -t 197001010000 current_branch;\
	fi

all: ${SRCS:R:S/$/.built/}
.PHONY: all

# Name conflict :|
.m4.built: current_branch downloading_deps usage
	m4\
		-PEE\
		-Dm4_current_branch="$$(<current_branch)"\
		-Dm4_usage="$$(<usage)"\
		-Dm4_downloading_deps="$$(<downloading_deps)"\
		${>:M*.m4} >$@.tmp
	mv $@.tmp $@

usage: ../../go.mod ../../go.sum ../../curlrevshell.go
	go run ../../ -h >$@.tmp 2>&1
	mv $@.tmp $@

${CURRENT_BRANCH_FILE}:
	make -C ${@D} ${@F}

current_branch: ${CURRENT_BRANCH_FILE}
	cp $> $@

downloading_deps: ../../go.mod ../../go.sum
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

clean:
	rm -f *.built downloading_deps usage current_branch
distclean: clean
.PHONY: clean distclean
