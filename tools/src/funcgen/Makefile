# Makefile
# Generate a -callback-template template
# By J. Stuart McMurray
# Created 20240524
# Last Modified 20240602

BIN   = ../../$(.CURDIR:T)
SRCS != find . ! -path $(BIN) -a ! -name '.*'

# Build funcgen
$(BIN): $(SRCS)
	m4 -PEE funcgen.sh.m4 > $@
	chmod 0755 $@

# Remove heredoc files and the built template
.PHONY: clean
clean:
	rm -f $(BIN) functions/*.fg_subr
