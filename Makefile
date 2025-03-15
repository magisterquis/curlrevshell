# Makefile
# Build curlrevshell
# By J. Stuart McMurray
# Created 20240323
# Last Modified 20250314

BINNAME     != basename $$(pwd)
BUILDFLAGS   = -trimpath -ldflags "-w -s"
SHMOREURL    = https://raw.githubusercontent.com/magisterquis/shmore/refs/heads/master/shmore.subr
TESTFLAGS   += -timeout 3s
TOOLSDIR     = tools
TOOLSRCDIRS != find ./lib/*/cmd -type d -maxdepth 1 -mindepth 1
VETFLAGS     = -printf.funcs 'debugf,errorf,erorrlogf,logf,printf,rerrorlogf,rlogf'


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
	prove -It --directives

tools: ${TOOLSRCDIRS:T:S,^,${TOOLSDIR}/,}

.for TOOLSRCDIR in ${TOOLSRCDIRS}
${TOOLSDIR}/${TOOLSRCDIR:T}! ${TOOLSRCDIR}
	go build ${BUILDFLAGS} -o $@ $>
.endfor

update: ## Fetch the latest Shmore and up-to-date Go things
	curl --fail --no-progress-meter --output t/shmore.subr ${SHMOREURL}
	go get go
	go get -u
	go mod tidy

install:
	go install ${BUILDFLAGS}

clean:
	rm -rf ${BINNAME} ${TOOLSDIR}

help: .NOTMAIN ## This help
	@perl -ne '/^(\S+?):+.*?##\s*(.*)/&&print"$$1\t-\t$$2\n"' \
		${MAKEFILE_LIST} | column -ts "$$(printf "\t")"
