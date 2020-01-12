SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64
export VERSION=(unknown)
GO := go
ENV ?= dev
LDFLAGS ?= -X main.version=$(VERSION)
BUILDFLAGS ?= -a -ldflags '$(LDFLAGS)'
APPSOURCES := $(wildcard cmd/*.go)
PROJECT_NAME := $(shell basename $(PWD))

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

.PHONY: all content dispatch feeds mobi web clean

all: content dispatch feeds mobi web

content: bin/content
bin/content: go.mod cli/content/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cli/content/main.go

mobi: bin/mobi
bin/mobi: go.mod cli/mobi/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cli/mobi/main.go

feeds: bin/feeds
bin/feeds: go.mod cli/feeds/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cli/feeds/main.go

dispatch: bin/dispatch
bin/dispatch: go.mod cli/dispatch/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cli/dispatch/main.go

web: bin/web
bin/web: go.mod cli/web/main.go $(APPSOURCES)
	$(BUILD) -tags $(ENV) -o $@ ./cli/web/main.go

clean:
	-$(RM) bin/*

units: all


install: units
	install bin/content $(DESTDIR)$(INSTALL_PREFIX)/bin/fcontent
	install bin/mobi $(DESTDIR)$(INSTALL_PREFIX)/bin/fmobi
	install bin/feeds $(DESTDIR)$(INSTALL_PREFIX)/bin/ffeeds
	install bin/dispatch $(DESTDIR)$(INSTALL_PREFIX)/bin/fdispatch
	install -m 644 *.service $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/
	install -m 644 *.timer $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/

uninstall:
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/bin/fcontent
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/bin/fmobi
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/bin/ffeeds
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/bin/fdispatch
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/fcontent.service
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/fmobi.service
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/ffeeds.service
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/ffeeds.service
	$(RM) $(DESTDIR)$(INSTALL_PREFIX)/usr/lib/systemd/fdispatch.service
