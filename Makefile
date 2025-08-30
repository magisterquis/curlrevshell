# Makefile
# Build curlrevshell
# By J. Stuart McMurray
# Created 20240323
# Last Modified 20250830

BINNAME     != basename $$(pwd)
BUILDFLAGS   = -trimpath -ldflags "-w -s"
SHMOREURL    = https://raw.githubusercontent.com/magisterquis/shmore/refs/heads/master/shmore.subr
SUBMAKES    != find . -mindepth 2 -type f -name Makefile
TESTFLAGS   += -timeout 3s
TOOLSDIR     = tools
TOOLSRCDIRS != find ./lib/*/cmd -type d -maxdepth 1 -mindepth 1

.PHONY: all test install clean

all: test tools build ## Build ALL the things! (and test them, default)

${BINNAME}! subdirs
	go build ${BUILDFLAGS} -o ${BINNAME}

build: ${BINNAME} ## Build curlrevshell
.PHONY: build

test: subdirs ## Run tests
	go test ${BUILDFLAGS} ${TESTFLAGS} ./...
	go vet  ${BUILDFLAGS} ./...
	go tool staticcheck ./...
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

subdirs:
	@echo "Making in subdirectories..."
.for SUBDIR in ${SUBMAKES:H}
	+${.MAKE} -C ${SUBDIR}
.endfor
	@echo "Finished making in subdirectories"
.PHONY: subdirs

tools: ${TOOLSRCDIRS:T:S,^,${TOOLSDIR}/,} ## Build supporting tools

.for TOOLSRCDIR in ${TOOLSRCDIRS}
${TOOLSDIR}/${TOOLSRCDIR:T}! subdirs ${TOOLSRCDIR}
	go build ${BUILDFLAGS} -o $@ $>
.endfor

update: ## Fetch the latest Shmore and up-to-date Go things
	curl --fail --no-progress-meter --output t/shmore.subr ${SHMOREURL}
	go get go
	go get -t -u
	go get -t -u tool
	go mod tidy

install: ## Install curlrevshell with go install
	go install ${BUILDFLAGS}

clean: ## Remove built things
	rm -rf ${BINNAME} ${TOOLSDIR}

help: .NOTMAIN ## This help
	@perl -ne '/^(\S+?):+.*?##\s*(.*)/&&print"$$1\t-\t$$2\n"' \
		${MAKEFILE_LIST} | column -ts "$$(printf "\t")"
