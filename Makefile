# Makefile
# Build curlrevshell
# By J. Stuart McMurray
# Created 20240323
# Last Modified 20240707

BINNAME    != basename $$(pwd)
BUILDFLAGS  = -trimpath -ldflags "-w -s"
CRSSRCS    != find lib internal -type f -o -type d
TESTFLAGS  += -timeout 3s
TOOLSDIR    = tools
VETFLAGS    = -printf.funcs 'debugf,errorf,erorrlogf,logf,printf,rerrorlogf,rlogf'

.PHONY: all test install clean

all: test build

${BINNAME}: *.go go.* ${CRSSRCS}
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

install:
	go install ${BUILDFLAGS}

clean:
	rm -f ${BINNAME}
