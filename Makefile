SHELL := /bin/sh

GO ?= go
PYTHON ?= python3
PORT ?= 4173
GOCACHE ?= $(CURDIR)/.gocache

.PHONY: test sample refresh update-site-data update-live-data data-status clean-rate-limited-enrichment-cache preview smoke

test:
	GOCACHE="$(GOCACHE)" $(GO) test ./...

sample:
	GOCACHE="$(GOCACHE)" $(GO) run ./cmd/statos-build sample

refresh:
	GOCACHE="$(GOCACHE)" $(GO) run ./cmd/statos-build refresh

update-site-data:
	$(MAKE) refresh
	$(MAKE) test
	$(MAKE) smoke
	$(MAKE) data-status

update-live-data:
	$(MAKE) refresh
	$(PYTHON) scripts/data-status.py --require-live
	$(MAKE) test
	$(MAKE) smoke

data-status:
	$(PYTHON) scripts/data-status.py

clean-rate-limited-enrichment-cache:
	$(PYTHON) scripts/clean-enrichment-cache.py --rate-limited --apply

preview:
	$(PYTHON) -m http.server $(PORT) --directory site

smoke:
	./scripts/smoke.sh
