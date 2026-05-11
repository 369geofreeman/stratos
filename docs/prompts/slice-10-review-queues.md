# Slice 10 Prompt: Review Queues And Taxonomy Worklists

Use this prompt for the implementation agent assigned to Slice 10.

## Implementation Prompt

You are working in the Statos repo.

Statos is a static GitHub Pages investment research site for Trading 212 Invest / Stocks ISA-compatible tickers. Trading 212 metadata is the source universe. Local Go code fetches, normalizes, enriches, applies manual taxonomy, and exports deterministic static files under `site/data`. The hosted site has no server.

Your task is to implement **Slice 10: Review Queues** from `docs/readiness-checklist.md`.

## Goal

Turn uncertainty into practical worklists that make taxonomy improvement fast.

The current live dataset is large enough that a single free-text `unclassified.csv` is no longer enough. After the full yfinance cache run, the site had roughly:

- 16,985 Trading 212 tickers
- 13,772 securities/identities
- 7,560 ticker-level enrichment successes
- 9,425 enrichment failures
- 16,964 unclassified rows, mostly because theme exposure has not been reviewed yet

The next workflow is manual category/theme/supply-chain review. Slice 10 should make that review guided, sortable, measurable, and hard to lose in refreshes.

## Hard Constraints

- Production remains fully static. No server, database, or API runtime.
- Keep the Go builder as the only writer of generated `site/data`.
- Keep generated outputs deterministic.
- Do not fetch Trading 212, Yahoo, or yfinance from the browser.
- Do not commit `.env`, `data/raw`, `data/cache`, `.gocache`, `.venv`, or secrets.
- Do not depend on the old Pluto repo.
- Preserve existing generated files and contracts unless you document and test a compatible migration.
- Keep frontend changes focused on review queues only. Broader frontend polish belongs to later Slice 7/8/9 work.

## Current Context

The repo already has:

- `site/data/unclassified.csv`
- `site/data/unclassified.json`
- `site/data/enrichment_failures.csv`
- `site/data/identity_issues.csv`
- `site/data/build_manifest.json`
- `site/data/app_bootstrap.json`
- `site/data/tickers_index.json`
- manual taxonomy files in `data/manual`
- `go run ./cmd/statos-build taxonomy coverage`
- `go run ./cmd/statos-build taxonomy exposure-template`
- static frontend view for the unclassified queue
- lazy loading so the frontend does not fetch `catalogue.json` at startup

The current `unclassified.reason` is a semicolon-separated string such as:

```text
missing sector; missing industry; missing theme exposure
```

This is useful for humans, but insufficient for filtering, trend reporting, and suggested manual edits.

## Required Outcome

Implement structured review queues with stable reason codes, generated summaries, and suggested manual-edit rows.

At minimum, support these issue families:

- taxonomy/classification gaps
- enrichment failures or ambiguous results
- identity issues or low-confidence identity mappings
- stale manual reviews

Keep the existing CSVs where possible for backwards compatibility, but add structured outputs the frontend and weekly workflow can consume.

## Required Reason Codes

Add explicit reason codes instead of relying only on prose.

Use a stable snake_case vocabulary. Suggested starting set:

- `missing_sector`
- `missing_industry`
- `missing_theme_exposure`
- `missing_company_id`
- `missing_isin`
- `identity_low_confidence`
- `identity_collision`
- `identity_duplicate_ticker`
- `identity_override_unmatched`
- `enrichment_failed`
- `enrichment_ambiguous`
- `enrichment_cache_miss`
- `enrichment_stale`
- `manual_review_stale`

You may add codes if the current data model exposes more precise cases. Document every generated reason code in `docs/data-contract.md` or a dedicated review-queue doc.

## Generated Outputs

Choose the simplest deterministic file layout that works well with the current static data contract.

Strongly preferred output shape:

- `site/data/review_queues.json`
- `site/data/review_summary.json`
- `site/data/taxonomy_issues.csv`
- `site/data/enrichment_issues.csv`
- `site/data/identity_issues.csv` preserved, and add JSON if useful
- `site/data/stale_reviews.csv`
- `site/data/suggested_classification_overrides.csv`
- `site/data/suggested_exposures.csv`

If you choose different names, document why and update smoke checks, docs, and frontend links.

`review_queues.json` should be optimized for frontend loading and should not duplicate the full catalogue. Each row should contain compact identifiers and display fields only.

Suggested row fields:

- `queue`: `taxonomy`, `enrichment`, `identity`, or `stale_review`
- `reasonCode`
- `severity`: `high`, `medium`, or `low`
- `ticker`
- `isin`
- `companyId`
- `securityId`
- `name`
- `sector`
- `industry`
- `themeIds`
- `layerIds`
- `sourceFile`
- `sourceRow`
- `suggestedAction`
- `suggestedManualFile`
- `suggestedCsvRow`
- `lastReviewed`
- `lastRefreshed`

Do not include large nested objects or raw provider responses.

`review_summary.json` should include counts by:

- queue
- reason code
- severity
- sector gap
- industry gap
- theme gap
- enrichment status/failure type
- identity issue type
- stale review bucket

Include these counts in `build_manifest.json` if they are useful for weekly diffs.

## Suggested Manual Rows

Generate suggested manual rows where the builder has enough information to be useful.

Examples:

- Missing sector/industry with known ticker/company:
  - suggest a blank `classification_overrides.csv` row with target identifiers filled
- Missing theme exposure:
  - suggest a blank `exposures.csv` row with ticker, ISIN, company ID filled
- Enrichment failure:
  - suggest a `ticker_overrides.csv` row with ticker and identifiers filled, leaving Yahoo/sector/industry fields blank
- Identity ambiguity:
  - suggest an `identity_overrides.csv` row with target identifiers filled

Do not hallucinate classifications, themes, layers, or sources. Leave unknown manual fields blank. The goal is to reduce copy/paste effort, not automate taxonomy judgment.

CSV suggestions must:

- use the exact current manual file headers
- be deterministic
- be safe to paste into the matching manual file after a human fills blanks
- include enough identifiers to avoid ambiguous manual edits

## Stale Review Rules

Add reviewed-date aging thresholds.

Suggested defaults:

- `manual_high`: stale after 180 days
- `manual_medium`: stale after 120 days
- `rule_low`: stale after 60 days
- any manual row with missing `last_reviewed`: stale immediately

Apply stale checks to manual files where `last_reviewed` exists:

- `data/manual/exposures.csv`
- `data/manual/classification_overrides.csv`
- `data/manual/company_overrides.csv`
- `data/manual/ticker_overrides.csv`
- `data/manual/identity_overrides.csv`
- `data/manual/relationships.csv`
- note frontmatter where applicable

If you choose different thresholds, document them.

## Manifest Trend Fields

If a previous `site/data/build_manifest.json` exists before writing the new one, compare review counts and include trend fields.

Suggested fields:

- `reviewQueueCounts`
- `reviewReasonCounts`
- `reviewQueueDeltas`
- `reviewReasonDeltas`
- `previousBuildAt`

Keep this deterministic and robust if no previous manifest exists. Do not fail a first build because there is no previous manifest.

Avoid self-referential checksum issues. Follow the existing manifest generated-file checksum approach.

## Frontend Requirements

Keep frontend changes focused and static.

Update the review/unclassified UI so a researcher can:

- filter by queue
- filter by reason code
- filter by severity
- filter by sector/industry/theme gap
- search within review rows
- see counts by reason
- copy a suggested manual CSV row
- copy an exposure/classification/identity/ticker override template where available

Do not redesign the broader app. Do not start Slice 7/8/9 polish in this work.

The UI must continue to:

- avoid fetching `catalogue.json` during startup
- lazy-load review queue data only when the review view needs it
- cap initial DOM rows and use the existing load-more pattern
- handle missing optional fields gracefully
- remain readable with the full live dataset

## Data Contract And Docs

Update at least:

- `internal/catalogue` types/build logic as needed
- `internal/export/export.go`
- exporter tests/goldens as needed
- `scripts/smoke.sh`
- `docs/data-contract.md`
- `docs/build-checklist.md`
- `docs/readiness-checklist.md`
- `docs/taxonomy-workflow.md`
- `README.md`
- `site/assets/app.js`
- `site/assets/styles.css` only if needed

If adding generated files, include them in:

- `build_manifest.json.generatedFiles`
- smoke checks
- export/download links
- data contract docs
- frontend loading logic if used by UI

## Tests

Add focused tests for queue generation.

Required coverage:

- unclassified prose reasons map to stable reason codes
- taxonomy queue rows are generated for missing sector, industry, and theme exposure
- enrichment issues are represented without raw provider payloads
- identity issues remain visible and are linked to structured review rows where possible
- stale manual rows are detected from `last_reviewed`
- missing `last_reviewed` in reviewable manual rows becomes stale
- suggested manual CSV rows use exact manual file headers/order
- generated queue ordering is deterministic
- manifest review counts and deltas are correct with and without a previous manifest
- frontend-compatible JSON shapes remain stable

Prefer small fixture-based Go tests. Do not require network access.

## Acceptance Criteria

- Structured reason codes exist for review rows.
- Separate taxonomy, enrichment, identity, and stale-review queues are generated or represented in a single structured queue file.
- Suggested manual rows are generated where enough identifiers exist.
- Counts by queue/reason/gap are generated.
- Reviewed-date aging thresholds are implemented and documented.
- Manifest trend fields are included when a previous manifest is available.
- Frontend review queue supports filtering by reason and copying suggested manual rows.
- Startup still does not fetch `data/catalogue.json`.
- `make sample` passes.
- `make test` passes.
- `make smoke` passes.
- `python3 scripts/data-status.py --require-live` still works when live data is present.
- `node --check site/assets/app.js` passes.
- `git diff --check` passes.
- `git ls-files .env data/raw data/cache .gocache .venv` prints nothing.

## Out Of Scope

Do not:

- Complete the actual taxonomy/category review for all sectors/themes.
- Invent sector, industry, theme, or exposure values.
- Add AI-assisted classification.
- Add scheduled GitHub Actions refresh.
- Add GitHub secrets or production server code.
- Replace the yfinance helper or enrichment provider layer.
- Redesign the supply-chain map.
- Finish all remaining frontend UX items from Slice 7.
- Finish local browser-data work from Slice 9.

## Suggested Verification Commands

Run:

```sh
make sample
make test
make smoke
node --check site/assets/app.js
git diff --check
git ls-files .env data/raw data/cache .gocache .venv
```

If live data is present locally, also run:

```sh
python3 scripts/data-status.py --require-live
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy coverage
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy exposure-template --out /tmp/statos-exposure-template.csv
```

If the implementation changes generated live `site/data`, do not run `make sample` at the end unless the intended final state is sample data. For a publishable dataset, finish with live or raw-replay generated files and state exactly which source mode the final `site/data/build_manifest.json` reports.

## Review Notes

After implementation, report:

- Files changed.
- New generated files and their purpose.
- Review queue counts from the manifest.
- Reason code vocabulary implemented.
- Suggested manual row files generated.
- Stale review thresholds used.
- Whether trend fields were available or skipped due to missing previous manifest.
- Frontend files loaded by the review queue view.
- Verification commands and results.
- Any checklist items intentionally left unchecked.

Do not sign off if review rows still only expose free-text reason strings or if the UI cannot filter by reason code.
