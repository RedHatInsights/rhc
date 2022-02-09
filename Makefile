SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
.POSIX:
.SUFFIXES:

# Project variables

# Used as a prefix to binary names. Cannot contain spaces.
SHORTNAME := rhc
# Used as file and directory names. Cannot contain spaces.
LONGNAME  := rhc
# Used as a long-form description. Can contain spaces and punctuation.
BRANDNAME   := rhc
# Used as the tarball file name. Cannot contain spaces.
PKGNAME   := rhc
VERSION   := 0.2.0
# Used as the prefix for MQTT topic names
TOPICPREFIX := redhat/insights
# Used to force sending all HTTP traffic to a specific host.
DATAHOST := cert.cloud.redhat.com
# Used to identify the agency providing the connection broker.
PROVIDER := Red Hat

# Installation directories
PREFIX        ?= /usr/local
BINDIR        ?= $(PREFIX)/bin
SBINDIR       ?= $(PREFIX)/sbin
LIBEXECDIR    ?= $(PREFIX)/libexec
SYSCONFDIR    ?= $(PREFIX)/etc
DATADIR       ?= $(PREFIX)/share
DATAROOTDIR   ?= $(PREFIX)/share
MANDIR        ?= $(DATADIR)/man
DOCDIR        ?= $(DATADIR)/doc
LOCALSTATEDIR ?= $(PREFIX)/var
DESTDIR       ?=

# Dependent package directories
SYSTEMD_SYSTEM_UNIT_DIR  := $(shell pkg-config --variable systemdsystemunitdir systemd)

# Build flags
LDFLAGS := 
LDFLAGS += -X 'main.Version=$(VERSION)'
LDFLAGS += -X 'main.ShortName=$(SHORTNAME)'
LDFLAGS += -X 'main.LongName=$(LONGNAME)'
LDFLAGS += -X 'main.BrandName=$(BRANDNAME)'
LDFLAGS += -X 'main.PrefixDir=$(PREFIX)'
LDFLAGS += -X 'main.BinDir=$(BINDIR)'
LDFLAGS += -X 'main.SbinDir=$(SBINDIR)'
LDFLAGS += -X 'main.LibexecDir=$(LIBEXECDIR)'
LDFLAGS += -X 'main.SysconfDir=$(SYSCONFDIR)'
LDFLAGS += -X 'main.DataDir=$(DATADIR)'
LDFLAGS += -X 'main.DatarootDir=$(DATAROOTDIR)'
LDFLAGS += -X 'main.ManDir=$(MANDIR)'
LDFLAGS += -X 'main.DocDir=$(DOCDIR)'
LDFLAGS += -X 'main.LocalstateDir=$(LOCALSTATEDIR)'
LDFLAGS += -X 'main.TopicPrefix=$(TOPICPREFIX)'
LDFLAGS += -X 'main.Provider=$(PROVIDER)'

BUILDFLAGS ?=
BUILDFLAGS += -buildmode=pie
ifeq ($(shell find . -name vendor), ./vendor)
BUILDFLAGS += -mod=vendor
endif

BIN  = rhc
DATA = rhc.bash \
	   rhc.1.gz \
	   USAGE.md

GOSRC := $(shell find . -name '*.go')
GOSRC += go.mod go.sum

.PHONY: all
all: $(BIN) $(DATA)

.PHONY: bin
bin: $(BIN)

$(BIN): $(GOSRC)
	go build $(BUILDFLAGS) -ldflags "$(LDFLAGS)" -o $@ .

.PHONY: data
data: $(DATA)

%.bash: $(GOSRC)
	go run $(BUILDFLAGS) -ldflags "$(LDFLAGS)" . --generate-bash-completion > $@

%.1: $(GOSRC)
	go run $(BUILDFLAGS) -ldflags "$(LDFLAGS)" . --generate-man-page > $@

%.1.gz: %.1
	gzip -k $^

USAGE.md: $(GOSRC)
	go run $(BUILDFLAGS) -ldflags "$(LDFLAGS)" . --generate-markdown > $@

%: %.in Makefile
	sed \
	    -e 's,[@]SHORTNAME[@],$(SHORTNAME),g' \
		-e 's,[@]LONGNAME[@],$(LONGNAME),g' \
		-e 's,[@]BRANDNAME[@],$(BRANDNAME),g' \
		-e 's,[@]VERSION[@],$(VERSION),g' \
		-e 's,[@]PACKAGE[@],$(PACKAGE),g' \
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
		$< > $@.tmp && mv $@.tmp $@

.PHONY: install
install: $(BIN) $(DATA)
	pkg-config --modversion dbus-1 || exit 1
	pkg-config --modversion systemd || exit 1
	install -D -m755 ./rhc      $(DESTDIR)$(BINDIR)/$(SHORTNAME)
	install -D -m644 ./rhc.1.gz $(DESTDIR)$(MANDIR)/man1/$(SHORTNAME).1.gz
	install -D -m644 ./rhc.bash $(DESTDIR)$(DATADIR)/bash-completion/completions/$(SHORTNAME)

.PHONY: uninstall
uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(SHORTNAME)
	rm -f $(DESTDIR)$(MANDIR)/man1/$(SHORTNAME).1.gz
	rm -f $(DESTDIR)$(DATADIR)/bash-completion/completions/$(SHORTNAME)

.PHONY: dist
dist:
	go mod vendor
	tar --create \
		--gzip \
		--file /tmp/$(PKGNAME)-$(VERSION).tar.gz \
		--exclude=.git \
		--exclude=.vscode \
		--exclude=.github \
		--exclude=.gitignore \
		--exclude=.copr \
		--transform s/^\./$(PKGNAME)-$(VERSION)/ \
		. && mv /tmp/$(PKGNAME)-$(VERSION).tar.gz .
	rm -rf ./vendor

.PHONY: clean
clean:
	go mod tidy
	rm $(BIN)
	rm $(DATA)

.PHONY: tests
tests:
	go test -v ./...

.PHONY: vet
vet:
	go vet -v ./...
