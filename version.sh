#!/usr/bin/env bash

set -e

# use GNU coreutils sort because it supports version sort
case `uname` in
(Darwin)
  sort='gsort'
  ;;
(*)
  sort='sort'
  ;;
esac

# collect all the latest versions tagged in Git for each major version
declare -A versions
while read version; do
  if [[ $version =~ ^v([0-9]+)\.[0-9]+\.[0-9]+$ ]]; then
    versions[${BASH_REMATCH[1]}]=$version
  fi
done < <(git tag -l 'v*' | $sort --version-sort)

# output YAML with top level "versions" containing a dictionary of the major version numbers as keys and latest versions
# as values
cat <<EOF
# Latest right_st versions by major version (this file is used by right_st's update check mechanism)
---
versions:
EOF
for major in ${!versions[@]}; do
  # only output versions 1 and above
  if [[ $major -ge 1 ]]; then
    echo "  $major: ${versions[$major]}"
  fi
done
