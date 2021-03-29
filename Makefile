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
LDFLAGS ?= -X main.version=$(VERSION)
BUILDFLAGS ?= -a -ldflags '$(LDFLAGS)'
APPSOURCES := $(wildcard *.go) go.mod
PROJECT_NAME := $(shell basename $(PWD))
DATA_PATH ?= /srv/data/feeds

DESTDIR ?= /
INSTALL_PREFIX ?= usr/local

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

.PHONY: all content dispatch feeds mobi web clean mod_tidy

all: content dispatch feeds mobi web

content: mod_tidy bin/content
bin/content: cli/content/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cli/content/main.go

mobi: mod_tidy bin/mobi
bin/mobi: cli/mobi/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cli/mobi/main.go

feeds: mod_tidy bin/feeds
bin/feeds: cli/feeds/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cli/feeds/main.go

dispatch: mod_tidy bin/dispatch
bin/dispatch: cli/dispatch/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cli/dispatch/main.go

web: mod_tidy bin/web
bin/web: cli/web/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cli/web/main.go

clean:
	-$(RM) bin/*
	-$(RM) systemd/*.service

units: $(patsubst %.service.in, %.service, $(wildcard systemd/*.service.in))

systemd/%.service: systemd/%.service.in
	$(M4) $(M4_FLAGS) -DBIN_NAME=`basename $< | cut -d'.' -f1` -DDATA_PATH=$(DATA_PATH) $< >$@

mod_tidy:
	$(GO) mod tidy

install: units
	install bin/content $(DESTDIR)$(INSTALL_PREFIX)/bin/content
	install bin/mobi $(DESTDIR)$(INSTALL_PREFIX)/bin/mobi
	install bin/feeds $(DESTDIR)$(INSTALL_PREFIX)/bin/feeds
	install bin/dispatch $(DESTDIR)$(INSTALL_PREFIX)/bin/dispatch
	install -m 644 *.service $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/
	install -m 644 *.timer $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/

uninstall:
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/bin/content
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/bin/mobi
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/bin/feeds
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/bin/dispatch
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/content.service
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/mobi.service
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/feeds.service
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/feeds.service
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/dispatch.service
