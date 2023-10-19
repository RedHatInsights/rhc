include Makefile

outdir ?= $(PWD)/outdir
spec ?= $(PWD)

SOURCEDIR := $(shell rpmbuild -E %_sourcedir)
SRCRPMDIR := $(shell rpmbuild -E %_srcrpmdir)

srpm: SHORTNAME = $(shell basename $(PWD))
srpm: LONGNAME = $(SHORTNAME)
srpm: SERVICENAME = "yggdrasil"
srpm: VERSION := $(shell echo $(VERSION) | awk -F. '{printf("%d.%d.%d", $$1, $$2, $$3+1)}')
srpm: RELEASE = $(shell printf "0.%s.git.%s" $(shell git rev-list $(shell git describe --tags --abbrev=0 --always | tr -d '\n')..HEAD --count | tr -d '\n') $(shell git rev-parse --short HEAD | tr -d '\n'))
srpm: PKGNAME = $(SHORTNAME)

.PHONY: srpm
srpm: deps
	mkdir -p $(SOURCEDIR)
	mkdir -p $(SRCRPMDIR)
	mkdir -p $(outdir)
	sed \
	    -e 's,[@]SHORTNAME[@],$(SHORTNAME),g' \
		-e 's,[@]LONGNAME[@],$(LONGNAME),g' \
		-e 's,[@]VERSION[@],$(VERSION),g' \
		-e 's,[@]RELEASE[@],$(RELEASE),g' \
		-e 's,[@]PKGNAME[@],$(PKGNAME),g' \
		-e 's,[@]TOPICPREFIX[@],$(TOPICPREFIX),g' \
		-e 's,[@]DATAHOST[@],$(DATAHOST),g' \
		-e 's,[@]PROVIDER[@],$(PROVIDER),g' \
		-e 's,[@]PREFIX[@],$(PREFIX),g' \
		-e 's,[@]BINDIR[@],$(BINDIR),g' \
		-e 's,[@]SBINDIR[@],$(SBINDIR),g' \
		-e 's,[@]LIBEXECDIR[@],$(LIBEXECDIR),g' \
		-e 's,[@]DATAROOTDIR[@],$(DATAROOTDIR),g' \
		-e 's,[@]DATADIR[@],$(DATADIR),g' \
		-e 's,[@]SYSCONFDIR[@],$(SYSCONFDIR),g' \
		-e 's,[@]LOCALSTATEDIR[@],$(LOCALSTATEDIR),g' \
		-e 's,[@]DOCDIR[@],$(DOCDIR),g' \
		-e 's,[@]SERVICENAME[@],$(SERVICENAME),g' \
		$(spec)/.copr/rhc.spec.in > $(spec)/rhc.spec
	make PKGNAME=$(PKGNAME) VERSION=$(VERSION)-$(RELEASE) dist
	install -D -m644 $(PKGNAME)-$(VERSION)-$(RELEASE).tar.gz $(SOURCEDIR)
	spectool --get-files --source 1 --sourcedir $(spec)/rhc.spec
	spectool --get-files --source 2 --sourcedir $(spec)/rhc.spec
	rpmbuild -bs $(spec)/rhc.spec
	install -D -m644 $(SRCRPMDIR)/*.rpm $(outdir)/

deps:
	dnf install -y golang git rpmdevtools
