SHELL := /bin/sh

GO ?= go
PYTHON ?= python3
PORT ?= 4173
GOCACHE ?= $(CURDIR)/.gocache

.PHONY: test sample refresh preview smoke

test:
	GOCACHE="$(GOCACHE)" $(GO) test ./...

sample:
	GOCACHE="$(GOCACHE)" $(GO) run ./cmd/statos-build sample

refresh:
	GOCACHE="$(GOCACHE)" $(GO) run ./cmd/statos-build refresh

preview:
	$(PYTHON) -m http.server $(PORT) --directory site

smoke:
	./scripts/smoke.sh
