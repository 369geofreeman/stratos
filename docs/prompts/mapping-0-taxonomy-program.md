# Mapping Program Prompt: Statos Taxonomy Completion

Use this prompt to coordinate the remaining mapping work before final frontend polish.

## Context

You are working in the Statos repo, a static GitHub Pages investment research site for Trading 212 Invest / Stocks ISA-compatible tickers.

The data pipeline now works. The blocker is mapping quality.

Current live generated data after the weekly refresh on 2026-05-16:

- 17,050 Trading 212 tickers
- 13,805 securities
- 13,216 companies
- 17,029 unclassified rows
- 9,459 missing sector
- 9,459 missing industry
- 17,029 missing theme exposure
- 9,348 enrichment failures
- 15,334 identity issues
- 11 manual exposure rows
- 0 manual relationships

The site is technically usable but not yet useful as a research catalogue. The goal is to reduce the review queues by adding reviewed, source-backed manual taxonomy and targeted rule/code improvements.

## Work Order

Do the work in this order:

1. Identity cleanup.
2. Sector and industry mapping.
3. Theme and supply-chain exposure mapping.
4. Relationships and peer groups.
5. Final frontend polish.

Do not start frontend polish until the mapping data is materially better.

## Hard Constraints

- Production remains fully static.
- Do not add a production server, database, or runtime API.
- Do not depend on the old Pluto repo.
- Do not commit `.env`, `data/raw`, `data/cache`, `.gocache`, `.venv`, provider cache, or secrets.
- Treat Trading 212 as the source universe.
- Treat Yahoo/yfinance as replaceable enrichment, not source of truth.
- Keep generated `site/data` committed when the task intentionally changes generated outputs.
- Keep source-backed manual taxonomy separate from raw snapshots and provider caches.
- Do not invent classifications, exposures, or relationships without a defensible source.

## Current Manual Files

Manual taxonomy lives in:

- `data/manual/classification_overrides.csv`
- `data/manual/identity_overrides.csv`
- `data/manual/exposures.csv`
- `data/manual/relationships.csv`
- `data/manual/ticker_overrides.csv`
- `data/manual/company_overrides.csv`
- `data/manual/themes.yml`
- `data/manual/supply_chains.yml`
- `data/manual/notes/*.md`

Generated queues and suggestions live in:

- `site/data/review_queues.json`
- `site/data/review_summary.json`
- `site/data/taxonomy_issues.csv`
- `site/data/identity_issues.csv`
- `site/data/enrichment_issues.csv`
- `site/data/suggested_classification_overrides.csv`
- `site/data/suggested_exposures.csv`
- `site/data/suggested_identity_overrides.csv`
- `site/data/suggested_ticker_overrides.csv`

## Operating Rule

Each mapping pass should:

1. Measure the queue before changing anything.
2. Make focused source/manual/code changes.
3. Rebuild from the latest raw snapshot without unnecessary network calls.
4. Measure the queue after the change.
5. Report what improved and what remains.

Use raw replay for most mapping work:

```sh
STATOS_ENRICHMENT_PROVIDER=cache GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh --no-fetch
make test
make smoke
python3 scripts/data-status.py --require-live
```

Do not run `make sample` at the end unless the intended final state is sample data. For mapping work, the final generated `site/data/build_manifest.json` should remain live or live raw-replay derived.

## Quality Bar

- Manual rows must include `source_url` and `last_reviewed` where the file requires them.
- Use `manual_high` only for direct, source-backed, reviewed classifications/exposures.
- Use `manual_medium` when the source supports the direction but the exact exposure is estimated.
- Use `rule_low` only for broad rule-based or name-derived mapping.
- Keep rationales short and specific.
- Do not use a theme/layer as a catch-all just to reduce queue counts.
- Prefer fewer high-quality mappings over large unsupported guesses.

## Program-Level Acceptance Criteria

The mapping program is ready for final frontend polish when:

- Missing sector and industry counts are reduced to a manageable tail.
- The core high-value operating companies and funds have useful sector/industry classifications.
- AI infrastructure has a broad supply-chain map beyond the seed 11 rows.
- Semiconductors, energy, defence, healthcare, fintech, and commodities have meaningful first-pass exposure coverage.
- Relationships are non-empty and source-backed for key mapped names.
- Review queues still exist, but they guide cleanup rather than define the whole site.
- `make test`, `make smoke`, and `python3 scripts/data-status.py --require-live` pass.
