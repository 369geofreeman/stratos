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

## Setup

```sh
cp .env.example .env
```

Fill in Trading 212 credentials if you want to use live account metadata. With no credentials, the builder falls back to the sample dataset so the site remains usable during development.

```sh
go run ./cmd/statos-build refresh
go test ./...
```

Preview the site:

```sh
cd site
python3 -m http.server 4173
```

Open `http://localhost:4173`.

## Builder Commands

```sh
go run ./cmd/statos-build refresh
go run ./cmd/statos-build refresh --sample
go run ./cmd/statos-build sample
```

`refresh` fetches Trading 212 metadata when credentials are present. `sample` always uses the embedded sample universe and is useful for UI development.

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

`.env`, raw snapshots, and cache files are ignored. Generated static outputs under `site/data` are intended to be committed.

## License

PolyForm Noncommercial or CC BY-NC 4.0
