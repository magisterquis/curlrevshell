#!/bin/ksh
#
# print_pid_and_exec.sh
# Prints the shell's pid and execs "$@", for getting go run's child's pid
# By J. Stuart McMurray
# Created 20260819
# Last Modified 20260819

# Print this shell process's pid.
echo "$$"
# Replace with whatever the arguments have for us.
exec "$@"
