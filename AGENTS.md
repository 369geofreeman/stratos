# Statos Agent Notes

Statos is a static investment research site plus a local Go builder. The hosted site must work on GitHub Pages with no production server.

## Ground Rules

- Do not depend on `/Users/geofreeman/code/aureus_technologies/pluto`.
- Do not commit secrets, `.env`, raw Trading 212 snapshots, or enrichment caches.
- Generated `site/data` outputs are intentionally committed for GitHub Pages.
- Keep raw, cache, manual, normalized, and generated data separate.
- Trading 212 instrument metadata is the source universe; Yahoo-style enrichment is secondary and replaceable.
- Use ISIN as the stable security key when present. Broker tickers are listing/broker identifiers.
- Make enrichment failures visible in `site/data/build_manifest.json` and `site/data/unclassified.csv`.

## Local Workflow

1. Copy `.env.example` to `.env` and fill in Trading 212 credentials.
2. Run `go run ./cmd/statos-build refresh`.
3. Review `site/data/unclassified.csv`.
4. Update committed manual files under `data/manual`.
5. Run tests with `go test ./...`.
6. Preview `site` with a local static server.
7. Commit code, manual taxonomy, and generated `site/data`.

## Data Ownership

- `data/raw/`: ignored raw API snapshots.
- `data/cache/`: ignored provider cache.
- `data/manual/`: committed human-maintained taxonomy, overrides, exposures, and notes.
- `site/data/`: committed generated static JSON/CSV.

## Design Direction

The first screen is the research interface, not a marketing page. Keep the UI dense, calm, sortable, filterable, and useful for repeated review.
