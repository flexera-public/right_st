#!/usr/bin/env bash

set -e

# use GNU coreutils sort because it supports version sort
case $(uname) in
(Darwin)
  sed='gsed'
  sort='gsort'
  ;;
(*)
  sed='sed'
  sort='sort'
  ;;
esac

# find the latest version tagged in Git
while read -r version; do
  if [[ $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    break
  fi
done < <(git tag -l 'v*' | $sort --reverse --version-sort)

# create a GitHub release from the latest version tagged in Git with ChangeLog entries and pre-compiled binary links
hub release create --file - "$version" <<EOF
$version Release

$($sed -nre "/^${version//./\\.} \/ .+\$/,/^$/{/^${version//./\\.} \/ .+\$/d;/^-+$/d;p}" ChangeLog.md)

Pre-compiled binaries:
* Linux: [$version/right_st-linux-amd64.tgz](https://binaries.rightscale.com/rsbin/right_st/$version/right_st-linux-amd64.tgz)
* macOS Intel: [$version/right_st-darwin-amd64.tgz](https://binaries.rightscale.com/rsbin/right_st/$version/right_st-darwin-amd64.tgz)
* macOS ARM64: [$version/right_st-darwin-arm64.tgz](https://binaries.rightscale.com/rsbin/right_st/$version/right_st-darwin-arm64.tgz)
* Windows: [$version/right_st-windows-amd64.zip](https://binaries.rightscale.com/rsbin/right_st/$version/right_st-windows-amd64.zip)
EOF
