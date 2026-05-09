# Prompt: Slice 4 Enrichment Provider Layer

Use this prompt for the implementation agent assigned to Slice 4.

## Implementation Prompt

You are working in the Statos repo.

Statos is a static investment research website for Trading 212 Invest / Stocks ISA-compatible tickers. Trading 212 metadata is the source universe. Enrichment providers, including Yahoo-style data, are secondary, replaceable, cached, and failure-prone.

Your task is to implement **Slice 4: Enrichment Provider Layer** from `docs/readiness-checklist.md`.

### Goal

Make enrichment provider behavior explicit, cacheable, inspectable, and replaceable. Enrichment should improve sector/industry/symbol/market-cap coverage when possible, but failures or ambiguous matches must be visible and must not block catalogue generation.

### Current Context

The repo already has:

- `internal/enrichment.Provider`
- cache-first provider wrapper
- optional Yahoo-compatible provider
- ticker-derived Yahoo symbol candidates with exchange suffix mapping
- manual enrichment override fields for Yahoo symbol, sector, industry, and country
- builder behavior that continues when enrichment fails

Yahoo/yfinance constraints:

- Yahoo Finance does not provide a stable official public developer API.
- yfinance docs state it is not affiliated, endorsed, or vetted by Yahoo and uses publicly available APIs for research/educational purposes.
- Treat Yahoo-style data as enrichment only, not source of truth.
- Keep the provider interface replaceable.
- Cache all responses or failures used by the builder.

Reference:

- yfinance docs: https://ranaroussi.github.io/yfinance/index.html

### Scope

Implement the remaining Slice 4 items that can be completed locally:

- Add enrichment cache schema/versioning.
- Add cache TTL/staleness reporting without forcing network calls.
- Add ISIN-first lookup path where provider supports it.
- Add ambiguous-match handling with candidate lists instead of trusting the first match.
- Add manual enrichment override fields for:
  - market cap
  - exchange
  - currency
- Add enrichment failure CSV:
  - `site/data/enrichment_failures.csv`
- Add generated enrichment diagnostics into the manifest.
- Add tests for:
  - cache hit
  - cache miss
  - stale cache
  - provider failure
  - cached failure replay
  - ambiguous match
  - manual override precedence
- Add provider interface documentation so Yahoo can be replaced later.
- Update `README.md`, `docs/build-checklist.md`, and `docs/readiness-checklist.md`.
- Update `scripts/smoke.sh` and export links for any new generated files.

### Suggested Data Model

Prefer explicit fields over opaque provider blobs.

Suggested cache envelope:

```json
{
  "schemaVersion": 1,
  "provider": "yahoo",
  "request": {
    "ticker": "VOD_L_EQ",
    "isin": "GB00BH4HKS39",
    "name": "Vodafone Group plc",
    "candidateSymbols": ["VOD.L", "VOD"]
  },
  "profile": {
    "symbol": "VOD.L",
    "name": "Vodafone Group plc",
    "sector": "Communication Services",
    "industry": "Telecom Services",
    "exchange": "LSE",
    "currency": "GBp",
    "country": "United Kingdom",
    "marketCap": 123
  },
  "candidates": [],
  "status": "hit",
  "error": "",
  "retrievedAt": "2026-05-09T12:00:00Z"
}
```

Suggested enrichment failure output:

```csv
ticker,isin,name,provider,attempted_symbols,status,error,next_action
```

Suggested manifest fields:

- `enrichmentCacheSchemaVersion`
- `enrichmentProvider`
- `enrichmentCacheHitCount`
- `enrichmentCacheMissCount`
- `enrichmentCacheStaleCount`
- `enrichmentAmbiguousCount`
- `enrichmentFailureCount`
- `enrichmentFailureCSV`
- `enrichmentOldestRetrievedAt`
- `enrichmentNewestRetrievedAt`

### Cache Behavior

The builder must remain useful offline.

- Cache lookup should be deterministic and should not require network access.
- Default provider mode should remain cache-only unless explicitly configured otherwise.
- Cache entries should include a schema version.
- Unknown schema versions should be visible, not silently trusted.
- Staleness should be reported in manifest/diagnostics, but stale entries may still be used unless the user opts into stricter behavior.
- Cached failures should be replayable and visible.
- Provider responses and failures should never write into `site/data` directly; only normalized/exported diagnostics should be committed.
- Cache files under `data/cache` remain ignored.

### Provider Behavior

For Yahoo-style lookup:

- Try ISIN-derived lookup/search first when possible.
- Then try candidate symbols derived from Trading 212 ticker.
- Preserve all plausible candidates when search is ambiguous.
- Do not blindly use the first search result if multiple plausible matches exist.
- If a match is ambiguous, either:
  - mark it ambiguous and do not apply provider fields, or
  - apply only if there is a strong deterministic reason and record that reason.
- Manual overrides must always win over provider data.
- Provider failures should produce useful next actions.

### Manual Override Design

Extend the existing committed manual override files if clean.

Likely extension:

```csv
data/manual/ticker_overrides.csv
```

Add optional columns:

- `market_cap`
- `exchange`
- `currency`

If you choose a different file, document why.

Validation expectations:

- `market_cap` must parse as an integer when present.
- Unknown or malformed fields should fail fast with row-specific errors.
- Manual fields should be visible in sources/identity reasons where appropriate.

### Out Of Scope

Do not:

- Add scheduled GitHub Actions refresh.
- Add API secrets to GitHub Actions.
- Treat Yahoo as source of truth.
- Add a paid provider unless only documenting the interface for future replacement.
- Add full taxonomy classification from enrichment alone.
- Redesign the frontend beyond adding export links or surfacing fields already present.
- Commit `data/cache`, `.env`, raw snapshots, or `.gocache`.

### Acceptance Criteria

- `make sample` passes.
- `make test` passes.
- `make smoke` passes.
- `refresh --no-fetch` still works.
- `site/data/enrichment_failures.csv` is generated and included in smoke checks.
- Manifest includes cache/enrichment diagnostics.
- Cache entries include schema version.
- Stale cache entries are counted without forcing network calls.
- Provider failures are cached/replayed and exported.
- Ambiguous provider matches are not silently treated as successful enrichment.
- Manual override fields for market cap, exchange, and currency win over provider data.
- Provider interface documentation exists.
- `docs/readiness-checklist.md` marks only completed Slice 4 work as done.

### Required Tests

Add focused Go tests for:

- Cache hit returning a profile.
- Cache miss with cache-only provider.
- Cache write with schema version.
- Unknown cache schema version handling.
- Stale cache detection.
- Cached failure replay.
- Provider failure cached and exported.
- Ambiguous candidate handling.
- ISIN-first lookup ordering where provider supports it.
- Manual override precedence over provider profile fields.
- Export generation for `enrichment_failures.csv`.
- Manifest enrichment counts.

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
- Cache schema/version changes.
- Provider behavior changes.
- Manual override changes.
- Manifest fields and exported diagnostics.
- Commands run and results.
- Any Slice 4 checklist items intentionally left unchecked and why.
- Any risks or follow-up work.

## Reviewer Prompt

Review the Slice 4 Enrichment Provider Layer implementation as a strict code reviewer.

Prioritize findings over summary. Look especially for:

- Yahoo/provider data being treated as source of truth.
- Provider failures blocking catalogue generation.
- Ambiguous matches silently applied as successful enrichment.
- Manual overrides not winning over provider data.
- Cache files or raw provider responses committed under `site/data`.
- `data/cache`, `.env`, raw snapshots, or `.gocache` accidentally tracked.
- Cache schema version missing or ignored.
- Stale cache data silently used without manifest visibility.
- Cached failures not exported or not visible.
- Network calls happening in default/cache-only mode.
- Tests that require network access.
- Manifest fields or frontend assumptions broken by renamed JSON fields.
- New generated files missing from smoke checks or export links.

For each finding, include severity, file/line reference, and the concrete failure mode. If there are no findings, state that clearly and list residual risks or test gaps.
