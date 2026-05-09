# Build Checklist

Use this checklist for the weekly manual refresh.

## Before Refresh

- Confirm `.env` contains the intended Trading 212 environment.
- Confirm no secrets or private raw snapshots are staged.
- Review recent manual taxonomy changes under `data/manual`.

## Refresh

```sh
go run ./cmd/statos-build refresh
go test ./...
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
- Re-run the builder after manual taxonomy edits.

## Preview

```sh
cd site
python3 -m http.server 4173
```

Open `http://localhost:4173` and check:

- Global search returns tickers, companies, ISINs, sectors, industries, themes, and notes.
- Supply-chain map rows contain expected cards.
- Ticker modal has identity, classification, sources, related tickers, and local note controls.
- Watchlist, tags, colour labels, import, and export work from browser local storage.
- Unclassified queue is visible and actionable.

## Commit

Commit source, manual data, and generated `site/data`. Do not commit `.env`, `data/raw`, or `data/cache`.
