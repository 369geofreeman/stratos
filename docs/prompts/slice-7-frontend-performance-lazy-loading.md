# Slice 7 Prompt: Frontend Performance And Lazy Loading

We are building Statos, a static GitHub Pages investment research site backed by a local Go builder.

The site now has a real Trading 212 universe:

- `site/data/catalogue.json` is about 42 MB.
- Live output currently has about 16,985 tickers and 13,001 companies.
- The frontend currently fetches `data/catalogue.json` at startup and renders full arrays directly.
- This causes high memory use and slow page interactions before we continue with richer frontend/review queue work.

Your task is to make the static frontend fast with the full live dataset while preserving the static GitHub Pages architecture.

## Hard Constraints

- Production remains fully static. No server, no database, no API runtime.
- Prefer plain HTML/CSS/JS. Do not introduce a frontend framework unless there is a strong documented reason.
- Keep generated `site/data` files committed and deterministic.
- Do not depend on old Pluto code.
- Do not fetch Trading 212 or Yahoo in GitHub Pages.
- Do not commit `.env`, `data/raw`, `data/cache`, `.gocache`, or secrets.

## Problem To Solve

Do not treat this as only a DOM issue. Rendering fewer rows helps, but the bigger problem is that the app loads the full bundled `catalogue.json` before first render.

Fix both:

1. Startup data loading.
2. Long-list rendering.

The first screen should become responsive quickly on the full live dataset.

## Required Outcome

The app should no longer fetch `data/catalogue.json` during startup. It may remain as a full export/download file, but the UI should boot from smaller files.

Add whatever generated static data slices are needed for fast frontend loading, for example:

- a small bootstrap file with manifest, themes, supply chains, sector/industry summaries, generated file metadata, and counts
- a compact ticker index/list file with only fields needed for table/filter/search rows
- optional paged or sharded ticker detail files
- optional JSON versions of review queues if CSV parsing becomes a bottleneck

Choose the simplest shape that works well with static hosting and the existing Go exporter.

Long lists must not render thousands of DOM nodes at once. Apply a shared lazy rendering pattern to all large lists/tables, especially:

- ticker table
- watchlist table
- unclassified queue
- future review queue surfaces if touched
- any sector/industry drilldown list that can grow large

Acceptable approaches:

- virtual scrolling with stable row heights
- incremental "load more" rendering with a small initial batch
- paged rendering with clear controls

Prefer the least complex approach that keeps the UI fast and predictable.

## Frontend Requirements

- Keep the first screen as the research interface, not a landing page.
- Show a clear loading state while each data slice loads.
- Keep metrics, filters, ticker table, supply-chain map, sector/industry view, watchlist, unclassified queue, modal, local notes/tags/colour labels, import/export working.
- Debounce global search and expensive filter operations.
- Avoid repeated linear `.find` calls in render loops; build maps/indexes once per loaded data slice.
- Lazy-load heavier detail data only when needed, such as when opening a ticker/company modal.
- Preserve local browser data behavior.
- Preserve existing URLs for export links.
- Handle missing optional fields gracefully.
- Keep the interface dense, calm, and research-focused.

## Data/Exporter Requirements

Update the Go exporter and data contract if new generated files are added.

Update at least:

- `internal/export/export.go`
- exporter tests/goldens as needed
- `scripts/smoke.sh`
- `docs/data-contract.md`
- `docs/build-checklist.md`
- `docs/readiness-checklist.md`
- `README.md`
- `site/assets/app.js`
- `site/assets/styles.css` if needed

If adding new files under `site/data`, they must appear in `build_manifest.json.generatedFiles` with checksum and byte metadata.

Do not remove the existing `catalogue.json` export unless the contract docs and frontend migration explicitly support the change. It is still useful as a full export artifact.

## Suggested Implementation Shape

One good implementation would be:

1. Add `site/data/app_bootstrap.json`.
   - manifest
   - themes
   - supplyChains
   - sectors
   - industries
   - generatedFiles
   - high-level counts

2. Add `site/data/tickers_index.json`.
   - compact ticker rows for table/search/filter only
   - no heavy nested objects
   - include ids needed to lazy-load detail: ticker, companyId, securityId, listingId

3. Keep detail data lazy.
   - Use existing `companies.json`, `securities.json`, `listings.json`, and `relationships.json` only when detail/supply-chain views require them.
   - If that still feels heavy, add deterministic detail shards in a small number of files rather than thousands of per-ticker files.

4. Add a small frontend data loader.
   - `loadBootstrap()`
   - `loadTickerIndex()`
   - `ensureDetailData()`
   - `ensureSearchIndex()` only when search needs it

5. Add a shared list renderer.
   - It should render an initial batch, such as 100-250 rows.
   - It should append more rows on scroll/sentinel or via a clear "Load more" control.
   - Re-filtering or sorting resets the visible window.

This shape is a suggestion, not a mandate. If you choose a different approach, document why.

## Acceptance Criteria

- Initial app boot does not fetch `data/catalogue.json`.
- The full live dataset remains usable in the browser.
- Ticker, watchlist, and unclassified views do not render more than a bounded number of row nodes initially.
- Search and filters remain responsive on the live dataset.
- Ticker/company modal still shows identity, classification, security/listing, sources, theme/layer membership, related tickers, local notes, watchlist status, colour labels, and tags.
- Exports view still links to generated files, including `catalogue.json`.
- `make sample` passes.
- `make test` passes.
- `make smoke` passes.
- `python3 scripts/data-status.py --require-live` still works when live data is present.
- `node --check site/assets/app.js` passes.
- `git diff --check` passes.
- `git ls-files .env data/raw data/cache .gocache` prints nothing.

## Review Notes

After implementation, report:

- Which files are loaded during startup.
- Which files are lazy-loaded by each view.
- Approximate generated file sizes for the new frontend slices.
- How many DOM rows/cards are rendered initially for each large list.
- Any remaining performance risks before Slice 7 continues.

Do not sign off if the page still needs to parse the 42 MB `catalogue.json` before becoming usable.
