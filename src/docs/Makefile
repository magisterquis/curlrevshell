# Makefile
# Put together a README with current help text
# By J. Stuart McMurray
# Created 20251209
# Last Modified 20251211

.SUFFIXES: .m4 .built

CURRENT_BRANCH_FILE=../../internal/currentversion/current_branch

# Make files available to m4.
M4_PASTABLES=downloading_deps usage ${CURRENT_BRANCH_FILE}
.for F in ${M4_PASTABLES}
M4_ARGS += -Dm4_${F:T}="$$(<$F)"
.endfor

# Name conflict :|
.m4.built: ${M4_PASTABLES}
	m4 -PEE ${M4_ARGS} ${>:M*.m4} >$@.tmp
	mv $@.tmp $@

usage: ../../go.mod ../../go.sum ../../curlrevshell.go ${CURRENT_BRANCH_FILE}
	TD=$$(mktemp -d); TF=$$TD/curlrevshell;\
		go build -o "$$TF" ../../ &&\
		"$$TF" -h >$@.tmp 2>&1 &&\
		rm "$$TF" && rmdir "$$TD"
	mv $@.tmp $@

${CURRENT_BRANCH_FILE}: ../../.git/HEAD
	make -C ${@D} ${@F}

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
	rm -f *.built downloading_deps usage
distclean: clean
.PHONY: clean distclean
