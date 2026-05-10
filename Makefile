SHELL := /bin/sh

GO ?= go
PYTHON ?= $(if $(wildcard $(CURDIR)/.venv/bin/python3),$(CURDIR)/.venv/bin/python3,python3)
PORT ?= 4173
GOCACHE ?= $(CURDIR)/.gocache

.PHONY: test test-python sample refresh enrich-yfinance update-site-data update-live-data update-live-data-yfinance data-status clean-rate-limited-enrichment-cache preview smoke

test:
	GOCACHE="$(GOCACHE)" $(GO) test ./...
	$(MAKE) test-python

test-python:
	$(PYTHON) scripts/test_enrich_yfinance.py

sample:
	GOCACHE="$(GOCACHE)" $(GO) run ./cmd/statos-build sample

refresh:
	GOCACHE="$(GOCACHE)" $(GO) run ./cmd/statos-build refresh

enrich-yfinance:
	$(PYTHON) scripts/enrich_yfinance.py

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

update-live-data-yfinance:
	STATOS_ENRICHMENT_PROVIDER=cache $(MAKE) refresh
	$(PYTHON) scripts/data-status.py --require-live
	$(MAKE) enrich-yfinance
	STATOS_ENRICHMENT_PROVIDER=cache $(MAKE) refresh
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
