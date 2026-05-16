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
4. conservative raw broker metadata fallback where available

Within `classification_overrides.csv`, broad company rows are applied first, then ISIN rows, then ticker rows.

## Generated Fund Classification

When provider and manual classification are both missing, the builder classifies clear fund-like and structured instruments from Trading 212 type/name metadata. This fallback does not classify operating companies.

- ETFs and funds use sector `Funds`.
- Warrants use sector `Structured Products`.
- Investment trusts use industry `Investment Trust`.
- ETF/fund industries are chosen from clear markers: `Equity ETF`, `Bond ETF`, `Commodity ETP`, `Crypto ETP`, `Leveraged ETP`, `Inverse ETP`, `Covered Call ETF`, `Money Market Fund`, `Multi-Asset Fund`, `Factor ETF`, or `Fund`.

Manual classification rows still win when a reviewed source supports a more precise classification.

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

Open the static site and use the Review queues view, or inspect:

- `site/data/review_summary.json` for counts by queue, reason, severity, taxonomy gap, enrichment status, identity issue type, stale bucket, and suggested manual file.
- `site/data/review_queues.json` for compact structured rows with stable reason codes.
- `site/data/taxonomy_issues.csv` for missing sector, industry, company ID, ISIN, and theme exposure work.
- `site/data/enrichment_issues.csv` and `site/data/enrichment_failures.csv` for enrichment cache/provider work.
- `site/data/identity_issues.csv` for collisions, duplicates, low-confidence mappings, and unmatched identity overrides.
- `site/data/stale_reviews.csv` for manual rows or notes whose `last_reviewed` date is missing or old.

`site/data/unclassified.csv` is still generated for compatibility, but the structured queue files are the primary review workflow.

## Suggested Manual Rows

The builder writes paste-friendly suggestion files with exact manual headers:

- `site/data/suggested_classification_overrides.csv`
- `site/data/suggested_exposures.csv`
- `site/data/suggested_ticker_overrides.csv`
- `site/data/suggested_identity_overrides.csv`

Suggested rows pre-fill identifiers only. Fill sector, industry, theme, layer, confidence, source URL, rationale, override fields, and `last_reviewed` yourself before pasting into `data/manual`.

The Review queues UI also exposes `suggestedCsvRow` per row and a copy button when a template is available.

## Stale Review Rules

Manual rows with missing `last_reviewed` are stale immediately. Reviewed rows age out using these thresholds:

- `manual_high` and `rule_high`: 180 days.
- `manual_medium`, `rule_medium`, and manual rows without a confidence column: 120 days.
- `manual_low` and `rule_low`: 60 days.

Stale checks apply to `exposures.csv`, `classification_overrides.csv`, `company_overrides.csv`, `ticker_overrides.csv`, `identity_overrides.csv`, `relationships.csv`, and note frontmatter when `last_reviewed` is present or missing.

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
last_reviewed: 2026-05-10
---
```

When frontmatter is present, `target_type`, `target_id`, and `title` are required. `last_reviewed` is optional but recommended so notes do not enter the stale-review queue immediately. Supported target types are `ticker`, `company`, `security`, `sector`, `industry`, `theme`, and `layer`. Empty note bodies fail validation.

## Validation Errors

Manual loaders reject unknown CSV columns, duplicate headers, malformed YAML fields, malformed scores, unknown confidence values, invalid dates, invalid URLs, missing exposure targets, unknown themes/layers, and malformed note frontmatter. Errors include the file path and row or line number where possible.

Fix the manual file, then rerun the build. Bad manual taxonomy fails before generated `site/data` files are written.
