.ONESHELL:
.SHELLFLAGS := -e -c

VERSION := $(shell rpmspec rhc.spec --query --queryformat '%{version}\n' | head -n 1)

# The 'build' target is not used during packaging; it is present for upstream development purposes.
.PHONY: build
build:
	go build -ldflags "-X main.Version=$(VERSION)" -o rhc ./cmd/rhc

# Compile translation files (.po -> .mo)
.PHONY: i18n
i18n:
	@for file in po/*/LC_MESSAGES/*.po; do \
		msgfmt --check -o $${file%.po}.mo $$file; \
	done

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
	find po -name "*.mo" -type f -delete
