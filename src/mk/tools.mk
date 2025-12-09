# tools.mk
# Build other tools
# By J. Stuart McMurray
# Created 20251209
# Last Modified 20251209

TOOLSDIR  = tools
TOOLS    != find\
	./lib/*/cmd\
	-maxdepth 1\
	-mindepth 1\
	-type d
TOOLS    := ${TOOLS:T:T:S,^,${TOOLSDIR}/,}

tools: ${TOOLS} .NOTMAIN ## Build supporting tools
.PHONY: tools

# Build ALL the tools.
.for TOOL in ${TOOLS}
${TOOL}! .NOTMAIN
	go build ${GOBUILDFLAGS} -o $@ ./lib/${@F}/cmd/${@F}
.endfor
