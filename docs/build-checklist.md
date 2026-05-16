# Build Checklist

Use this checklist for the weekly manual refresh.

## Before Refresh

- Confirm `.env` contains the intended Trading 212 environment.
- Confirm no secrets, private raw snapshots, enrichment caches, or `.gocache` files are staged.
- Review recent manual taxonomy changes under `data/manual`.
- Keep [Manual Taxonomy Workflow](taxonomy-workflow.md) open for file purposes and validation rules.

## Refresh

```sh
make update-live-data
```

Use `make update-site-data` only when sample fallback is acceptable. For publishable account data, `make update-live-data` should report `sourceMode=live_fetch` and fail if the builder fell back to sample data.

To enrich using the optional yfinance cache warmer instead of the direct Go Yahoo-compatible provider:

```sh
python3 -m venv .venv
.venv/bin/python3 -m pip install -r requirements-enrichment.txt
make update-live-data-yfinance
```

For a small probe first:

```sh
python3 scripts/enrich_yfinance.py --limit 100
STATOS_ENRICHMENT_PROVIDER=cache make refresh
make smoke
```

If Yahoo enrichment reports widespread `429 Too Many Requests` rows, wait for the provider limit to reset, then run:

```sh
make clean-rate-limited-enrichment-cache
STATOS_ENRICHMENT_DELAY=2s make update-live-data
```

You can also put the delay in `.env`:

```sh
STATOS_ENRICHMENT_DELAY=2s
make update-live-data
```

To rebuild from the latest ignored Trading 212 raw snapshots without calling Trading 212:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh --no-fetch
make smoke
```

To replay an alternate raw snapshot directory:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh --no-fetch --input-raw-dir data/raw/trading212
```

The builder should write:

- `site/data/app_bootstrap.json`
- `site/data/tickers_index.json`
- `site/data/explorer_index.json`
- `site/data/catalogue.json`
- `site/data/tickers.csv`
- `site/data/companies.json`
- `site/data/sectors.json`
- `site/data/industries.json`
- `site/data/themes.json`
- `site/data/supply_chains.json`
- `site/data/search_index.json`
- `site/data/securities.json`
- `site/data/listings.json`
- `site/data/relationships.json`
- `site/data/unclassified.json`
- `site/data/review_queues.json`
- `site/data/review_summary.json`
- `site/data/unclassified.csv`
- `site/data/taxonomy_issues.csv`
- `site/data/enrichment_issues.csv`
- `site/data/identity_issues.csv`
- `site/data/enrichment_failures.csv`
- `site/data/stale_reviews.csv`
- `site/data/suggested_classification_overrides.csv`
- `site/data/suggested_exposures.csv`
- `site/data/suggested_ticker_overrides.csv`
- `site/data/suggested_identity_overrides.csv`
- `site/data/securities.csv`
- `site/data/listings.csv`
- `site/data/build_manifest.json`

## Review

- Open `site/data/build_manifest.json` and check data contract/schema versions, generated file checksum metadata for all generated files including `app_bootstrap.json`, `tickers_index.json`, and review queue files; source mode; Trading 212 environment/base URL; raw snapshot paths; endpoint diagnostics; rate-limit observations; source counts; enrichment provider/cache hit/miss/stale/failure counts; unclassified counts; identity duplicate/collision counts; review queue counts, reason counts, and deltas; category/flag counts; and freshness.
- Open `site/data/review_summary.json` to pick the highest-impact queue/reason bucket for the week.
- Review `site/data/review_queues.json` through the static Review queues UI, or use the focused CSVs below.
- Review `site/data/taxonomy_issues.csv` for missing sector, industry, company ID, ISIN, and theme exposure work.
- Review `site/data/enrichment_issues.csv` and `site/data/enrichment_failures.csv` for cache misses, cached provider failures, ambiguous matches, and unknown cache schema rows.
- Review `site/data/identity_issues.csv` for low-confidence identity mappings, duplicate tickers, shared ISIN collisions, and manual override misses.
- Review `site/data/stale_reviews.csv` for manual rows or notes that need a fresh source check.
- Print taxonomy coverage without network calls:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy coverage
```

- Use generated manual row suggestions to reduce copy/paste:

```sh
site/data/suggested_classification_overrides.csv
site/data/suggested_exposures.csv
site/data/suggested_ticker_overrides.csv
site/data/suggested_identity_overrides.csv
```

Fill the blank judgment fields before pasting suggested rows into `data/manual`.

- Generate an exposure template from legacy unclassified rows when needed:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy exposure-template --out /tmp/statos-exposure-template.csv
```

- Review `site/data/securities.csv`, `site/data/listings.csv`, `site/data/securities.json`, and `site/data/listings.json` when identity queue rows point to security/listing problems.
- Review `site/data/relationships.json` when manual relationship rows change.
- Add or update exposures in `data/manual/exposures.csv`.
- Add or update classification overrides in `data/manual/classification_overrides.csv` when provider sector, industry, or country values are missing or wrong.
- Add or update relationship rows in `data/manual/relationships.csv` for peers, substitutes, suppliers, customers, and related plays.
- Add notes in `data/manual/notes`.
- Re-run `make refresh` after manual taxonomy edits.
- Re-run `make smoke` before publishing or committing generated data.

## Preview

```sh
make preview
```

Open `http://localhost:4173` and check:

- Initial page load does not request `site/data/catalogue.json`; the default ticker view loads from `app_bootstrap.json` and `tickers_index.json`.
- Global search returns tickers, companies/security names, ISINs, sectors, industries, themes, and local notes/tags.
- Supply-chain map rows contain expected cards.
- Ticker modal has identity, classification, sources, related tickers, and local note controls.
- Watchlist, tags, colour labels, import, and export work from browser local storage.
- Review queues are visible, filterable by queue/reason/severity/gap, and suggested manual rows can be copied.

## Commit

Commit source, manual data, and generated `site/data`. Do not commit `.env`, `data/raw`, `data/cache`, `.gocache`, logs, or raw provider artifacts. `git ls-files .env data/raw data/cache .gocache` should print nothing.
