# tools.mk
# Build other tools
# By J. Stuart McMurray
# Created 20251209
# Last Modified 20260812

TOOLSDIR  = tools
CMDDIRS  != find . -name cmd -type d
TOOLDIRS != find ${CMDDIRS} -type d -mindepth 1 -maxdepth 1
TOOLS     =

# Build ALL the tools.
.for TOOLDIR in ${TOOLDIRS}
TOOL         = ${TOOLSDIR}/${TOOLDIR:T}
TOOLS       += ${TOOLSDIR}/${TOOLDIR:T}
${TOOL}_DIR  = ${TOOLDIR}
${TOOL}! .NOTMAIN
	go build ${GOBUILDFLAGS} -o $@ ${${@}_DIR}
.endfor
# On second thought, this was perhaps a bit unkind.

tools: ${TOOLS} .NOTMAIN ## Build supporting tools
.PHONY: tools
