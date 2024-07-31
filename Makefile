# Makefile
# Build curlrevshell
# By J. Stuart McMurray
# Created 20240323
# Last Modified 20240731

BINNAME     != basename $$(pwd)
BUILDFLAGS   = -trimpath -ldflags "-w -s"
TESTFLAGS   += -timeout 3s
VETFLAGS     = -printf.funcs 'debugf,errorf,erorrlogf,logf,printf,rerrorlogf,rlogf'
TOOLSDIR     = tools
TOOLSRCDIRS != find ./lib/*/cmd -type d -maxdepth 1 -mindepth 1


.PHONY: all test install clean

all: test tools build

${BINNAME}!
	go build ${BUILDFLAGS} -o ${BINNAME}

build: ${BINNAME}

test:
	go test ${BUILDFLAGS} ${TESTFLAGS} ./...
	go vet  ${BUILDFLAGS} ${VETFLAGS} ./...
	staticcheck ./...
	go run ${BUILDFLAGS} . -h 2>&1 |\
	awk '\
		/^Options:$$|MQD DEBUG PACKAGE LOADED$$/\
			{ exit }\
		/^Usage: /\
			{ sub(/^Usage: [^[:space:]]+\//, "Usage: ") }\
		/.{80,}/\
			{ print "Long usage line: " $0; exit 1 }\
	'

tools: ${TOOLSRCDIRS:T:S,^,${TOOLSDIR}/,}

.for TOOLSRCDIR in ${TOOLSRCDIRS}
${TOOLSDIR}/${TOOLSRCDIR:T}! ${TOOLSRCDIR}
	go build ${BUILDFLAGS} -o $@ $>
.endfor


install:
	go install ${BUILDFLAGS}

clean:
	rm -rf ${BINNAME} ${TOOLSDIR}
