#!/usr/bin/env bash
# Echo the received JSON to stderr so tests can assert on it.
in=$(cat)
echo "$in" 1>&2
exit 0
