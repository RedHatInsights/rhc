.ONESHELL:
.SHELLFLAGS := -e -c

VERSION := $(shell rpmspec rhc.spec --query --srpm --queryformat '%{version}')
LDFLAGS := -ldflags "-X github.com/redhatinsights/rhc/pkg/version.Version=$(VERSION)"
GO_BUILD := go build $(LDFLAGS)

# The 'build' target is not used during downstream packaging; it is present for upstream development purposes.
.PHONY: build
build:
	$(GO_BUILD) -o rhc ./cmd/rhc
	$(GO_BUILD) -o rhc-server ./cmd/rhc-server
	$(GO_BUILD) -o rhc-collector ./cmd/rhc-collector
	$(GO_BUILD) -o com.redhat.minimal ./cmd/minimal-collector

.PHONY: archive
archive:
	git archive --prefix rhc-$(VERSION)/ --format tar.gz HEAD > rhc-$(VERSION).tar.gz

# Generate -vendor tarball to be used as .spec's Source1, containing dependencies.
# On Fedora, this could be done by go-vendor-tools package, but CentOS Stream and RHEL
# do not have it available yet.
.PHONY: archive-deps
archive-deps:
	go mod vendor
	tar --create --bzip2 \
		--file "$(CURDIR)/rhc-$(VERSION)-vendor.tar.bz2" \
		--sort name \
		--mtime="@$${SOURCE_DATE_EPOCH:-0}" \
		--owner 0 --group 0 --numeric-owner \
		go.mod go.sum vendor/

.PHONY: srpm
srpm: archive archive-deps
	rpmbuild --define "_sourcedir $$(pwd)" -bs rhc.spec

.PHONY: rpm
rpm: srpm
	rpmbuild --define "_sourcedir $$(pwd)" -bb rhc.spec

# The 'clean' target removes build artifacts.
.PHONY: clean
clean:
	rm -f rhc
	rm -f rhc-server
	rm -f rhc-collector
	rm -f com.redhat.minimal
	rm -f rhc-*.tar*
	rm -rf vendor/
	rm -rf x86_64/
