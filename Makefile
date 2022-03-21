SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

M4 = /usr/bin/m4
M4_FLAGS = -P

export GOOS=linux
export GOARCH=amd64
export VERSION=(unknown)
GO := go
ENV ?= dev
LDFLAGS ?= -X main.version=$(VERSION) -X github.com/mariusor/feeds.PocketConsumerKey=$(POCKET_CONSUMER_KEY)
BUILDFLAGS ?= -a -ldflags '$(LDFLAGS)'
APPSOURCES := $(wildcard *.go) go.mod
PROJECT_NAME := $(shell basename $(PWD))
DATA_PATH ?= /srv/data/feeds

DESTDIR ?= /
INSTALL_PREFIX ?= usr/local
UNITDIR = lib/systemd/system

ifneq ($(ENV), dev)
	LDFLAGS += -s -w -extldflags "-static"
endif


ifeq ($(shell git describe --always > /dev/null 2>&1 ; echo $$?), 0)
export VERSION = $(shell git describe --always --dirty="-git")
endif
ifeq ($(shell git describe --tags > /dev/null 2>&1 ; echo $$?), 0)
export VERSION = $(shell git describe --tags)
endif

BUILD := $(GO) build $(BUILDFLAGS)
TEST := $(GO) test $(BUILDFLAGS)

.PHONY: all content dispatch feeds ebook web clean download

all: content dispatch feeds ebook web

content: download bin/content
bin/content: cmd/content/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cmd/content/main.go

ebook: download bin/ebook
bin/ebook: cmd/ebook/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cmd/ebook/main.go

feeds: download bin/feeds
bin/feeds: cmd/feeds/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cmd/feeds/main.go

dispatch: download bin/dispatch
bin/dispatch: cmd/dispatch/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cmd/dispatch/main.go

web: download bin/web
bin/web: cmd/web/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cmd/web/main.go

clean:
	-$(RM) bin/*
	-$(RM) systemd/*.service

units: $(patsubst %.service.in, %.service, $(wildcard systemd/*.service.in))

systemd/%.service: systemd/%.service.in
	$(M4) $(M4_FLAGS) -DBIN_NAME=`basename $< | cut -d'.' -f1` -DDATA_PATH=$(DATA_PATH) $< >$@

download:
	$(GO) mod download all

install: units
	test -d $(DESTDIR)$(INSTALL_PREFIX)/$(UNITDIR)/ || mkdir -p $(DESTDIR)$(INSTALL_PREFIX)/$(UNITDIR)/
	install bin/content $(DESTDIR)$(INSTALL_PREFIX)/bin/content
	install bin/ebook $(DESTDIR)$(INSTALL_PREFIX)/bin/ebook
	install bin/feeds $(DESTDIR)$(INSTALL_PREFIX)/bin/feeds
	install bin/dispatch $(DESTDIR)$(INSTALL_PREFIX)/bin/dispatch
	install -m 644 systemd/*.service $(DESTDIR)$(INSTALL_PREFIX)/$(UNITDIR)/
	install -m 644 systemd/*.timer $(DESTDIR)$(INSTALL_PREFIX)/$(UNITDIR)/

uninstall:
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/bin/content
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/bin/dispatch
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/bin/ebook
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/bin/feeds
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/$(UNITDIR)/content.service
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/$(UNITDIR)/dispatch.service
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/$(UNITDIR)/ebook.service
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/$(UNITDIR)/feeds.service
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/$(UNITDIR)/content.timer
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/$(UNITDIR)/dispatch.timer
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/$(UNITDIR)/ebook.timer
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/$(UNITDIR)/feeds.timer
