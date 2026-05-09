# Prompt: Slice 1 Repo And Deployment Hardening

Use this prompt for the implementation agent assigned to Slice 1.

## Implementation Prompt

You are working in the Statos repo.

Statos is a static investment research website for Trading 212 Invest / Stocks ISA-compatible tickers. Production must be a static GitHub Pages site with no server. A local Go builder generates committed files under `site/data`. Secrets, raw snapshots, and caches must never be committed.

Your task is to implement **Slice 1: Repo And Deployment Hardening** from `docs/readiness-checklist.md`.

### Goal

Make the project easy to clone, run, verify, and publish safely.

### Important Context

- Current static site lives under `site/`.
- Generated static data lives under `site/data/` and should be committed.
- Raw Trading 212 snapshots live under `data/raw/` and are ignored.
- Enrichment caches live under `data/cache/` and are ignored.
- `.env` is ignored; `.env.example` is committed.
- The current builder command is `go run ./cmd/statos-build`.
- The first version should not add scheduled data refresh. GitHub Actions scheduled refresh is V2.
- If adding a GitHub Pages workflow, it must only publish already-committed static files from `site/`; it must not fetch Trading 212, call Yahoo, or require secrets.
- GitHub Pages branch publishing currently supports repository root `/` or `/docs` as the branch source folder. Because this repo keeps the static site in `/site`, choose and document one of these approaches:
  - Use a GitHub Actions Pages deploy workflow that uploads `site/`.
  - Use a dedicated `gh-pages` branch containing the static site root.
  - Move/duplicate the static publish root to a GitHub Pages-supported branch folder only if you explain why and keep the repo shape coherent.

### Scope

Implement the repo/deployment hardening items that can be completed locally:

- Add GitHub Pages deployment instructions to `README.md`.
- Decide and document the publish path.
- Add `.nojekyll` where appropriate for the chosen publish path.
- Add `CONTRIBUTING.md` with local data safety and generated file expectations.
- Add `LICENSE` matching the non-commercial license already named in the README, or resolve the README/license mismatch clearly.
- Add a `Makefile` or `justfile` with shortcuts for:
  - `test`
  - `sample`
  - `refresh`
  - `preview`
  - `smoke`
- Document required Go version and the local Go build-cache workaround if needed.
- Add a smoke check that verifies expected generated `site/data` files exist.
- Add a pre-commit checklist for secrets, generated files, and unclassified review.
- Update `docs/readiness-checklist.md` checkboxes for completed Slice 1 items only.
- Update `docs/build-checklist.md` if the weekly workflow changes.

### Out Of Scope

Do not:

- Add scheduled refresh.
- Add real Trading 212 credentials.
- Fetch live Trading 212 data.
- Commit anything under `data/raw/`, `data/cache/`, or `.gocache/`.
- Change the data model unless required by the smoke check.
- Rework the frontend design.
- Move the site out of `site/` unless you explicitly document the tradeoff and keep existing commands working.

### Implementation Notes

- Prefer a small shell script under `scripts/` for the smoke check if a `Makefile` target would get too dense.
- The smoke check should fail clearly if any expected file is missing:
  - `site/data/catalogue.json`
  - `site/data/tickers.csv`
  - `site/data/companies.json`
  - `site/data/sectors.json`
  - `site/data/industries.json`
  - `site/data/themes.json`
  - `site/data/supply_chains.json`
  - `site/data/search_index.json`
  - `site/data/unclassified.csv`
  - `site/data/build_manifest.json`
- The smoke check may also validate that `site/data/catalogue.json` and `site/data/build_manifest.json` are parseable JSON using whatever dependency-free tool is already available in the repo environment.
- If you add a GitHub Actions Pages deploy workflow, keep it minimal and deployment-only. It may run the smoke check, but it must not run the refresh command or require API secrets.
- If you choose a workflow, document that GitHub repository settings must use Pages source `GitHub Actions`.
- If you choose a `gh-pages` branch workflow instead, document the manual branch publishing process and make sure `.nojekyll` is included in the published root.

### Acceptance Criteria

- `make test` or equivalent runs `go test ./...` successfully.
- `make sample` or equivalent generates sample `site/data`.
- `make smoke` or equivalent verifies all expected generated files exist.
- `make preview` or equivalent starts a local static server for `site/`.
- README clearly explains local setup, sample generation, refresh, preview, and the chosen GitHub Pages publish path.
- `CONTRIBUTING.md` clearly says what is committed and what must stay local.
- `LICENSE` exists and matches the license stated in README.
- `.nojekyll` exists in the static publish root if needed by the chosen deployment path.
- `docs/readiness-checklist.md` reflects completed Slice 1 work and does not mark live/deployment verification done unless actually verified.
- No secrets or private/generated local artifacts are staged.

### Required Verification

Run and report:

```sh
make sample
make test
make smoke
```

If the project uses `just` instead of `make`, run the equivalent `just` commands.

If the Go build cache fails due sandbox permissions, use a repo-local cache:

```sh
GOCACHE="$PWD/.gocache" go test ./...
```

### Final Response

Summarize:

- Files changed.
- Deployment path chosen and why.
- Commands run and results.
- Any unchecked Slice 1 items and why they remain unchecked.
- Any risks or follow-up work.

## Reviewer Prompt

Review the Slice 1 Repo And Deployment Hardening implementation as a strict code reviewer.

Prioritize findings over summary. Look especially for:

- Secrets, raw snapshots, caches, or local artifacts that could be committed.
- GitHub Pages deployment instructions that do not match the repo layout.
- Accidental scheduled refresh, live API calls, or secret requirements in deployment.
- `Makefile`/`justfile` targets that do something surprising or destructive.
- Smoke checks that can pass while expected generated files are missing.
- README, checklist, and implementation drift.
- License mismatch between `README.md` and `LICENSE`.
- Any change that breaks `go run ./cmd/statos-build sample`, `go test ./...`, or static preview.

For each finding, include severity, file/line reference, and the concrete failure mode. If there are no findings, state that clearly and list residual risks or test gaps.
