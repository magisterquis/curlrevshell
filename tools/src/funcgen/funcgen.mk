# funcgen.mk
# Makefile which assembles shell functions
# By J. Stuart McMurray
# Created 20240528
# Last Modified 20240602

# Make sure we have all of the bits we need
.poison empty (SRCDIRS)
.poison !defined (TEMPLATEFILE)

SUBR_SUFFIX  = fg_subr
OUTFILE     ?= /dev/null

.SUFFIXES: .sh .subr .pl .$(SUBR_SUFFIX)

# Work out where to get files
SRCS  != find $(SRCDIRS) -type f ! -name '*.$(SUBR_SUFFIX)' -a ! -name '.*'
SUBRS  = $(SRCS:R:S/$/.$(SUBR_SUFFIX)/)

# Work out which way to build the output file.
.if empty(TEMPLATEFILE)
$(OUTFILE): build_ctrl_i_file
.else
$(OUTFILE): build_template
.endif

# Convert files to the proper form.
.sh.$(SUBR_SUFFIX) .subr.$(SUBR_SUFFIX):
	@cp $< $@

.pl.$(SUBR_SUFFIX):
	@perl -e "$$HEXEVALIFY_PL" $> > $@

# build_ctrl_i_file rolls all of the subroutines into a single file.
build_ctrl_i_file: $(SUBRS) .USE
	@echo "# Generated $$(date)" > $@
.       for F in $(SUBRS)
		@cat $F >> $@
.       endfor

# build_template sends all of the subroutines into a template.
build_template: $(SUBRS) $(TEMPLATEFILE) .USE
	@cat $(SUBRS) |\
		sed -e "s/'/'\\\''/g" |\
		m4 -PEE $(TEMPLATEFILE) > $@

# Clean removes generated files from the functions directories
clean:
	@find $(SRCDIRS) -name '*.$(SUBR_SUFFIX)' ! -name '.*' -delete
