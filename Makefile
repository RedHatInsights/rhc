.ONESHELL:
.SHELLFLAGS := -e -c

VERSION := $(shell git rev-parse --short=7 HEAD)

# The 'build' target is not used during packaging; it is present for upstream development purposes.
.PHONY: build
build:
	go build -ldflags "-X main.Version=$(VERSION)" -o rhc ./cmd/rhc

.PHONY: archive
archive:
	git archive --prefix rhc-$(VERSION)/ --format tar.gz HEAD > rhc-$(VERSION).tar.gz
	go_vendor_archive create --output rhc-$(VERSION)-vendor.tar.bz2 .

.PHONY: srpm
srpm: archive
	rpmbuild --define "_sourcedir $$(pwd)" -bs rhc.spec

# The 'clean' target removes build artifacts.
.PHONY: clean
clean:
	rm -f rhc
	rm -f rhc-*.tar*
