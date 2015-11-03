#! /usr/bin/make
#
# Makefile for Golang projects. Dependent on Go 1.5 + experimental vendor support
#
# Features:
# - runs ginkgo tests recursively, computes code coverage report
# - code coverage ready for travis-ci to upload and produce badges for README.md
# - build for linux/amd64, darwin/amd64, windows/amd64
# - produces .tgz/.zip build output
# - bundles *.sh files in ./script subdirectory
# - produces version.go for each build with string in global variable VV, please
#   print this using a --version option in the executable
# - to include the build status and code coverage badge in CI use (replace NAME by what
#   you set $(NAME) to further down, and also replace magnum.travis-ci.com by travis-ci.org for
#   publicly accessible repos [sigh]):
#   [![Build Status](https://magnum.travis-ci.com/rightscale/NAME.svg?token=4Q13wQTY4zqXgU7Edw3B&branch=master)](https://magnum.travis-ci.com/rightscale/NAME
#   ![Code Coverage](https://s3.amazonaws.com/rs-code-coverage/NAME/cc_badge_master.svg)
#
# Top-level targets:
# default: compile the program, you can thus use make && ./NAME -options ...
# build: builds binaries for linux and darwin
# test: runs unit tests recursively and produces code coverage stats and shows them
# travis-test: just runs unit tests recursively
# depend: calls glide up to install dependencies
# clean: removes build stuff
#
# the upload target is used in the .travis.yml file and pushes binary archives to
# https://$(BUCKET).s3.amazonaws.com/rsbin/$(NAME)/$(BRANCH)/$(NAME)-$(GOOS)-$(GOARCH).tgz
# (.zip for windows)
#

#NAME=$(shell basename $$PWD)
NAME=right_st
BUCKET=rightscale-binaries
ACL=public-read
# version for gopkg.in, e.g. v1, v2, ...
GOPKG_VERS=v1
# Dependencies handled by go+glide. Requires 1.5+
export GO15VENDOREXPERIMENT=1
# github.com/rogpeppe/gover requires auth?
#=== below this line ideally remains unchanged, add new targets at the end  ===

TRAVIS_BRANCH?=dev
DATE=$(shell date '+%F %T')
SECONDS=$(shell date '+%s')
TRAVIS_COMMIT?=$(shell git symbolic-ref HEAD | cut -d"/" -f 3)
GIT_BRANCH:=$(shell git symbolic-ref --short -q HEAD || echo "master")
SHELL:=/bin/bash

# the default target builds a binary in the top-level dir for whatever the local OS is
default: $(NAME)
$(NAME): *.go version
	go build -o $(NAME) .

install: $(NAME)
	go install

# the standard build produces a "local" executable, a linux tgz, and a darwin (macos) tgz
build: $(NAME) build/$(NAME)-linux-amd64.tgz build/$(NAME)-darwin-amd64.tgz build/$(NAME)-windows-amd64.zip

# create a tgz with the binary and any artifacts that are necessary
# note the hack to allow for various GOOS & GOARCH combos
build/$(NAME)-%.tgz: *.go version
	rm -rf build/$(NAME)
	mkdir -p build/$(NAME)
	tgt=$*; GOOS=$${tgt%-*} GOARCH=$${tgt#*-} go build  -o build/$(NAME)/$(NAME) .
	chmod +x build/$(NAME)/$(NAME)
	tar -zcf $@ -C build $(NAME)
	rm -r build/$(NAME)

# create a zip with the binary and any artifacts that are necessary
# note the hack to allow for various GOOS & GOARCH combos, sigh
build/$(NAME)-%.zip: *.go version
	rm -rf build/$(NAME)
	mkdir -p build/$(NAME)
	tgt=$*; GOOS=$${tgt%-*} GOARCH=$${tgt#*-} go build -o build/$(NAME)/$(NAME).exe .
	cd build; zip -r $(notdir $@) $(NAME)
	rm -r build/$(NAME)

# upload assumes you have AWS_ACCESS_KEY_ID and AWS_SECRET_KEY env variables set,
# which happens in the .travis.yml for CI
upload:
	@which gof3r >/dev/null || (echo 'Please "go get github.com/rlmcpherson/s3gof3r/gof3r"'; false)
	(cd build; set -ex; \
	  for f in *.tgz *.zip; do \
	    gof3r put --no-md5 --acl=$(ACL) -b ${BUCKET} -k rsbin/$(NAME)/$(TRAVIS_COMMIT)/$$f <$$f; \
	    if [ "$(TRAVIS_PULL_REQUEST)" = "false" ]; then \
	      gof3r put --no-md5 --acl=$(ACL) -b ${BUCKET} -k rsbin/$(NAME)/$(TRAVIS_BRANCH)/$$f <$$f; \
	      re='^(v[0-9]+)\.[0-9]+\.[0-9]+$$' ;\
	      if [[ "$(TRAVIS_BRANCH)" =~ $$re ]]; then \
	        gof3r put --no-md5 --acl=$(ACL) -b ${BUCKET} -k rsbin/$(NAME)/$${BASH_REMATCH[1]}/$$f <$$f; \
	      fi; \
	    fi; \
	  done)

# produce a version string that is embedded into the binary that captures the branch, the date
# and the commit we're building
version:
	@echo -e "package main\n\nconst VV = \"$(NAME) $(TRAVIS_BRANCH) - $(DATE) - $(TRAVIS_COMMIT)\"" \
	  >version.go
	@echo "version.go: `tail -1 version.go`"

# Handled natively in GO now for 1.5! Use glide to manage!
depend:
#	github.com/Masterminds/glide
	glide up

clean:
	rm -rf build
	rm -f version.go httpclient/user_agent.go

# gofmt uses the awkward *.go */*.go because gofmt -l . descends into the Godeps workspace
# and then pointlessly complains about bad formatting in imported packages, sigh
#	check-govers
lint:
	@if gofmt -l *.go 2>&1 | grep .go; then \
	  echo "^- Repo contains improperly formatted go files; run gofmt -w *.go" && exit 1; \
	  else echo "All .go files formatted correctly"; fi
	go tool vet -composites=false *.go

travis-test: lint
	ginkgo -r -cover

# running ginkgo twice, sadly, the problem is that -cover modifies the source code with the effect
# that if there are errors the output of gingko refers to incorrect line numbers
# tip: if you don't like colors use gingkgo -r -noColor
test: lint
	@test "$$PWD" != `/bin/pwd` && echo "*** Please cd `/bin/pwd` if compilation fails"
	ginkgo -r
	ginkgo -r -cover
	go tool cover -func=`basename $$PWD`.coverprofile

#===== SPECIAL TARGETS FOR right_st =====

.PHONY: right_st test
