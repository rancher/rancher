#!/bin/bash

# this is used for local test as fluentd may not be installed
# no validation is performed just the invocation parameters are printed

if [[ $1 == "--version" ]]; then
  echo "fake-fluentd 1.0"
  exit 0
fi

file="${@: -1}"
echo
echo Invoked with "$@"
echo __________________

cat "$file"

echo __________________

# for unit tests: if the input contains #ERROR, exit with 1
if grep 'ERROR' < "$file" ; then
  exit 1
fi

exit 0
