# Prompt: Slice 3 Identity Resolution And Normalization

Use this prompt for the implementation agent assigned to Slice 3.

## Implementation Prompt

You are working in the Statos repo.

Statos is a static investment research website for Trading 212 Invest / Stocks ISA-compatible tickers. The local Go builder fetches or replays Trading 212 metadata, normalizes broker instruments into durable research entities, applies manual taxonomy/overrides, and exports committed static files under `site/data`.

Your task is to implement **Slice 3: Identity Resolution And Normalization** from `docs/readiness-checklist.md`.

### Goal

Separate broker tickers, listings, securities, and companies cleanly, while making ambiguous or low-confidence identity decisions visible instead of silently collapsing data.

### Current Model

The existing model already has:

- `Instrument`: Trading 212 broker item.
- `Ticker`: exported broker ticker-level row.
- `Security`: stable security-level entity, currently keyed by ISIN when present.
- `Listing`: exchange/currency/listing representation.
- `Company`: business entity.
- Manual `ticker_overrides.csv` and `company_overrides.csv`.
- First-pass broker ticker parsing.
- First-pass ISIN grouping.
- First-pass directionality detection for inverse/short/leveraged instruments.

### Scope

Implement the remaining Slice 3 items that can be completed locally:

- Expand broker ticker parsing with observed Trading 212 ticker patterns.
- Add manual identity merge/split overrides for cases where ISIN is missing, shared incorrectly, or not enough to identify the business entity.
- Expand company ID override support for ADRs, dual listings, funds, trusts, and ETFs.
- Add normalized instrument category classification:
  - `stock`
  - `etf`
  - `fund`
  - `investment_trust`
  - `warrant`
  - `crypto`
  - `forex`
  - `bond`
  - `commodity`
  - `other`
- Add identity/structure flags where inferable:
  - `inverse`
  - `short`
  - `leveraged`
  - `synthetic`
  - `hedged`
  - `accumulating`
  - `distributing`
  - `adr`
  - `gdr`
  - `fund_like`
- Add tests for inverse/short/leveraged markers using realistic Trading 212-style tickers and names.
- Add duplicate/collision reports to the build manifest.
- Add generated CSV output for securities and listings if useful for manual review.
- Add identity confidence fields and reasons to exported entities.
- Add or extend review queues for ambiguous identity cases.
- Update `docs/readiness-checklist.md` checkboxes for completed Slice 3 work only.
- Update README/build docs if commands, generated files, or manual override files change.

### Suggested Data Model Additions

Prefer small, explicit fields over opaque blobs.

Suggested exported fields:

- On `Ticker`:
  - `instrumentCategory`
  - `structureFlags`
  - `identityConfidence`
  - `identityReasons`
- On `Security`:
  - `identityConfidence`
  - `identityReasons`
  - `structureFlags`
- On `Company`:
  - `identityConfidence`
  - `identityReasons`

Suggested manifest additions:

- `emptyTickerCount`
- `duplicateTickerCount`
- `duplicateISINCount`
- `missingISINCount`
- `identityCollisionCount`
- `identityOverrideCount`
- `instrumentCategoryCounts`
- `structureFlagCounts`

Suggested review output:

- `site/data/identity_issues.csv`

Suggested columns:

```csv
issue_code,ticker,isin,security_id,company_id,name,reason,suggested_action
```

Example issue codes:

- `missing_ticker`
- `missing_isin`
- `duplicate_ticker`
- `shared_isin_multiple_companies`
- `manual_override_unknown_ticker`
- `manual_override_conflict`
- `low_confidence_company_identity`

### Manual Override Design

If new manual identity files are needed, keep them committed and easy to edit under `data/manual`.

Suggested file:

```text
data/manual/identity_overrides.csv
```

Suggested columns:

```csv
target_type,ticker,isin,security_id,company_id,override_security_id,override_company_id,category,flags,confidence,reason,source_url,last_reviewed
```

Guidelines:

- `target_type` can be `ticker`, `isin`, `security`, or `company`.
- `override_security_id` can force a security merge/split.
- `override_company_id` can force a company mapping.
- `category` can override instrument category.
- `flags` can be semicolon-separated.
- `confidence` should use clear values such as `manual_high`, `manual_medium`, `rule_medium`, `rule_low`.
- Manual overrides must win over rule/provider inference.
- Invalid overrides should fail fast with useful errors.

Do not add a new file if existing `ticker_overrides.csv` and `company_overrides.csv` can be extended cleanly without becoming confusing. If you extend existing files, document the new columns.

### Inference Rules

Use conservative deterministic rules. Do not infer more than the data supports.

Broker ticker parsing:

- Continue handling patterns like `NVDA_US_EQ`.
- Preserve the full broker ticker as the stable broker-level ID.
- Extract base symbol, exchange/venue code, and asset code when the suffix is parseable.
- If parsing is uncertain, keep the raw ticker and add an identity reason instead of failing.

Category examples:

- Trading 212 `type=STOCK` -> `stock` unless manual override says otherwise.
- Trading 212 `type=ETF` -> `etf`.
- Names containing `Investment Trust` may classify as `investment_trust` if type/context supports it.
- Names/tickers indicating crypto/forex/bonds/commodities should be classified only with clear markers or manual overrides.
- Unknown types should become `other` and appear in identity issues.

Flag examples:

- `SHORT`, `INVERSE`, `-1X`, `1S`, `X1S` -> `short` or `inverse`.
- `2X`, `3X`, `X2`, `X3`, `LEVERAGED` -> `leveraged`.
- `ACC`, `ACCUMULATING` -> `accumulating`.
- `DIST`, `DISTRIBUTING` -> `distributing`.
- `HEDGED`, `GBP HEDGED`, `USD HEDGED` -> `hedged`.
- `ADR`, `ADS`, `American Depositary` -> `adr`.
- `GDR`, `Global Depositary` -> `gdr`.
- ETF/fund/trust categories should also add `fund_like`.

### Out Of Scope

Do not:

- Add live Trading 212 refresh behavior beyond what Slice 2 already supports.
- Add scheduled GitHub Actions refresh.
- Add provider/Yahoo enrichment changes except where category/identity fields need manual overrides.
- Build the full relationships/peer group model. That is a later slice.
- Overhaul the frontend design.
- Hide ambiguous instruments by dropping them from exports.
- Commit `.env`, raw snapshots, caches, or `.gocache`.

### Acceptance Criteria

- `make sample` passes.
- `make test` passes.
- `make smoke` passes.
- Existing `site/data` outputs remain valid.
- New identity fields appear in `catalogue.json` and `tickers.csv` where appropriate.
- `site/data/securities.csv` and/or `site/data/listings.csv` are generated if the implementation chooses to add CSV review outputs.
- Identity issues are exported when collisions, missing IDs, invalid overrides, or low-confidence mappings are detected.
- Manifest includes duplicate/collision/missing identity counts.
- Manual identity overrides are validated and deterministic.
- ISIN grouping still works.
- A company can intentionally own multiple securities/listings/tickers.
- No instrument with a non-empty Trading 212 ticker is silently dropped.
- `docs/readiness-checklist.md` marks only completed Slice 3 work as done.

### Required Tests

Add focused Go tests for:

- Broker ticker parsing with normal and unusual Trading 212-style tickers.
- ISIN grouping across multiple listings.
- Missing ISIN fallback identity.
- Manual company/security override behavior.
- Inverse/short/leveraged marker detection.
- Accumulating/distributing/hedged/ADR/GDR/fund-like flag detection where implemented.
- Instrument category classification.
- Identity issue generation.
- Manifest identity counts.
- Export generation for any new CSV/JSON output.

### Suggested Verification Commands

Run:

```sh
make sample
make test
make smoke
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh --no-fetch
make smoke
```

Then verify private local files are still ignored:

```sh
git ls-files .env data/raw data/cache .gocache
git status --short --ignored
```

Before committing, restore deterministic sample output unless intentionally committing replay/live generated data:

```sh
make sample
make smoke
```

### Final Response

Summarize:

- Files changed.
- Identity model changes.
- Manual override file/column changes.
- New manifest fields and review outputs.
- Commands run and results.
- Any Slice 3 checklist items intentionally left unchecked and why.
- Any risks or follow-up work.

## Reviewer Prompt

Review the Slice 3 Identity Resolution And Normalization implementation as a strict code reviewer.

Prioritize findings over summary. Look especially for:

- Instruments silently dropped or hidden from exports.
- ISIN grouping regressions.
- ADR/GDR/ETF/fund/trust instruments incorrectly merged into operating companies without explicit confidence.
- Manual overrides that can silently conflict or reference missing tickers/companies.
- Non-deterministic IDs, ordering, generated files, or manifest counts.
- Identity confidence fields that always say high even when rule-based or ambiguous.
- Duplicate/collision reports that miss obvious conflicts.
- Tests that only cover the sample happy path.
- New generated files not covered by smoke checks.
- Frontend assumptions broken by renamed JSON fields.
- Raw snapshots, `.env`, caches, or `.gocache` accidentally tracked.

For each finding, include severity, file/line reference, and the concrete failure mode. If there are no findings, state that clearly and list residual risks or test gaps.
