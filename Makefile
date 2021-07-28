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
VERSION   := 0.2.98
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
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.Version=$(VERSION)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.ShortName=$(SHORTNAME)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.LongName=$(LONGNAME)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.BrandName=$(BRANDNAME)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.PrefixDir=$(PREFIX)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.BinDir=$(BINDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.SbinDir=$(SBINDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.LibexecDir=$(LIBEXECDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.SysconfDir=$(SYSCONFDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.DataDir=$(DATADIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.DatarootDir=$(DATAROOTDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.ManDir=$(MANDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.DocDir=$(DOCDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.LocalstateDir=$(LOCALSTATEDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.TopicPrefix=$(TOPICPREFIX)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.DataHost=$(DATAHOST)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.Provider=$(PROVIDER)'

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
