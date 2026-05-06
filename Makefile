.ONESHELL:
.SHELLFLAGS := -e -c

VERSION := $(shell rpmspec rhc.spec --query --queryformat '%{version}')
LDFLAGS := -ldflags "-X github.com/redhatinsights/rhc/pkg/version.Version=$(VERSION)"
GO_BUILD := go build $(LDFLAGS)

# The 'build' target is not used during packaging; it is present for upstream development purposes.
.PHONY: build
build:
	$(GO_BUILD) -o rhc ./cmd/rhc
	$(GO_BUILD) -o rhc-server ./cmd/rhc-server
	$(GO_BUILD) -o rhc-collector ./cmd/rhc-collector

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
	rm -f rhc-server
	rm -f rhc-collector
	rm -f rhc-*.tar*
