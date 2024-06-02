# Makefile
# Build curlrevshell
# By J. Stuart McMurray
# Created 20240323
# Last Modified 20240602

BINNAME    != basename $$(pwd)
BUILDFLAGS  = -trimpath -ldflags "-w -s"
CRSSRCS    != find lib internal -type f -o -type d
TESTFLAGS  += -timeout 3s
TOOLSDIR    = tools
VETFLAGS    = -printf.funcs 'debugf,errorf,erorrlogf,logf,printf,rerrorlogf,rlogf'

.PHONY: all test install tools clean

all: test build tools

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

clean::
	rm -f ${BINNAME}

# List of tools to build when asked
TOOLSRCDIRS != find $(TOOLSDIR)/src -type d -mindepth 1 -maxdepth 1
tools: $(TOOLSRCDIRS:S,src/,,)

# Build ALL the tools/
.for TSD in $(TOOLSRCDIRS)

# Build a tool when its dependencies change
DEPS != find $(TSD) -type f ! -name '*.swp' -a ! -path $(TSD)/$(TSD:T)
$(TOOLSDIR)/$(TSD:T): $(DEPS)
	$(.MAKE) -C "$(TOOLSDIR)/src/$(@:T)" BIN="$(.CURDIR)/$@"

# When we make clean, clean up this tools' files as well.
clean::
	$(.MAKE) -C $(TSD) clean

.endfor
