# Makefile
# Build curlrevshell
# By J. Stuart McMurray
# Created 20240323
# Last Modified 20251209

BINNAME     != basename $$(pwd)
BUILDFLAGS   = -trimpath -ldflags "-w -s"
SHMOREURL    = https://raw.githubusercontent.com/magisterquis/shmore/refs/heads/master/shmore.subr
SUBMAKES    != find . -mindepth 2 -name Makefile -type f
TESTFLAGS   += -timeout 3s
TOOLSDIR     = tools
TOOLSRCDIRS != find ./lib/*/cmd -maxdepth 1 -mindepth 1 -type d
GENDOCS      = README.md doc/config.md

.PHONY: all build test tools help install clean

all: test tools build ## Build ALL the things! (and test them, default)

${BINNAME}! subdirs
	go build ${BUILDFLAGS} -o ${BINNAME}

# Document-builders.
.for D in ${GENDOCS}
$D: internal/docsrc/${@F}.built
	cp $> $@

internal/docsrc/${D:T}.built!
	make -C ${@D} ${@F}
.endfor

build: ${BINNAME} ${GENDOCS} ## Build curlrevshell and its documentation
.PHONY: build

test: subdirs ${GENDOCS} ## Run tests
	go test ${BUILDFLAGS} ${TESTFLAGS} ./...
	go vet  ${BUILDFLAGS} ./...
	! which staticcheck >/dev/null || staticcheck ./...
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
.PHONY: tools

.for TOOLSRCDIR in ${TOOLSRCDIRS}
${TOOLSDIR}/${TOOLSRCDIR:T}! subdirs ${TOOLSRCDIR}
	go build ${BUILDFLAGS} -o $@ ${>:Nsubdirs}
.endfor

update: ## Fetch the latest Shmore and up-to-date Go things
	curl --fail --no-progress-meter --output t/shmore.subr ${SHMOREURL}
	go get go
	go get -t -u
	go mod tidy

install: ## Install curlrevshell with go install
	go install ${BUILDFLAGS}

clean: ## Remove built things
.for SUBDIR in ${SUBMAKES:H}
	+${.MAKE} -C ${SUBDIR} $@
.endfor
	rm -rf ${BINNAME} ${TOOLSDIR}

help: .NOTMAIN ## This help
	@perl -ne '/^(\S+?):+.*?##\s*(.*)/&&print"$$1\t-\t$$2\n"' \
		${MAKEFILE_LIST} | column -ts "$$(printf "\t")"
