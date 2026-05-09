# Prompt: Slice 2 Trading 212 Live Ingestion

Use this prompt for the implementation agent assigned to Slice 2.

## Implementation Prompt

You are working in the Statos repo.

Statos is a static investment research website for Trading 212 Invest / Stocks ISA-compatible tickers. Production is GitHub Pages serving committed files from `site/`. A local Go builder fetches source data, writes ignored raw snapshots/caches locally, normalizes data, and exports committed `site/data`.

Your task is to implement **Slice 2: Trading 212 Live Ingestion** from `docs/readiness-checklist.md`.

### Goal

Reliably collect and replay the Trading 212 account-accessible instrument universe from official metadata endpoints, while keeping credentials and private raw data out of Git.

### Current Official API Constraints

Use the official Trading 212 docs as the source of truth:

- API environments: https://docs.trading212.com/api/section/general-information/api-environments
- Instruments metadata: https://docs.trading212.com/api/instruments/instruments
- Exchanges metadata: https://docs.trading212.com/api/instruments/exchanges
- Optional account summary: https://docs.trading212.com/api/accounts/getaccountsummary

Important constraints from the docs:

- Public API is for Invest and Stocks ISA account types.
- Demo base URL: `https://demo.trading212.com/api/v0`
- Live base URL: `https://live.trading212.com/api/v0`
- Authentication is HTTP Basic auth using API key as username and API secret as password.
- `GET /api/v0/equity/metadata/exchanges`
  - Returns accessible exchanges and working schedules.
  - Data refreshes every 10 minutes.
  - Rate limit: `1 req / 30s`.
- `GET /api/v0/equity/metadata/instruments`
  - Returns accessible instruments.
  - Data refreshes every 10 minutes.
  - Rate limit: `1 req / 50s`.
- Optional sanity endpoint: `GET /api/v0/equity/account/summary`
  - Rate limit: `1 req / 5s`.
- Relevant failure responses include `401`, `403`, `408`, and `429`.
- API responses include rate-limit headers:
  - `x-ratelimit-limit`
  - `x-ratelimit-period`
  - `x-ratelimit-remaining`
  - `x-ratelimit-reset`
  - `x-ratelimit-used`

### Scope

Implement the remaining Slice 2 items that can be handled locally:

- Verify the Trading 212 client can call instruments and exchanges using `.env` credentials only.
- Capture HTTP status, endpoint, request time, response time, and rate-limit headers for metadata calls.
- Add friendly errors for `401`, `403`, `408`, `429`, and unexpected statuses.
- Keep timestamped raw snapshots and `latest` aliases.
- Add a replay mode to rebuild from latest raw snapshots without fetching:
  - Suggested flag: `go run ./cmd/statos-build refresh --no-fetch`
  - It should read `data/raw/trading212/instruments_latest.json` and `data/raw/trading212/exchanges_latest.json`.
- Add an input raw directory option to replay older snapshots:
  - Suggested flag: `--input-raw-dir data/raw/trading212`.
- Add tests with fixture JSON matching Trading 212 instruments and exchanges responses.
- Add manifest fields for:
  - Trading 212 base URL or environment.
  - Fetch timestamp.
  - Raw snapshot timestamp/path summary.
  - Per-endpoint HTTP diagnostics.
  - Per-endpoint rate-limit observations.
  - Whether the build used live fetch, sample data, or raw replay.
- Update `docs/readiness-checklist.md` checkboxes for completed Slice 2 items only.
- Update `README.md` and `docs/build-checklist.md` with the live fetch and replay workflow.

### Out Of Scope

Do not:

- Add scheduled GitHub Actions refresh.
- Add Trading 212 secrets to GitHub Actions.
- Commit `.env`, `data/raw`, `data/cache`, or `.gocache`.
- Add order placement, portfolio trading, or other execution endpoints.
- Expand Yahoo/enrichment behavior except as needed to keep existing refresh output working.
- Solve full identity resolution beyond preserving and replaying source data accurately.
- Change the static frontend unless a manifest field rename would otherwise break it.

### Implementation Notes

- Keep the builder safe by default:
  - If credentials are absent and `--no-fetch` is not set, current sample fallback may remain.
  - If `--no-fetch` is set and raw files are missing, fail clearly.
  - If credentials are present, use `.env` values and do not print secrets.
- Prefer a small internal type for request diagnostics, for example:
  - endpoint name
  - path
  - started at
  - completed at
  - duration
  - status code
  - error code/category
  - rate-limit headers
- Store diagnostics in the manifest without leaking credentials or full authorization headers.
- Consider making raw snapshot writes atomic enough that interrupted writes do not corrupt `latest` files.
- Use fixture tests for decoding real response shapes instead of relying only on sample data.
- The Trading 212 docs currently show `workingScheduleId` on instruments and exchange working schedules. Preserve unknown fields only if useful, but do not fail on new fields from the API.
- Rate limits should be captured and surfaced. Do not implement aggressive retry loops that can burn the account limit.
- A `429` error should mention the reset header/time if available.

### Acceptance Criteria

- `make sample` still works.
- `make test` passes.
- `make smoke` passes.
- A no-fetch replay command works from generated raw sample snapshots.
- Missing raw replay files produce a clear error.
- Trading 212 fixture tests cover instruments and exchanges decoding.
- Friendly HTTP error tests cover at least `401`, `403`, `408`, and `429`.
- Build manifest makes source mode and Trading 212 fetch/replay diagnostics visible.
- Live refresh can be run locally with `.env` credentials without committing raw snapshots.
- `docs/readiness-checklist.md` marks only verified Slice 2 work as done.

### Suggested Verification Commands

Run:

```sh
make sample
make test
make smoke
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh --no-fetch
make smoke
```

If credentials are available locally, run exactly one live metadata refresh while respecting rate limits:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh
```

Then verify private raw files are still ignored:

```sh
git ls-files .env data/raw data/cache .gocache
git status --short --ignored
```

### Final Response

Summarize:

- Files changed.
- Fetch/replay behavior added.
- Manifest fields added.
- Commands run and results.
- Whether a real Trading 212 refresh was run or skipped.
- Any Slice 2 checklist items intentionally left unchecked and why.
- Any risks or follow-up work.

## Reviewer Prompt

Review the Slice 2 Trading 212 Live Ingestion implementation as a strict code reviewer.

Prioritize findings over summary. Look especially for:

- Secrets printed, stored, committed, or exposed through manifest/logs.
- Raw snapshots, caches, `.env`, or `.gocache` accidentally tracked.
- Live fetch behavior inside GitHub Actions or Pages deployment.
- `--no-fetch` replay reading the wrong files, silently falling back to sample, or masking missing raw data.
- Non-deterministic replay output.
- HTTP client failures that hide status codes or response context.
- `401`, `403`, `408`, and `429` errors that are not actionable.
- Rate-limit headers not captured or not surfaced.
- API client too strict about unknown response fields.
- Manifest fields that break the existing frontend.
- Tests that only cover sample data and miss Trading 212 fixture shapes.
- Any command that can accidentally place orders or use non-metadata endpoints beyond optional account summary.

For each finding, include severity, file/line reference, and the concrete failure mode. If there are no findings, state that clearly and list residual risks or test gaps.
