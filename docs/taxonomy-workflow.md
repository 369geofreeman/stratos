# Manual Taxonomy Workflow

Use this workflow after a sample, replay, or live refresh to turn review queues into committed manual taxonomy edits.

## Manual Files

- `data/manual/themes.yml`: committed theme IDs, names, descriptions, and optional `#RRGGBB` colours.
- `data/manual/supply_chains.yml`: committed layer maps for each theme.
- `data/manual/exposures.csv`: manually reviewed theme/layer exposure rows for tickers, ISINs, or companies.
- `data/manual/classification_overrides.csv`: manual sector, industry, and country overrides separate from provider enrichment.
- `data/manual/relationships.csv`: manual peer, substitute, supplier, customer, and related-play rows.
- `data/manual/ticker_overrides.csv`: legacy ticker-level identity, listing, Yahoo symbol, and classification overrides.
- `data/manual/company_overrides.csv`: legacy company-level name and classification overrides.
- `data/manual/identity_overrides.csv`: manual security/company identity corrections, category corrections, and structure flags.
- `data/manual/notes/*.md`: frontmatter-backed research notes.

Classification precedence is deterministic:

1. `classification_overrides.csv`
2. existing ticker/company manual overrides
3. provider enrichment
4. raw broker metadata fallback where available

Within `classification_overrides.csv`, broad company rows are applied first, then ISIN rows, then ticker rows.

## Weekly Review

Run a refresh or replay, then inspect the review queues:

```sh
make refresh
make smoke
```

For offline replay:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh --no-fetch
make smoke
```

Open `site/data/unclassified.csv`. Each row names the ticker, company ID, ISIN, and missing classification or exposure reason. Use it to decide whether to add a classification override, an exposure row, an identity override, or a note.

## Coverage Command

Print paste-friendly taxonomy coverage without network calls:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy coverage
```

Use another generated catalogue if needed:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy coverage --catalogue /tmp/catalogue.json
```

The report includes theme coverage, layer exposure counts and confidence mix, sector counts, industry counts, and the unclassified count.

## Exposure Templates

Generate empty exposure rows from `site/data/unclassified.csv`:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy exposure-template > /tmp/statos-exposure-template.csv
```

Or write an explicit output file:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy exposure-template --out /tmp/statos-exposure-template.csv
```

The template uses the exact `data/manual/exposures.csv` header, pre-fills `ticker`, `isin`, and `company_id`, and leaves manual decision fields blank. It does not append to committed manual files.

## Add Themes And Layers

Add a theme in `themes.yml` with a slug ID, non-empty name, optional description, and optional `#RRGGBB` colour.

Add a matching supply chain in `supply_chains.yml` with a known `theme_id`, non-empty chain name, and at least one layer. Layer IDs must be unique within a theme, layer names must be non-empty, and layer orders must be unique integers within the chain.

## Add Exposure Rows

Add rows to `data/manual/exposures.csv` after review. Required fields are:

- `theme_id`
- `layer_id`
- at least one of `ticker`, `isin`, or `company_id`
- `exposure_score` from `0` to `5`
- allowed `confidence`
- absolute `http` or `https` `source_url`
- `last_reviewed` as `YYYY-MM-DD`

Allowed confidence values are `manual_high`, `manual_medium`, `manual_low`, `rule_high`, `rule_medium`, and `rule_low`.

## Add Classification Overrides

Use `data/manual/classification_overrides.csv` when provider sector, industry, or country data is missing or wrong. Set `target_type` to `ticker`, `isin`, or `company`, and populate exactly one matching target field. Fill any of `sector`, `industry`, and `country`.

Existing `ticker_overrides.csv` and `company_overrides.csv` classification columns remain supported, but the dedicated classification file wins when both specify a value.

## Add Relationship Rows

Use `data/manual/relationships.csv` for reviewed relationships. Allowed `relationship_type` values are `peer`, `substitute`, `upstream_supplier`, `downstream_customer`, and `related_play`.

Each row must set exactly one source target and exactly one target target. `theme_id` and `layer_id` are optional, but if `layer_id` is present it must belong to the supplied `theme_id`. Confidence, source URL, and last reviewed date are required.

## Notes

Notes with frontmatter must use only these keys:

```yaml
---
target_type: ticker
target_id: NVDA_US_EQ
title: NVIDIA AI infrastructure note
tags: ai, accelerators
---
```

When frontmatter is present, `target_type`, `target_id`, and `title` are required. Supported target types are `ticker`, `company`, `security`, `sector`, `industry`, `theme`, and `layer`. Empty note bodies fail validation.

## Validation Errors

Manual loaders reject unknown CSV columns, duplicate headers, malformed YAML fields, malformed scores, unknown confidence values, invalid dates, invalid URLs, missing exposure targets, unknown themes/layers, and malformed note frontmatter. Errors include the file path and row or line number where possible.

Fix the manual file, then rerun the build. Bad manual taxonomy fails before generated `site/data` files are written.
