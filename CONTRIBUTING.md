# Contributing

Statos is a static site plus a local Go builder. Production is GitHub Pages serving committed files from `site/`; there is no production server and deployment must not fetch broker or enrichment data.

## Local Setup

Use Go 1.23 or newer, matching `go.mod`.

```sh
cp .env.example .env
make sample
make test
make smoke
```

The Make targets set `GOCACHE` to `.gocache` inside the repo so Go commands work in restricted local environments. `.gocache/` is ignored and must stay local.

## Data Safety

Commit:

- Source, docs, and manual taxonomy under `data/manual`.
- Generated static outputs under `site/data`.

Keep local:

- `.env` and any credentials.
- Raw Trading 212 snapshots under `data/raw`.
- Enrichment caches under `data/cache`.
- Go build cache under `.gocache`.

Trading 212 instrument metadata is the source universe. Yahoo-style enrichment is secondary and replaceable. Use ISIN as the stable security key when present; broker tickers are listing or broker identifiers.

## Generated Files

Run `make sample` for deterministic sample output while developing UI or export logic. Run `make refresh` only when you intentionally want to use local Trading 212 credentials or fallback sample data.

After changing source or manual taxonomy, regenerate `site/data`, run `make smoke`, and review `site/data/build_manifest.json` plus `site/data/unclassified.csv`.

## Pre-Commit Checklist

- `git status --short` shows only intentional source, docs, manual taxonomy, and generated `site/data` changes.
- `git ls-files .env data/raw data/cache .gocache` prints nothing.
- `site/data` has been regenerated when source or manual taxonomy changed.
- `site/data/unclassified.csv` has been reviewed, with intentional gaps left visible.
- `site/data/build_manifest.json` has been checked for enrichment failures, counts, and freshness.
- `make test` passes.
- `make smoke` passes.
