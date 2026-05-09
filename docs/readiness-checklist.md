# Statos Readiness Checklist

This is the product checklist for getting Statos from the first slice to a V1-ready GitHub Pages research site.

## Readiness Definition

Statos is V1-ready when a fresh checkout can:

- Configure credentials without committing secrets.
- Fetch the current Trading 212 Invest / Stocks ISA instrument universe.
- Preserve raw snapshots and enrichment caches locally.
- Normalize broker tickers into securities, listings, and companies with visible identity uncertainty.
- Apply committed manual taxonomy, overrides, notes metadata, themes, supply-chain layers, and exposures.
- Generate deterministic `site/data` JSON and CSV files.
- Serve a static GitHub Pages-compatible research interface with no production server.
- Let browser-local notes, tags, colours, and watchlists be imported and exported.
- Make unclassified, failed, stale, and low-confidence data obvious.
- Pass the test suite and a manual preview checklist before publishing.

## Current Status

Checked boxes reflect the repo state after the first implementation slice. Live account refresh, real-world enrichment review, and hosted deployment are still intentionally unchecked.

- [x] First static repo slice exists.
- [x] Go builder command exists.
- [x] Trading 212 metadata client exists.
- [x] Sample data can generate `site/data`.
- [x] Manual taxonomy seed files exist.
- [x] Static UI shell exists with search, tables, supply-chain map, detail modal, local notes, watchlist, import, and export.
- [x] Basic tests cover normalization, taxonomy loading, enrichment symbol candidates, and export generation.
- [ ] Live Trading 212 refresh has been exercised with real credentials.
- [ ] Yahoo/enrichment provider has been exercised against real-world misses and ambiguous matches.
- [ ] GitHub Pages deployment is configured and verified.
- [ ] Taxonomy workflow has been used against the full Trading 212 universe.

## Slice 1: Repo And Deployment Hardening

Goal: Make the project easy to clone, run, verify, and publish.

- [ ] Initialize/confirm repo settings and default branch policy.
- [x] Add GitHub Pages deployment instructions to `README.md`.
- [x] Decide GitHub Pages publish path: GitHub Actions uploads committed `site/`.
- [x] Add a `.nojekyll` file if publishing static assets directly from `site`.
- [x] Add a short `CONTRIBUTING.md` for local data safety and generated file expectations.
- [x] Add `LICENSE` matching the intended non-commercial license.
- [x] Add `Makefile` or `justfile` shortcuts for `test`, `sample`, `refresh`, `preview`, and `smoke`.
- [x] Document required Go version and known local cache workaround if needed.
- [x] Add a smoke check that verifies all expected `site/data` files exist.
- [x] Add a pre-commit checklist for secrets, generated files, and unclassified review.

Definition of done:

- [x] A fresh clone can run tests and generate sample data using documented commands.
- [x] The publish path is clear and checked into docs.
- [x] No private local artifacts are tracked.

## Slice 2: Trading 212 Live Ingestion

Goal: Reliably collect the broker universe from the official metadata endpoints.

- [ ] Verify `GET /equity/metadata/instruments` against a real demo or live account.
- [ ] Verify `GET /equity/metadata/exchanges` against a real demo or live account.
- [ ] Confirm auth works with `.env` values only.
- [x] Record rate-limit headers in raw metadata or build diagnostics.
- [x] Add friendly errors for 401, 403, 408, 429, and malformed responses.
- [x] Ensure raw snapshots are timestamped and `latest` aliases are written.
- [x] Add `--no-fetch` mode to rebuild from the latest raw snapshots.
- [x] Add `--input-raw-dir` or equivalent for replaying older snapshots.
- [x] Add tests with fixture JSON matching Trading 212 instrument/exchange responses.
- [x] Add manifest fields for Trading 212 endpoint, account environment, fetch timestamp, and rate-limit observations.

Definition of done:

- [ ] One live refresh produces raw snapshots and deterministic generated output.
- [ ] A replay from raw snapshots produces the same normalized output.
- [x] API failures are visible and do not silently produce partial confidence.

## Slice 3: Identity Resolution And Normalization

Goal: Separate broker tickers, listings, securities, and companies cleanly.

- [x] Add first-pass broker ticker parser for Trading 212-style ticker suffixes.
- [ ] Expand broker ticker parser with observed Trading 212 ticker patterns from the full universe.
- [x] Expand broker ticker parser with common/sample Trading 212-style patterns.
- [x] Confirm ISIN grouping across multiple exchange/currency listings.
- [x] Add manual identity merge/split overrides for cases where ISIN is missing or misleading.
- [x] Add first-pass company ID override support through ticker overrides.
- [x] Expand company ID override support for ADRs, dual listings, funds, trusts, and ETFs.
- [x] Add classification for instrument category: stock, ETF, trust, fund, warrant, crypto, forex, other.
- [x] Add first-pass directionality flags for inverse, short, and leveraged instruments.
- [x] Expand flags for synthetic, hedged, accumulating, distributing, ADR, GDR, and fund-like instruments.
- [x] Add tests for inverse/short/leveraged markers from real tickers.
- [x] Add duplicate and collision reports to the manifest.
- [x] Add CSV output for securities and listings if needed for manual review.
- [x] Add identity confidence fields and reasons.

Definition of done:

- [x] One company can correctly own multiple securities/listings/tickers.
- [x] Ambiguous identities are exported to review queues.
- [x] No instrument is dropped without an explicit manifest count and reason.

## Slice 4: Enrichment Provider Layer

Goal: Enrich when possible, cache everything, and make failures useful.

- [x] Decide the V1 enrichment mode: cache-only by default, optional Yahoo direct lookup.
- [x] Add provider result schema versioning in cache files.
- [x] Add cache TTL/staleness reporting without forcing network calls.
- [x] Add ISIN-first lookup path where provider supports it.
- [x] Add ticker-derived Yahoo symbol candidates with exchange suffix mapping.
- [x] Add ambiguous-match handling with candidate lists instead of first-match trust.
- [x] Add manual enrichment override fields for Yahoo symbol, sector, industry, and country.
- [x] Add manual enrichment override fields for market cap, exchange, and currency.
- [x] Add enrichment failure CSV with ticker, ISIN, attempted symbols, provider, error, and next action.
- [x] Add tests for cache hit, cache miss, provider failure, and ambiguous match.
- [x] Add provider interface documentation so Yahoo can be replaced later.

Definition of done:

- [x] Enrichment can fail for many tickers without blocking catalogue generation.
- [x] Every enrichment value has a source and freshness signal.
- [x] Manual overrides win deterministically over provider data.

## Slice 5: Manual Taxonomy Workflow

Goal: Make taxonomy improvement fast and safe.

- [x] Validate `themes.yml` and `supply_chains.yml` structure with useful error messages.
- [x] Validate exposure rows reference known themes and layers.
- [x] Validate exposure scores and confidence values.
- [x] Validate reviewed dates and source URLs.
- [x] Add manual files for peer groups and relationships.
- [x] Add manual files for sector/industry overrides separate from provider enrichment.
- [x] Add note frontmatter validation.
- [x] Add command to print taxonomy coverage by theme, layer, sector, and industry.
- [x] Add command to generate empty exposure templates from unclassified rows.
- [x] Add docs for how to review `unclassified.csv` and update manual files.

Definition of done:

- [x] Bad manual taxonomy fails fast before writing generated outputs.
- [x] Review workflow turns unclassified rows into suggested manual edits.
- [x] Themes can coexist without hard-coding AI infrastructure assumptions.

## Slice 6: Static Data Contract

Goal: Treat `site/data` as a stable frontend API.

- [ ] Add documented JSON schemas or Go-generated contract docs for each `site/data` file.
- [ ] Add schema/version fields to generated JSON.
- [ ] Add `securities.json` and `listings.json` if the UI needs them independently.
- [ ] Add `relationships.json` for peers, substitutes, upstream suppliers, downstream customers, and related plays.
- [ ] Add `sources.json` if source reuse grows.
- [ ] Ensure CSV headers are stable and documented.
- [x] Ensure generated file ordering is deterministic.
- [ ] Add tests that compare sample output to golden files.
- [ ] Add manifest checksums for generated outputs.
- [x] Add backwards-compatible frontend handling for missing optional fields.

Definition of done:

- [ ] The frontend can be changed against a clear data contract.
- [x] Generated outputs are deterministic enough to review in Git diffs.
- [x] Manifest explains counts, freshness, and failures.
- [ ] Manifest explains file/schema versions.

## Slice 7: Frontend Core Research UX

Goal: Make the static site efficient for real research.

- [x] Add first-pass loading and error state for `catalogue.json`.
- [ ] Add robust loading and error states for every generated data file.
- [ ] Add table sorting for all relevant columns.
- [ ] Add compound filters for sector, industry, type, currency, exchange, directionality, theme, layer, confidence, and unclassified state.
- [ ] Add search result grouping for ticker, company, ISIN, industry, theme, layer, and note text.
- [ ] Add keyboard-friendly modal open/close/focus behavior.
- [ ] Add company-level detail view separate from ticker-level detail.
- [ ] Add listing/security tabs inside the detail modal.
- [x] Add source links and reviewed dates in detail views.
- [x] Add empty states that name the missing data and likely next action.
- [ ] Add visual distinction between provider data, manual overrides, and local browser notes.

Definition of done:

- [ ] A full catalogue remains usable without a framework or server.
- [ ] Detail views answer identity, classification, exposure, relation, and local-note questions.
- [ ] The interface is dense, calm, and readable on desktop and mobile.

## Slice 8: Supply-Chain Map V1

Goal: Generalize the Loniss-style layered map across themes.

- [x] Add first-pass supply-chain theme selector.
- [ ] Make supply-chain theme selector independent of the global filter state.
- [ ] Add layer summaries: mapped count, average exposure score, confidence mix, unclassified count.
- [x] Encode card width from exposure relevance with a fallback.
- [ ] Add optional market-cap-based card sizing.
- [x] Encode colour from confidence or exposure status, not decoration.
- [x] Show badges for exposure score and confidence.
- [ ] Show badges for reviewed date and directionality.
- [ ] Add layer detail panel with rationale and sources.
- [ ] Add card sorting controls: exposure score, market cap, name, confidence, reviewed date.
- [x] Add support for companies without a direct ticker match.
- [ ] Add low-confidence and stale-review indicators.
- [ ] Add print/export-friendly supply-chain view.

Definition of done:

- [x] AI infrastructure works as the first theme.
- [x] Energy, defence, healthcare, fintech, semiconductors, and commodities can use the same model.
- [ ] Visual encodings are documented and consistent.

## Slice 9: Local User Data

Goal: Make browser-local research data reliable and portable.

- [x] Version the local storage schema.
- [ ] Add migration path for future local schema changes.
- [ ] Add local notes per ticker and company, not only ticker.
- [ ] Add local tags with autocomplete from existing tags.
- [x] Add colour labels with fixed semantic names.
- [x] Add first-pass local filters for watchlist, tagged, and colour-labelled tickers.
- [ ] Add local filters for specific tag, specific colour, note text, and reviewed state.
- [ ] Add import validation and friendly failure handling.
- [ ] Add merge-vs-replace choice for imports.
- [ ] Add export metadata with exported timestamp and app data version.
- [ ] Add clear-local-data action behind a confirmation.

Definition of done:

- [x] Local research survives generated data refreshes.
- [x] Local data can be backed up and restored.
- [x] Import errors cannot corrupt existing local data.

## Slice 10: Review Queues

Goal: Turn uncertainty into visible worklists.

- [ ] Expand `unclassified.csv` reasons into structured reason codes.
- [ ] Add UI filters by reason: missing sector, missing industry, missing theme, missing company ID, enrichment failed, identity ambiguous, stale review.
- [ ] Add generated suggested override rows where possible.
- [ ] Add separate queues for identity issues, enrichment issues, taxonomy issues, and stale manual reviews.
- [ ] Add counts by sector/industry/theme gap.
- [ ] Add "copy CSV row" or "copy manual template" helpers in the UI.
- [ ] Add reviewed-date aging thresholds.
- [ ] Add manifest trend fields if previous build manifest is available.
- [ ] Add tests for queue generation.
- [ ] Document a weekly review process.

Definition of done:

- [x] The site makes missing classification work obvious.
- [ ] Manual taxonomy editing is guided by generated queues.
- [ ] Refreshes do not hide regressions in coverage.

## Slice 11: Relationships And Peer Groups

Goal: Model more than sector/industry membership.

- [x] Add first-pass related tickers from same-company and same-industry grouping.
- [ ] Add relationship types: peer, substitute, upstream supplier, downstream customer, related play.
- [ ] Add manual relationship CSV/YAML.
- [ ] Add confidence, source URL, rationale, and reviewed date to relationships.
- [ ] Generate inverse relationships where appropriate.
- [ ] Show relationships in ticker/company detail modal.
- [x] Show first-pass related tickers in ticker detail modal.
- [ ] Add relationship filters and graph/list view.
- [ ] Add peer group pages or panels.
- [ ] Add tests for relationship loading and inverse generation.
- [ ] Add unclassified queue entries for companies with no peers or theme relationships.
- [ ] Keep relationship model independent from any one theme.

Definition of done:

- [x] Detail views can answer a first-pass "what else is like this?"
- [ ] Detail views can answer source-backed "what is connected to this?"
- [ ] Relationships are source-backed and reviewable.
- [ ] Peer groups can cut across sectors and themes.

## Slice 12: Quality, Testing, And Tooling

Goal: Keep the weekly refresh trustworthy.

- [ ] Add Go tests for Trading 212 fixture parsing.
- [ ] Add Go tests for raw replay mode.
- [ ] Add Go tests for identity merge/split overrides.
- [ ] Add Go tests for manual validation failures.
- [ ] Add Go tests for manifest counts.
- [ ] Add golden tests for sample generated files.
- [ ] Add JavaScript unit tests for filtering, local storage, and rendering helpers if tooling remains lightweight.
- [x] Add manual static check command for JS syntax during development.
- [ ] Add repeatable static checks for HTML, CSS, and JS syntax.
- [ ] Add CI for tests and sample generation.
- [ ] Add a manual QA checklist for each release.

Definition of done:

- [ ] Tests cover the highest-risk data transformations.
- [ ] CI catches broken generated data contracts.
- [ ] Manual QA is short enough to run before every publish.

## Slice 13: Performance And Accessibility

Goal: Keep the site fast and usable as the catalogue grows.

- [ ] Measure load time and render time with a full real catalogue.
- [ ] Avoid rendering all heavy rows when filters are narrow.
- [ ] Add pagination or virtualized table rendering if needed.
- [ ] Keep search index compact.
- [x] Add first-pass accessible labels to key interactive controls.
- [ ] Verify keyboard navigation for tabs, tables, modal, and local controls.
- [x] Verify colour labels are not the only signal.
- [x] Add responsive layouts for mobile, tablet, and desktop widths.
- [ ] Verify responsive layouts with screenshots/manual QA.
- [ ] Add high-contrast and reduced-motion considerations if needed.
- [ ] Add browser compatibility notes.

Definition of done:

- [ ] Full catalogue feels responsive in a normal browser.
- [ ] Core workflows can be completed with keyboard navigation.
- [ ] Visual meaning is available without relying only on colour.

## Slice 14: Security, Privacy, And Legal Boundaries

Goal: Keep private data and provider terms in view.

- [x] Confirm `.env`, raw snapshots, and cache files are ignored.
- [ ] Add docs explaining that local notes/watchlists stay in browser storage.
- [ ] Add UI notice for local data export/import ownership.
- [ ] Add checks that generated `site/data` does not include secrets.
- [ ] Confirm raw Trading 212 account/private metadata is not committed accidentally.
- [ ] Keep Trading 212 order/account endpoints out of V1 unless explicitly needed for sanity checks.
- [x] Document Yahoo Finance as unofficial enrichment and replaceable.
- [x] Add provider attribution/source fields where required.
- [x] Avoid exposing API keys in frontend code.
- [ ] Add final pre-publish privacy checklist.

Definition of done:

- [ ] The hosted site contains only intended static research data.
- [x] Secrets are never required by, or exposed to, the browser.
- [x] Provider limitations are documented honestly.

## Slice 15: GitHub Pages V1 Release

Goal: Publish the static research site.

- [ ] Configure GitHub Pages source.
- [ ] Commit generated `site/data` from a real or accepted sample refresh.
- [ ] Verify all export links work on the hosted path.
- [ ] Verify fetch paths work under the GitHub Pages base URL.
- [ ] Verify browser local storage works on the hosted domain.
- [ ] Verify import/export local JSON works in production.
- [ ] Run full test suite.
- [ ] Run sample generation and real refresh if credentials are available locally.
- [ ] Run manual UI QA checklist.
- [ ] Tag or mark the first V1 release.

Definition of done:

- [ ] The public/static URL works without any server.
- [ ] Data refresh remains a local manual workflow.
- [ ] The site is usable for weekly research and taxonomy review.

## V2 Backlog

These are useful but not required for the first fully ready static site.

- [ ] GitHub Actions scheduled refresh.
- [ ] Optional paid enrichment provider.
- [ ] IndexedDB for larger local notes and structured research logs.
- [ ] Historical manifest trend charts.
- [ ] Full-text search library if plain search becomes slow.
- [ ] Graph visualization for relationships.
- [ ] Source extraction from filings/transcripts.
- [ ] More sophisticated exposure dimensions: revenue mix, market share, priced-in status, substitution risk, conviction, and valuation sensitivity.
- [ ] Portfolio import if it can be done without turning Statos into a trading or order-execution system.
- [ ] Multi-user sync, if ever needed, with explicit privacy design.
