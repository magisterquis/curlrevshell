#!/usr/bin/awk -f
#
# extract_templates.awk
# Extract the templates in doc/template.md
# By J. Stuart McMurray
# Created 20250219
# Last Modified 20250302

# Run as ./extract_templates.awk -v TMPLD=$TMPLD ./doc/template.md

BEGIN {
        # Make sure we have a temporary directory.
        if ("" == TMPLD) {
                print "Please pass a directory for extraction with "\
                      "-v TMPLD=..." >"/dev/stderr"
                exit 1
        }
        # State variables.
        shb = 0  # In a syntax-highlighted block
        tln = 0  # Template line number
        tfn = "" # Template name
        tno = 0  # Template number, which turns into a name
}

# endtemplate closes tfn, if not the empty string, and resets the state
# variables.
function endtemplate() {
        # Close the template file if we have one.
        if ("" != tfn) {
                close(tfn)
        }
        # Reset the state variables.
        tfn = ""
        shb = tln = 0
}

# Backticks with syntax highlighting aren't a template, but we pay attention
# so we don't treat their closing ``` as a template's opening ```.
/^```.+$/ {
        shb = 1
        next
}

# Templates are fenced with three backticks, but with no syntax highlighting.
/^```$/ { 
        # If we were in a code block, we're not when we get the next ```.
        if (shb || ("" != tfn)) {
                endtemplate()
        # Starting a template.  Make a new filename for it.
        } else {
                tno++
                tfn = sprintf("%s/template_%d", TMPLD, tno)
        }
        next
}

# Ignore non-codeblocks and syntax-highlighted blocks.
1 == shb || "" == tfn {
        next
}

# Lines when we have a template name are part of the template.
"" != tfn {
        tln++

        # But if the first line isn't a {{ line, it's not a template.
        if ((1 == tln) && ($0 !~ /^{{/)) {
                # Remove the bogus template we've created
                ret = system("rm " tfn)
                if (0 != ret) {
                        printf "Failed to remove %s (exit %d)\n", tfn, ret;
                        exit ret;
                }
                endtemplate()
                shb = 1
                next
        }

        # Glory be, finally got a template line.
        print $0 >>tfn
}
