# Statos

Statos is a static investment research website for mapping Trading 212 Invest / Stocks ISA-compatible tickers into companies, securities, listings, sectors, industries, themes, supply-chain layers, peer groups, watchlists, and notes.

The production site is static and suitable for GitHub Pages. A local Go CLI refreshes broker metadata, applies enrichment and manual taxonomy, then writes committed files into `site/data`.

## Current Scope

- Trading 212 metadata client for instruments and exchanges.
- Local builder command at `cmd/statos-build`.
- Standard-library enrichment interface with cache-first Yahoo-compatible provider.
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

`make refresh` fetches Trading 212 metadata when credentials are present. `make sample` always uses the embedded sample universe and is useful for UI development. `make smoke` verifies the expected generated `site/data` files exist and checks that `catalogue.json` and `build_manifest.json` parse as JSON.

The underlying builder remains available directly:

```sh
go run ./cmd/statos-build refresh
go run ./cmd/statos-build refresh --sample
go run ./cmd/statos-build sample
```

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

## Data Flow

1. Fetch Trading 212 metadata from `GET /api/v0/equity/metadata/exchanges` and `GET /api/v0/equity/metadata/instruments`.
2. Write raw snapshots into ignored `data/raw/trading212`.
3. Normalize broker instruments into tickers, listings, securities, and companies.
4. Resolve enrichment through cache and optional Yahoo-compatible lookup.
5. Apply manual overrides, themes, supply chains, exposures, and notes.
6. Export JSON/CSV to committed `site/data`.

## Manual Taxonomy

Edit these committed files:

- `data/manual/themes.yml`
- `data/manual/supply_chains.yml`
- `data/manual/company_overrides.csv`
- `data/manual/ticker_overrides.csv`
- `data/manual/exposures.csv`
- `data/manual/notes/*.md`

Exposure rows include theme, layer, target, score, confidence, source URL, rationale, and last reviewed date.

## Enrichment

Yahoo Finance does not provide a stable official public API. Statos treats Yahoo-style data as replaceable enrichment, not the source of truth. Set `STATOS_ENRICHMENT_PROVIDER=yahoo` only when you want the builder to attempt live enrichment and cache the response locally.

## Safety

`.env`, raw snapshots, enrichment caches, and `.gocache/` are ignored. Generated static outputs under `site/data` are intended to be committed.

## License

PolyForm Noncommercial License 1.0.0. See `LICENSE`.
