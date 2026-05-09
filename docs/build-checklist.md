# Build Checklist

Use this checklist for the weekly manual refresh.

## Before Refresh

- Confirm `.env` contains the intended Trading 212 environment.
- Confirm no secrets, private raw snapshots, enrichment caches, or `.gocache` files are staged.
- Review recent manual taxonomy changes under `data/manual`.
- Keep [Manual Taxonomy Workflow](taxonomy-workflow.md) open for file purposes and validation rules.

## Refresh

```sh
make refresh
make test
make smoke
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

- `site/data/catalogue.json`
- `site/data/tickers.csv`
- `site/data/companies.json`
- `site/data/sectors.json`
- `site/data/industries.json`
- `site/data/themes.json`
- `site/data/supply_chains.json`
- `site/data/search_index.json`
- `site/data/unclassified.csv`
- `site/data/identity_issues.csv`
- `site/data/enrichment_failures.csv`
- `site/data/securities.csv`
- `site/data/listings.csv`
- `site/data/build_manifest.json`

## Review

- Open `site/data/build_manifest.json` and check source mode, Trading 212 environment/base URL, raw snapshot paths, endpoint diagnostics, rate-limit observations, source counts, enrichment provider/cache hit/miss/stale/failure counts, unclassified counts, identity duplicate/collision counts, category/flag counts, and freshness.
- Review `site/data/enrichment_failures.csv` for cache misses, cached provider failures, ambiguous matches, and unknown cache schema rows.
- Review `site/data/unclassified.csv`.
- Print taxonomy coverage without network calls:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy coverage
```

- Generate an exposure template when unclassified rows need theme/layer review:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy exposure-template --out /tmp/statos-exposure-template.csv
```

- Review `site/data/identity_issues.csv`, `site/data/securities.csv`, and `site/data/listings.csv` for low-confidence identity mappings, duplicate tickers, shared ISIN collisions, and manual override misses.
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

- Global search returns tickers, companies, ISINs, sectors, industries, themes, and notes.
- Supply-chain map rows contain expected cards.
- Ticker modal has identity, classification, sources, related tickers, and local note controls.
- Watchlist, tags, colour labels, import, and export work from browser local storage.
- Unclassified queue is visible and actionable.

## Commit

Commit source, manual data, and generated `site/data`. Do not commit `.env`, `data/raw`, `data/cache`, `.gocache`, logs, or raw provider artifacts. `git ls-files .env data/raw data/cache .gocache` should print nothing.
