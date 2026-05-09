# Statos

Statos is a static investment research website for mapping Trading 212 Invest / Stocks ISA-compatible tickers into companies, securities, listings, sectors, industries, themes, supply-chain layers, peer groups, watchlists, and notes.

The production site is static and suitable for GitHub Pages. A local Go CLI refreshes broker metadata, applies enrichment and manual taxonomy, then writes committed files into `site/data`.

## Current Scope

- Trading 212 metadata client for instruments and exchanges.
- Timestamped Trading 212 raw snapshots with latest aliases, HTTP diagnostics, rate-limit observations, and raw replay mode.
- Local builder command at `cmd/statos-build`.
- Standard-library enrichment interface with versioned cache-first Yahoo-compatible provider, stale-cache diagnostics, ambiguous-match handling, and failure exports.
- Normalized catalogue model covering instruments, securities, companies, listings, classifications, themes, supply-chain layers, exposures, sources, and unclassified rows.
- Manual taxonomy files under `data/manual`.
- Static HTML/CSS/JS research UI with search, tables, supply-chain map, modal detail, watchlist, local notes/tags/colour labels, import/export, and unclassified review.

## Requirements

- Go 1.23 or newer, matching `go.mod`.
- Python 3 for the local static preview and smoke-check JSON parsing.
- `make` for the documented local shortcuts.

## Setup

```sh
cp .env.example .env
```

Fill in Trading 212 credentials if you want to use live account metadata. With no credentials, the builder falls back to the sample dataset so the site remains usable during development.

```sh
make sample
make test
make smoke
```

Preview the site:

```sh
make preview
```

Open `http://localhost:4173`.

The Make targets run Go with `GOCACHE="$PWD/.gocache"` so local builds work when the default Go build cache is not writable. `.gocache/` is ignored and should not be committed. If running Go manually in a restricted environment, use the same workaround:

```sh
GOCACHE="$PWD/.gocache" go test ./...
```

## Builder Commands

```sh
make sample
make refresh
make test
make smoke
make preview
```

`make refresh` fetches Trading 212 metadata when credentials are present. With no credentials it falls back to the embedded sample dataset. `make sample` always uses the embedded sample universe and is useful for UI development. `make smoke` verifies the expected generated `site/data` files exist, including identity and enrichment review CSVs, and checks that `catalogue.json` and `build_manifest.json` parse as JSON.

The underlying builder remains available directly:

```sh
go run ./cmd/statos-build refresh
go run ./cmd/statos-build refresh --sample
go run ./cmd/statos-build refresh --no-fetch
go run ./cmd/statos-build refresh --no-fetch --input-raw-dir data/raw/trading212
go run ./cmd/statos-build sample
go run ./cmd/statos-build taxonomy coverage
go run ./cmd/statos-build taxonomy exposure-template
```

`refresh --no-fetch` rebuilds from `instruments_latest.json` and `exchanges_latest.json` in `data/raw/trading212` by default. Use `--input-raw-dir` to replay an alternate raw snapshot directory. Replay does not call Trading 212 and uses cache-only enrichment.

The `taxonomy coverage` and `taxonomy exposure-template` commands read generated `site/data` files only. They do not fetch Trading 212 or enrichment data.

Live Trading 212 fetches read credentials only from `.env` or the process environment. Set `STATOS_TRADING212_ENV=demo` or `live`, or set `STATOS_TRADING212_BASE_URL` explicitly. Successful fetches write timestamped ignored raw files plus `*_latest.json` aliases under `data/raw/trading212`.

## GitHub Pages Deployment

Statos keeps the static publish root in `site/`. GitHub Pages branch publishing only serves repository root `/` or `/docs`, so this repo uses GitHub Actions Pages deployment instead of moving or duplicating the site.

The workflow at `.github/workflows/pages.yml` uploads the committed `site/` directory as the Pages artifact. It runs `make smoke` first, but it does not run `make refresh`, fetch Trading 212 data, call enrichment providers, or require Trading 212/Yahoo secrets. The `site/.nojekyll` file is included in the published root.

Repository setup:

1. In GitHub, open Settings -> Pages.
2. Set Source to `GitHub Actions`.
3. Push to `main` or run the `Deploy Pages` workflow manually.

Before publishing, regenerate data locally with `make sample` or `make refresh`, review `site/data/unclassified.csv` and `site/data/build_manifest.json`, run `make test` and `make smoke`, then commit source, manual taxonomy, and generated `site/data`.

## Project Checklists

- [Product readiness checklist](docs/readiness-checklist.md)
- [Weekly build checklist](docs/build-checklist.md)
- [Manual taxonomy workflow](docs/taxonomy-workflow.md)

## Data Flow

1. Fetch Trading 212 metadata from `GET /api/v0/equity/metadata/exchanges` and `GET /api/v0/equity/metadata/instruments`, or replay ignored raw snapshots with `--no-fetch`.
2. Write raw snapshots into ignored `data/raw/trading212` during sample or live fetch runs.
3. Normalize broker instruments into tickers, listings, securities, and companies.
4. Resolve enrichment through the versioned cache and optional Yahoo-compatible lookup.
5. Apply manual overrides, themes, supply chains, exposures, and notes.
6. Export JSON/CSV to committed `site/data`, including normalized enrichment diagnostics.

## Manual Taxonomy

Edit these committed files:

- `data/manual/themes.yml`
- `data/manual/supply_chains.yml`
- `data/manual/company_overrides.csv`
- `data/manual/ticker_overrides.csv`
- `data/manual/classification_overrides.csv`
- `data/manual/identity_overrides.csv`
- `data/manual/exposures.csv`
- `data/manual/relationships.csv`
- `data/manual/notes/*.md`

Exposure rows include theme, layer, target, score, confidence, source URL, rationale, and last reviewed date. Exposure scores must parse as numbers from `0` to `5`, confidence must use an allowed manual/rule confidence value, source URLs must be absolute HTTP(S), and reviewed dates use `YYYY-MM-DD`.

Use `classification_overrides.csv` for manual sector, industry, and country data separate from provider enrichment. It wins over legacy ticker/company classification columns, which still remain backward compatible.

Use `relationships.csv` for reviewed peers, substitutes, upstream suppliers, downstream customers, and related plays. Relationship rows are loaded and validated now; full relationship graph exports are planned separately.

Identity override rows can target a ticker, ISIN, security, or company and can force `override_security_id`, `override_company_id`, normalized category, structure flags, and confidence. Use this file for missing or misleading ISINs, ADR/GDR mappings, dual listings, ETFs, funds, and trusts where rule-based identity is not enough.

Ticker override rows can also set enrichment overrides for `yahoo_symbol`, `sector`, `industry`, `country`, `market_cap`, `exchange`, and `currency`. `market_cap` must be an integer when present. These manual fields win over provider profile data.

Detailed review steps are in [Manual Taxonomy Workflow](docs/taxonomy-workflow.md).

## Enrichment

Yahoo Finance does not provide a stable official public API. Statos treats Yahoo-style data as replaceable enrichment, not the source of truth. Set `STATOS_ENRICHMENT_PROVIDER=yahoo` only when you want the builder to attempt live enrichment and cache the response or failure locally.

The default provider mode is cache-only. Cache misses, stale entries, cached failures, unknown cache schema versions, and ambiguous matches are surfaced in `site/data/build_manifest.json` and `site/data/enrichment_failures.csv`. Stale cache entries are still used by default so offline builds remain useful.

Provider interface and cache contract details are documented in [Enrichment Provider Contract](docs/enrichment-provider.md).

## Safety

`.env`, raw snapshots, enrichment caches, and `.gocache/` are ignored. Generated static outputs under `site/data` are intended to be committed.

`site/data/build_manifest.json` includes the source mode, Trading 212 environment/base URL, fetch timestamp, raw snapshot path summary, per-endpoint HTTP diagnostics, observed Trading 212 rate-limit headers, enrichment cache/provider diagnostics, and identity counts for missing tickers/ISINs, duplicates, collisions, categories, flags, and applied overrides. It never stores API keys, API secrets, authorization headers, or raw provider responses.

## License

PolyForm Noncommercial License 1.0.0. See `LICENSE`.
