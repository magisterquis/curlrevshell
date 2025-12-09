# Makefile
# Build curlrevshell
# By J. Stuart McMurray
# Created 20240323
# Last Modified 20251209

BINNAME      != basename $$(pwd)
GOBUILDFLAGS  = -trimpath -ldflags "-w -s"
SHMOREURL     = https://raw.githubusercontent.com/magisterquis/shmore/refs/heads/master/shmore.subr
SUBMAKES     != find * -mindepth 1 -name Makefile -type f
TESTFLAGS    += -timeout 3s

.include "src/mk/docs.mk"
.include "src/mk/tools.mk"
.include "src/mk/gosubdirs.mk"

all: test .WAIT docs build tools ## Build ALL the things! (and test them, default)
.PHONY: all

build: ${BINNAME} ## Build curlrevshell
.PHONY: build

${BINNAME}! gosubdirs
	go build ${GOBUILDFLAGS} -o ${BINNAME}

test: gosubdirs docs ## Run tests
	go test ${GOBUILDFLAGS} ${TESTFLAGS} ./...
	go vet  ${GOBUILDFLAGS} ./...
	! which staticcheck >/dev/null || staticcheck ./...
	go run ${GOBUILDFLAGS} . -h 2>&1 |\
	awk '\
		/^Options:$$|MQD DEBUG PACKAGE LOADED$$/\
			{ exit }\
		/^Usage: /\
			{ sub(/^Usage: [^[:space:]]+\//, "Usage: ") }\
		/.{80,}/\
			{ print "Long usage line: " $0; exit 1 }\
	'
	prove -It --directives
.PHONY: test

update: ## Fetch the latest Shmore and up-to-date Go things
	curl --fail --no-progress-meter --output t/shmore.subr ${SHMOREURL}
	go get go
	go get -t -u
	go mod tidy
	${.MAKE} docs
.PHONY: update

install: ## Install curlrevshell with go install
	go install ${GOBUILDFLAGS}
.PHONY: install

clean:     subclean ## Remove built things
distclean: subclean ## Remove more built things

subclean: .USE
.for SUBDIR in ${SUBMAKES:H}
	+${.MAKE} -C ${SUBDIR} $@
.endfor
	rm -rf ${BINNAME} ${TOOLSDIR}
.PHONY: clean

help: .NOTMAIN ## This help
	@perl -ne '/^(\S+?):+.*?##\s*(.*)/&&print"$$1\t-\t$$2\n"' \
		${MAKEFILE_LIST} | column -ts "$$(printf "\t")"
.PHONY: help
