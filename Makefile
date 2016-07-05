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
EXE:=$(NAME)$(shell go env GOEXE)
BUCKET=rightscale-binaries
ACL=public-read
# version for gopkg.in, e.g. v1, v2, ...
GOPKG_VERS=v1
# Dependencies handled by go+glide. Requires 1.5+
export GO15VENDOREXPERIMENT=1
GLIDE_VERSION?=v0.11.0
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
GLIDE_TGZ=glide-$(GLIDE_VERSION)-$(GOOS)-$(GOARCH).tar.gz
ifeq ($(GOOS),windows)
GLIDE_EXEC=glide-$(GLIDE_VERSION).exe
else
GLIDE_EXEC=glide-$(GLIDE_VERSION)
endif
# Dependencies that need to be installed
INSTALL_DEPEND=	github.com/onsi/ginkgo/ginkgo \
		github.com/rlmcpherson/s3gof3r/gof3r
# github.com/rogpeppe/gover requires auth?
#=== below this line ideally remains unchanged, add new targets at the end  ===

TRAVIS_BRANCH?=dev
DATE=$(shell date '+%F %T')
SECONDS=$(shell date '+%s')
TRAVIS_COMMIT?=$(shell git rev-parse HEAD)
GIT_BRANCH:=$(shell git symbolic-ref --short -q HEAD || echo "master")
SHELL:=/bin/bash

# if we are building a tag overwrite TRAVIS_BRANCH with TRAVIS_TAG
ifneq ($(TRAVIS_TAG),)
TRAVIS_BRANCH:=$(TRAVIS_TAG)
endif

ifeq ($(GOOS),windows)
export CC:=x86_64-w64-mingw32-gcc
export CXX:=x86_64-w64-mingw32-g++
endif

# determine archives to make for the build job which determines what will be uploaded by the Travis CI job
ifeq ($(GOOS),linux)
UPLOADS:=build/$(NAME)-linux-amd64.tgz build/$(NAME)-windows-amd64.zip
else
  ifeq ($(GOOS),darwin)
UPLOADS:=build/$(NAME)-darwin-amd64.tgz
  else
    ifeq ($(GOOS),windows)
UPLOADS:=build/$(NAME)-windows-amd64.zip
    else
UPLOADS:=build/$(NAME)-$(GOOS)-$(GOARCH).tgz
    endif
  endif
endif

# the default target builds a binary in the top-level dir for whatever the local OS is
default: $(EXE)
$(EXE): *.go version
	go build -tags right_st_make -o $(EXE) .

install: $(EXE)
	go install

# the standard build produces a "local" executable, a linux tgz, and a darwin (macos) tgz
build: $(EXE) $(UPLOADS)

# create a tgz with the binary and any artifacts that are necessary
# note the hack to allow for various GOOS & GOARCH combos
build/$(NAME)-%.tgz: *.go version
	rm -rf build/$(NAME)
	mkdir -p build/$(NAME)
	tgt=$*; GOOS=$${tgt%-*} GOARCH=$${tgt#*-} go build -tags right_st_make -o build/$(NAME)/$(NAME) .
	chmod +x build/$(NAME)/$(NAME)
	tar -zcf $@ -C build $(NAME)
	rm -r build/$(NAME)

# create a zip with the binary and any artifacts that are necessary
# note the hack to allow for various GOOS & GOARCH combos, sigh
build/$(NAME)-%.zip: *.go version
	rm -rf build/$(NAME)
	mkdir -p build/$(NAME)
	tgt=$*; GOOS=$${tgt%-*} GOARCH=$${tgt#*-} go build -tags right_st_make -o build/$(NAME)/$(NAME).exe .
	cd build; zip -r $(notdir $@) $(NAME)
	rm -r build/$(NAME)

# upload assumes you have AWS_ACCESS_KEY_ID and AWS_SECRET_KEY env variables set,
# which happens in the .travis.yml for CI
upload:
	@which gof3r >/dev/null || (echo 'Please "make depend"'; false)
	(cd build; set -ex; shopt -s nullglob; \
	  re='^(v[0-9]+)\.[0-9]+\.[0-9]+$$' ;\
	  for f in *.tgz *.zip; do \
	    gof3r put --no-md5 --acl=$(ACL) -b ${BUCKET} -k rsbin/$(NAME)/$(TRAVIS_COMMIT)/$$f <$$f; \
	    if [ "$(TRAVIS_PULL_REQUEST)" = "false" ]; then \
	      gof3r put --no-md5 --acl=$(ACL) -b ${BUCKET} -k rsbin/$(NAME)/$(TRAVIS_BRANCH)/$$f <$$f; \
	      if [[ "$(TRAVIS_TAG)" =~ $$re ]]; then \
	        gof3r put --no-md5 --acl=$(ACL) -b ${BUCKET} -k rsbin/$(NAME)/$${BASH_REMATCH[1]}/$$f <$$f; \
	      fi; \
	    fi; \
	  done; \
	  if [[ $(GOOS) == linux ]] && [[ "$(TRAVIS_TAG)" =~ $$re ]]; then \
	    ../version.sh > version.yml; \
	    gof3r put --no-md5 --acl=$(ACL) -b ${BUCKET} -k rsbin/$(NAME)/version.yml <version.yml; \
	  fi)

# produce a version string that is embedded into the binary that captures the branch, the date
# and the commit we're building
version:
	@echo -e "// +build right_st_make\n\npackage main\n\nconst VV = \"$(NAME) $(TRAVIS_BRANCH) - $(DATE) - $(TRAVIS_COMMIT)\"" \
	  >version.go
	@echo "version.go: `tail -1 version.go`"

bin/$(GLIDE_EXEC):
	mkdir -p bin tmp
	cd tmp && curl -sSfL --tlsv1 --connect-timeout 30 --max-time 180 --retry 3 \
	  -O https://github.com/Masterminds/glide/releases/download/$(GLIDE_VERSION)/$(GLIDE_TGZ)
	cd tmp && tar xzvf $(GLIDE_TGZ)
	mv tmp/$(GOOS)-$(GOARCH)/glide* bin/$(GLIDE_EXEC)
	rm -rf tmp

# Handled natively in GO now for 1.5! Use glide to manage!
depend: bin/$(GLIDE_EXEC)
	./bin/$(GLIDE_EXEC) --quiet install --cache
	for d in $(INSTALL_DEPEND); do (cd vendor/$$d && go install); done

clean:
	rm -rf build $(EXE)
	rm -f version.go

# gofmt uses the awkward *.go */*.go because gofmt -l . descends into the Godeps workspace
# and then pointlessly complains about bad formatting in imported packages, sigh
#	check-govers
lint:
	@if gofmt -l *.go 2>&1 | grep .go; then \
	  echo "^- Repo contains improperly formatted go files; run gofmt -w *.go" && exit 1; \
	  else echo "All .go files formatted correctly"; fi
	go tool vet -composites=false *.go

test: lint
	ginkgo -cover -race $(shell ./bin/$(GLIDE_EXEC) novendor)

#===== SPECIAL TARGETS FOR right_st =====

.PHONY: right_st test
