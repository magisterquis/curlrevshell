# Makefile
# Build curlrevshell
# By J. Stuart McMurray
# Created 20240323
# Last Modified 20250109

BINNAME     != basename $$(pwd)
BUILDFLAGS   = -trimpath -ldflags "-w -s"
TESTFLAGS   += -timeout 3s
VETFLAGS     = -printf.funcs 'debugf,errorf,erorrlogf,logf,printf,rerrorlogf,rlogf'
TOOLSDIR     = tools
TOOLSRCDIRS != find ./lib/*/cmd -type d -maxdepth 1 -mindepth 1


.PHONY: all build test tools help install clean

all: test tools build ## Build ALL the things (default)

${BINNAME}!
	go build ${BUILDFLAGS} -o ${BINNAME}

build: ${BINNAME} ## Build just curlrevshell

test: ## Just run tests
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

tools: ${TOOLSRCDIRS:T:S,^,${TOOLSDIR}/,} ## Build additional tools

.for TOOLSRCDIR in ${TOOLSRCDIRS}
${TOOLSDIR}/${TOOLSRCDIR:T}! ${TOOLSRCDIR}
	go build ${BUILDFLAGS} -o $@ $>
.endfor

help: .NOTMAIN ## This help
	@perl -ne '/^(\S+):.*?##\s*(.*)/&&print"$$1\t-\t$$2\n"' ${MAKEFILE_LIST}\
		| column -ts "$$(printf "\t")" | sort

install: ## Install to GOBIN ($GOPATH/bin or $HOME/go/bin)
	go install ${BUILDFLAGS}

clean: ## Remove built things
	rm -rf ${BINNAME} ${TOOLSDIR}
