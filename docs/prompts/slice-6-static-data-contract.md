# Prompt: Slice 6 Static Data Contract

Use this prompt for the implementation agent assigned to Slice 6.

## Implementation Prompt

You are working in the Statos repo.

Statos is a static investment research website for Trading 212 Invest / Stocks ISA-compatible tickers. The Go builder writes committed static JSON/CSV files under `site/data`, and the plain HTML/CSS/JS frontend treats those files as its production API.

Your task is to implement **Slice 6: Static Data Contract** from `docs/readiness-checklist.md`.

### Goal

Treat `site/data` as a stable frontend data contract. Generated files should have documented schemas, version/freshness/checksum metadata, deterministic output, and tests that catch accidental breaking changes.

### Current Context

The repo already has:

- Generated JSON:
  - `site/data/catalogue.json`
  - `site/data/companies.json`
  - `site/data/sectors.json`
  - `site/data/industries.json`
  - `site/data/themes.json`
  - `site/data/supply_chains.json`
  - `site/data/search_index.json`
  - `site/data/build_manifest.json`
- Generated CSV:
  - `site/data/tickers.csv`
  - `site/data/securities.csv`
  - `site/data/listings.csv`
  - `site/data/unclassified.csv`
  - `site/data/identity_issues.csv`
  - `site/data/enrichment_failures.csv`
- `catalogue.json` already embeds tickers, securities, listings, companies, sectors, industries, themes, supply chains, exposures, notes, unclassified rows, identity issues, and manifest.
- Manual relationships are loaded and validated in Slice 5, but full relationship graph export/UI is still pending.
- Generated ordering is mostly deterministic already.
- The frontend has some backwards-compatible handling for missing optional fields.

Keep this slice focused on the generated data contract. Do not redesign the frontend UX or taxonomy workflow.

### Scope

Implement the remaining Slice 6 items that can be completed locally:

- Add documented JSON/data contract docs for every `site/data` file.
- Add schema/version fields to generated JSON outputs.
- Add standalone `site/data/securities.json`.
- Add standalone `site/data/listings.json`.
- Add `site/data/relationships.json` for loaded manual relationships.
- Add `site/data/sources.json` if source reuse can be exported cleanly; otherwise document why it remains out of scope.
- Document all CSV headers as stable contract fields.
- Add golden-file or snapshot-style tests comparing deterministic sample output to expected files.
- Add manifest checksums for generated outputs.
- Update smoke checks and export links for any new generated files.
- Update `README.md`, `docs/build-checklist.md`, and `docs/readiness-checklist.md`.

### Contract Versioning

Add explicit contract metadata without making the frontend brittle.

Suggested constants:

```go
const DataContractVersion = 1
const DataContractSchemaVersion = 1
```

Suggested manifest fields:

- `dataContractVersion`
- `schemaVersion`
- `generatedFiles`

Suggested per-file generated metadata:

- For top-level object JSON files, include:
  - `schemaVersion`
  - `generatedAt` or equivalent freshness value where appropriate
  - `data`
- For files that are naturally arrays, choose either:
  - keep array shape stable and document version in `build_manifest.json`, or
  - introduce object envelopes only where the frontend can be updated safely.

Be conservative. If changing an existing JSON file from an array to an object would create unnecessary frontend churn, keep the array stable and put its schema version in the manifest. `catalogue.json` and `build_manifest.json` are the best places for global metadata.

### Manifest Checksums

Add deterministic checksums for generated files.

Suggested manifest shape:

```json
{
  "dataContractVersion": 1,
  "schemaVersion": 1,
  "generatedFiles": [
    {
      "path": "site/data/catalogue.json",
      "format": "json",
      "schemaVersion": 1,
      "sha256": "...",
      "bytes": 12345
    }
  ]
}
```

Guidelines:

- Use SHA-256.
- Include every generated `site/data` file.
- Exclude volatile local-only files.
- Make output deterministic.
- Avoid self-referential checksum instability for `build_manifest.json`.

For `build_manifest.json`, either:

- checksum a stable manifest projection excluding `generatedFiles`, or
- include `build_manifest.json` in `generatedFiles` with a documented empty/checksum-excluded convention.

Document the choice clearly and test it.

### JSON Outputs

Add standalone JSON files where useful:

```text
site/data/securities.json
site/data/listings.json
site/data/relationships.json
```

Expected behavior:

- `securities.json` exports `cat.Securities`.
- `listings.json` exports `cat.Listings`.
- `relationships.json` exports loaded, validated manual relationships in deterministic order.
- Existing `catalogue.json` should remain a complete bundle unless there is a strong reason to split it.
- Existing frontend code should continue to load.

If adding `sources.json`:

- Deduplicate sources by stable ID or URL/kind/label tuple.
- Preserve source references on existing entities.
- Do not remove entity-local `sources` arrays unless the frontend is updated safely.

If not adding `sources.json`:

- Add a short note in the contract docs explaining that entity-local sources are the V1 contract and global source reuse is deferred.

### Relationships Contract

Manual relationships from Slice 5 should become contract-visible.

Suggested `relationships.json` rows:

- `relationshipType`
- `sourceTicker`
- `sourceIsin`
- `sourceCompanyId`
- `targetTicker`
- `targetIsin`
- `targetCompanyId`
- `themeId`
- `layerId`
- `confidence`
- `sourceUrl`
- `rationale`
- `lastReviewed`

Also consider adding `relationships` to `catalogue.json` if it keeps the bundle coherent. If you do, update docs and tests.

Do not build the relationship graph UI in this slice.

### Contract Documentation

Add a concise generated-data contract doc, for example:

```text
docs/data-contract.md
```

It should document:

- Data contract version and compatibility policy.
- Every generated file under `site/data`.
- JSON top-level shape and important fields.
- CSV headers and meanings.
- Which files are intended for frontend loading vs human review.
- Manifest fields, generated file metadata, and checksum behavior.
- Backward compatibility expectations for frontend code.
- How to intentionally make a breaking schema change.

Keep the docs practical. The purpose is to help future frontend work and code review, not to create a huge formal spec.

Optional: generate part of this doc from Go constants so header lists do not drift. If hand-written, add tests that catch drift between documented CSV headers and exporter headers.

### Golden Tests

Add tests that catch accidental output shape changes.

Preferred approach:

- Generate sample output into a temp directory.
- Compare selected generated files to golden fixtures under `internal/export/testdata` or `cmd/statos-build/testdata`.
- Include a controlled update path in docs or test comments.

At minimum cover:

- `build_manifest.json` shape, contract version fields, and generated file metadata.
- `catalogue.json` key shape.
- `tickers.csv` header.
- `securities.csv` header.
- `listings.csv` header.
- `relationships.json`.
- `securities.json`.
- `listings.json`.

Because timestamps can be volatile, either:

- run sample mode with deterministic `builtAt`, or
- normalize volatile fields before comparison.

Do not make tests depend on network access.

### CSV Header Stability

Centralize CSV headers where practical so docs/tests/export use the same source.

Current CSV files:

- `tickers.csv`
- `securities.csv`
- `listings.csv`
- `unclassified.csv`
- `identity_issues.csv`
- `enrichment_failures.csv`

Acceptance expectation:

- A test fails if a CSV header changes without an intentional contract update.
- The contract docs list all headers.

### Frontend Compatibility

Keep the existing static frontend working.

Requirements:

- Existing `site/assets/app.js` should continue to load `catalogue.json`.
- Export links should include any new generated files.
- If JSON envelopes are added to files the frontend reads directly, update the frontend with backwards-compatible loading.
- Do not require a build step or frontend framework.

### Out Of Scope

Do not:

- Redesign the frontend UX.
- Add a browser taxonomy editor.
- Add a server or database.
- Add scheduled GitHub Actions refresh.
- Fetch Trading 212 or Yahoo data as part of contract tests.
- Add a full relationship graph UI.
- Replace committed generated files with runtime-only generated data.
- Commit `.env`, `data/raw`, `data/cache`, `.gocache`, or provider cache files.

### Acceptance Criteria

- `make sample` passes.
- `make test` passes.
- `make smoke` passes.
- `refresh --no-fetch` still works.
- `site/data/securities.json` is generated.
- `site/data/listings.json` is generated.
- `site/data/relationships.json` is generated.
- `site/data/build_manifest.json` includes data contract/schema version fields.
- Manifest includes generated file metadata and checksums.
- Checksum behavior is deterministic and documented.
- Every generated `site/data` file is documented in `docs/data-contract.md`.
- CSV headers are documented and tested.
- Golden/snapshot tests catch accidental output shape changes.
- Smoke checks include new generated files.
- Frontend export links include new generated files.
- Existing frontend loading still works.
- `docs/readiness-checklist.md` marks only completed Slice 6 work as done.

### Required Tests

Add focused Go tests for:

- `WriteSiteData` writes all expected JSON and CSV files.
- `securities.json` contents match catalogue securities.
- `listings.json` contents match catalogue listings.
- `relationships.json` contents match manual relationships and are deterministic.
- Manifest contract version fields.
- Manifest generated file metadata and checksum coverage.
- `build_manifest.json` checksum convention.
- CSV header stability for all generated CSVs.
- Golden/snapshot output for deterministic sample data.
- Frontend-compatible JSON shape if any envelope is introduced.

### Suggested Verification Commands

Run:

```sh
make sample
make test
make smoke
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh --no-fetch
make smoke
```

Then verify generated file metadata:

```sh
python3 -m json.tool site/data/build_manifest.json >/tmp/statos-manifest.json
python3 -m json.tool site/data/securities.json >/tmp/statos-securities.json
python3 -m json.tool site/data/listings.json >/tmp/statos-listings.json
python3 -m json.tool site/data/relationships.json >/tmp/statos-relationships.json
```

Verify private local files are still ignored:

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
- Contract/version fields added.
- New generated files added.
- Manifest checksum behavior.
- Contract documentation added.
- Golden/header tests added.
- Commands run and results.
- Any Slice 6 checklist items intentionally left unchecked and why.
- Any risks or follow-up work.

## Reviewer Prompt

Review the Slice 6 Static Data Contract implementation as a strict code reviewer.

Prioritize findings over summary. Look especially for:

- Breaking existing frontend loading of `catalogue.json`.
- Schema/version fields missing or inconsistently named.
- Generated file checksums that are nondeterministic or self-referential.
- New generated files missing from smoke checks, export links, docs, or manifest metadata.
- CSV headers changed without docs/tests.
- Golden tests that are too weak, too brittle, or dependent on wall-clock time.
- Relationships loaded in manual data but not exported or documented.
- `sources.json` added without stable IDs, or omitted without a clear doc note.
- Manifest metadata omitting generated files.
- Contract docs drifting from actual exporter behavior.
- Network access in contract tests.
- `.env`, raw snapshots, cache files, `.gocache`, or temporary golden update files accidentally tracked.

For each finding, include severity, file/line reference, and the concrete failure mode. If there are no findings, state that clearly and list residual risks or test gaps.
