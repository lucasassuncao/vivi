#!/usr/bin/env sh
# Prints the `## ` help text of every makefile passed as an argument. A script
# so awk's $1, $2 and \033 need no escaping past make; grep -h so the filename
# prefix of a multi-file match does not become part of the target name.
grep -hE '^[a-zA-Z_-]+:.*?## .*$' "$@" |
	sort -t: -k1,1 |
	awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $1, $2}'
