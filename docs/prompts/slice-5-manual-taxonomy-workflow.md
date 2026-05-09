# Prompt: Slice 5 Manual Taxonomy Workflow

Use this prompt for the implementation agent assigned to Slice 5.

## Implementation Prompt

You are working in the Statos repo.

Statos is a static investment research website for Trading 212 Invest / Stocks ISA-compatible tickers. The local Go builder fetches or replays Trading 212 metadata, normalizes broker instruments into durable research entities, applies committed manual taxonomy, and exports static files under `site/data`.

Your task is to implement **Slice 5: Manual Taxonomy Workflow** from `docs/readiness-checklist.md`.

### Goal

Make taxonomy improvement fast, safe, and repeatable. Bad manual taxonomy should fail before generated files are written. Weekly review should turn unclassified rows and coverage gaps into clear manual edit tasks.

### Current Context

The repo already has:

- Manual files under `data/manual`.
- Basic theme and supply-chain YAML loading.
- CSV loading for company overrides, ticker overrides, identity overrides, and exposures.
- Notes loaded from `data/manual/notes/*.md`.
- Exposure validation that references known themes and layers.
- Identity override validation.
- Generated review files:
  - `site/data/unclassified.csv`
  - `site/data/identity_issues.csv`
  - `site/data/enrichment_failures.csv`
  - `site/data/securities.csv`
  - `site/data/listings.csv`
- Manifest counts for identity, enrichment, and unclassified rows.

Keep this slice focused on manual taxonomy workflow. Do not solve the whole relationship graph, frontend review UI, or static data contract yet.

### Scope

Implement the remaining Slice 5 items that can be completed locally:

- Validate `themes.yml` and `supply_chains.yml` structure with useful, row/line-specific errors.
- Strengthen exposure validation:
  - scores
  - confidence values
  - required target fields
  - reviewed dates
  - source URLs
- Validate reviewed dates and source URLs across manual CSV files where those fields exist.
- Add note frontmatter validation.
- Add a committed manual file for peer groups and relationships.
- Add committed manual classification override support separate from provider enrichment.
- Add a command to print taxonomy coverage by:
  - theme
  - supply-chain layer
  - sector
  - industry
- Add a command to generate empty exposure templates from unclassified rows.
- Add docs for reviewing `unclassified.csv` and updating manual taxonomy files.
- Update `README.md`, `docs/build-checklist.md`, and `docs/readiness-checklist.md`.

### Manual File Design

Keep committed manual files easy to edit in a spreadsheet or text editor.

#### Classification Overrides

Add a dedicated file for manual classification data, separate from provider enrichment:

```text
data/manual/classification_overrides.csv
```

Suggested columns:

```csv
target_type,ticker,isin,company_id,sector,industry,country,source_url,last_reviewed
```

Guidelines:

- `target_type` should be one of `ticker`, `isin`, or `company`.
- Exactly one matching target field should be populated for the chosen `target_type`.
- `sector`, `industry`, and `country` are manual classification fields.
- Manual classification overrides must win over provider enrichment.
- Existing `ticker_overrides.csv` and `company_overrides.csv` classification columns should remain backward compatible unless you migrate existing data cleanly.
- If both old and new override files specify conflicting values, use a deterministic precedence rule and document it.

Suggested precedence:

1. `classification_overrides.csv`
2. existing ticker/company manual overrides
3. provider enrichment
4. raw broker metadata fallback

#### Relationships

Add a committed manual relationships file:

```text
data/manual/relationships.csv
```

Suggested columns:

```csv
relationship_type,source_ticker,source_isin,source_company_id,target_ticker,target_isin,target_company_id,theme_id,layer_id,confidence,source_url,rationale,last_reviewed
```

Allowed `relationship_type` values:

- `peer`
- `substitute`
- `upstream_supplier`
- `downstream_customer`
- `related_play`

Guidelines:

- Require one source target and one target target.
- Validate `theme_id` and `layer_id` if present.
- Validate confidence, source URL, and reviewed date.
- Load and validate this file now, but do not build the full relationship graph UI in this slice.
- Do not add a separate `relationships.json` unless it is the simplest way to keep the generated data coherent. Dedicated static data contract work belongs to Slice 6.

### Validation Rules

Prefer strict, deterministic validation with clear errors. Bad manual files must fail before `site/data` is written.

#### General CSV Validation

For all manual CSV loaders touched in this slice:

- Reject unknown columns.
- Reject duplicate headers.
- Include file path and row number in validation errors.
- Treat blank rows as ignorable.
- Trim whitespace.
- Keep generated output deterministic.

#### Dates

Validate `last_reviewed` values where present.

- Format: `YYYY-MM-DD`.
- Empty is allowed only where the field is genuinely optional.
- Exposure rows and manual relationship rows should generally require `last_reviewed`.

#### URLs

Validate `source_url` values where present.

- Allow blank only where the field is optional.
- If present, require an absolute `http` or `https` URL.
- Include file path and row number in errors.

#### Confidence Values

Use a shared validation helper where practical.

Suggested allowed confidence values:

- `manual_high`
- `manual_medium`
- `manual_low`
- `rule_high`
- `rule_medium`
- `rule_low`

Exposure and relationship rows should not accept arbitrary confidence strings.

#### Exposure Rows

Validate:

- `theme_id` is known.
- `layer_id` belongs to the theme.
- At least one of `ticker`, `isin`, or `company_id` is present.
- `exposure_score` parses and is between `0` and `5`, inclusive.
- `confidence` is allowed.
- `source_url` is present and valid for manually reviewed rows.
- `last_reviewed` is present and valid.

Do not silently parse malformed scores as zero.

#### Themes

Validate:

- `themes:` root exists.
- Theme IDs are non-empty, unique slugs.
- Theme names are non-empty.
- Unknown fields fail clearly.
- `color`, if present, is `#RRGGBB`.

#### Supply Chains

Validate:

- `supply_chains:` root exists.
- Each chain has a known `theme_id`.
- Each chain has a non-empty name.
- Each chain has at least one layer.
- Layer IDs are non-empty and unique within a theme.
- Layer names are non-empty.
- Layer order parses as an integer.
- Layer order values are unique within a chain.
- Unknown fields or malformed indentation fail clearly.

It is fine to keep the current small YAML subset parser if it is made stricter. Do not add a YAML dependency unless the tradeoff is documented and tests cover it.

#### Notes

Validate note frontmatter in `data/manual/notes/*.md`.

Suggested frontmatter:

```yaml
---
target_type: ticker
target_id: NVDA_US_EQ
title: NVIDIA AI infrastructure note
tags: ai, accelerators
---
```

Validation expectations:

- Unknown frontmatter keys fail clearly.
- `target_type`, `target_id`, and `title` are required when frontmatter is present.
- `target_type` should be one of:
  - `ticker`
  - `company`
  - `security`
  - `sector`
  - `industry`
  - `theme`
  - `layer`
- Tags should parse deterministically.
- Empty note body should fail unless there is a clear reason to allow it.

### Review Commands

Add commands that help the weekly manual workflow without requiring network access.

Choose the simplest command shape that fits the existing CLI. Suggested shape:

```sh
go run ./cmd/statos-build taxonomy coverage
go run ./cmd/statos-build taxonomy exposure-template
```

If nested commands add too much complexity, use flat names such as:

```sh
go run ./cmd/statos-build taxonomy-coverage
go run ./cmd/statos-build exposure-template
```

Document whichever shape you implement.

#### Coverage Command

The coverage command should be deterministic and should not call Trading 212 or Yahoo.

Preferred input:

- Read `site/data/catalogue.json` by default.
- Allow `--catalogue` to point to another generated catalogue file.

Output can be plain text, CSV, or TSV. Keep it easy to paste into notes.

At minimum include:

- theme ID/name
- exposed ticker/company count per theme
- covered layers vs total layers per theme
- layer exposure count
- confidence mix by layer
- sector counts
- industry counts
- unclassified count

#### Exposure Template Command

Generate empty exposure rows from `site/data/unclassified.csv` and/or `site/data/catalogue.json`.

Suggested command:

```sh
go run ./cmd/statos-build taxonomy exposure-template --out /tmp/statos-exposure-template.csv
```

Behavior:

- Default to stdout if `--out` is omitted.
- Use the exact `data/manual/exposures.csv` header.
- Pre-fill known fields:
  - `ticker`
  - `isin`
  - `company_id`
- Leave manual decision fields blank:
  - `theme_id`
  - `layer_id`
  - `exposure_score`
  - `confidence`
  - `source_url`
  - `rationale`
  - `last_reviewed`
- Do not append directly to committed manual files unless explicitly requested by a flag.
- Sort rows deterministically by ticker.

### Documentation

Add a concise workflow doc, for example:

```text
docs/taxonomy-workflow.md
```

It should explain:

- What each manual file is for.
- How to review `site/data/unclassified.csv`.
- How to use the coverage command.
- How to use the exposure template command.
- How to add a new theme/layer.
- How to add or update exposure rows.
- How to add classification overrides.
- How to add relationship rows.
- What validation errors mean and how to fix them.

Update `README.md` and `docs/build-checklist.md` to link to this workflow.

### Out Of Scope

Do not:

- Add a browser-based taxonomy editor.
- Add a server or database.
- Add scheduled GitHub Actions refresh.
- Add Trading 212/Yahoo network calls to taxonomy review commands.
- Add full relationship graph visualization.
- Add peer-group UI.
- Add `relationships.json` or static JSON schema work unless unavoidable. That is Slice 6.
- Redesign the frontend.
- Commit `.env`, `data/raw`, `data/cache`, `.gocache`, or provider cache files.

### Acceptance Criteria

- `make sample` passes.
- `make test` passes.
- `make smoke` passes.
- `refresh --no-fetch` still works.
- Invalid manual taxonomy fails before writing generated `site/data`.
- Theme and supply-chain validation catches malformed structure with useful file/line errors.
- Exposure validation catches malformed scores, invalid confidence values, bad dates, bad URLs, missing targets, and unknown theme/layer references.
- Note frontmatter validation catches missing required fields and unknown keys.
- `classification_overrides.csv` exists, loads, validates, and applies deterministically.
- `relationships.csv` exists, loads, and validates.
- Coverage command works without network access.
- Exposure template command works without network access and produces deterministic CSV.
- README/build docs explain the manual taxonomy review workflow.
- `docs/readiness-checklist.md` marks only completed Slice 5 work as done.

### Required Tests

Add focused Go tests for:

- Valid theme and supply-chain loading.
- Duplicate theme IDs.
- Missing theme names.
- Invalid theme colors.
- Unknown YAML fields.
- Missing/duplicate layer IDs.
- Malformed layer order.
- Exposure score parse failures.
- Exposure score range validation.
- Exposure confidence validation.
- Exposure missing target validation.
- Exposure date and URL validation.
- Classification override loading, validation, and precedence over provider enrichment.
- Relationship loading and validation.
- Note frontmatter validation.
- Coverage command output on sample data.
- Exposure template command output on sample data.
- Manual validation failure does not write generated outputs.

### Suggested Verification Commands

Run:

```sh
make sample
make test
make smoke
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh --no-fetch
make smoke
```

Run the new workflow commands, adjusting names to match the implementation:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy coverage
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy exposure-template > /tmp/statos-exposure-template.csv
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
- Manual file additions.
- Validation rules added.
- Workflow commands added.
- Documentation added.
- Commands run and results.
- Any Slice 5 checklist items intentionally left unchecked and why.
- Any risks or follow-up work.

## Reviewer Prompt

Review the Slice 5 Manual Taxonomy Workflow implementation as a strict code reviewer.

Prioritize findings over summary. Look especially for:

- Bad manual taxonomy still writing partial or stale generated outputs.
- Malformed exposure scores silently parsed as zero.
- Unknown confidence values accepted.
- Invalid dates or URLs accepted without useful errors.
- Theme/layer validation missing duplicates or malformed structure.
- Manual classification overrides not winning over provider data.
- Backward compatibility broken for existing manual override files.
- Relationship rows loaded but not validated.
- Notes with malformed frontmatter silently accepted.
- Review commands making network calls or mutating committed manual files unexpectedly.
- Coverage/template outputs being nondeterministic.
- New manual files missing from docs.
- New generated files missing from smoke checks if any are added.
- `.env`, raw snapshots, cache files, `.gocache`, or local templates accidentally tracked.

For each finding, include severity, file/line reference, and the concrete failure mode. If there are no findings, state that clearly and list residual risks or test gaps.
