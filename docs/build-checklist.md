# Build Checklist

Use this checklist for the weekly manual refresh.

## Before Refresh

- Confirm `.env` contains the intended Trading 212 environment.
- Confirm no secrets, private raw snapshots, enrichment caches, or `.gocache` files are staged.
- Review recent manual taxonomy changes under `data/manual`.

## Refresh

```sh
make refresh
make test
make smoke
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
- `site/data/build_manifest.json`

## Review

- Open `site/data/build_manifest.json` and check source counts, enrichment failures, unclassified counts, and freshness.
- Review `site/data/unclassified.csv`.
- Add or update exposures in `data/manual/exposures.csv`.
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

Commit source, manual data, and generated `site/data`. Do not commit `.env`, `data/raw`, `data/cache`, `.gocache`, logs, or raw provider artifacts.
