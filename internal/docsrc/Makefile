# Makefile
# Put together a README with current help text
# By J. Stuart McMurray
# Created 20251209
# Last Modified 20251209

.SUFFIXES: .m4 .built

# Name conflict :|
.m4.built: ../currentversion/current_branch downloading_deps usage
	m4\
		-PEE\
		-Dm4_current_branch="$$(<${>:M*/current_branch})"\
		-Dm4_usage="$$(<usage)"\
		-Dm4_downloading_deps="$$(<downloading_deps)"\
		${>:M*.m4} >$@.tmp
	mv $@.tmp $@

usage: ../../go.mod ../../go.sum ../../curlrevshell.go
	go run ../../ -h >$@.tmp 2>&1
	mv $@.tmp $@

../currentversion/current_branch:
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
