# Mapping Prompt 5: Frontend Final Polish After Data Mapping

Use this prompt only after identity, sector/industry, theme exposure, and relationship mapping have materially improved.

## Goal

Finish the static research interface so the mapped data is easy to use.

Do not start this before the catalogue has useful taxonomy data. The frontend should be last because UI polish cannot compensate for unmapped research data.

## Current Open Frontend Areas

From `docs/readiness-checklist.md`, remaining frontend/product work includes:

- table sorting for all relevant columns
- compound filters for sector, industry, type, currency, exchange, directionality, theme, layer, confidence, and unclassified state
- grouped search results for ticker, company, ISIN, industry, theme, layer, and note text
- keyboard-friendly modal open/close/focus behavior
- company-level detail view separate from ticker-level detail
- listing/security tabs inside detail modal
- visual distinction between provider data, manual overrides, and local browser notes
- supply-chain layer summaries and sorting
- market-cap card sizing where useful
- low-confidence and stale-review indicators
- print/export-friendly supply-chain view
- local-data import validation, merge/replace, export metadata, and clear-local-data confirmation
- accessibility and responsive QA
- final privacy/legal notices for local browser data

## Hard Constraints

- Plain HTML/CSS/JS unless there is a strong documented reason otherwise.
- No server, database, or production API.
- Do not fetch `catalogue.json` during startup.
- Preserve lazy loading of data slices.
- Keep generated static data files as the frontend API.
- Do not redesign into a marketing site.
- Keep the first screen as the research interface.

## Inputs

Review:

- `site/index.html`
- `site/assets/app.js`
- `site/assets/styles.css`
- `docs/readiness-checklist.md`
- `docs/data-contract.md`
- `site/data/app_bootstrap.json`
- `site/data/tickers_index.json`
- `site/data/review_queues.json`
- `site/data/companies.json`
- `site/data/securities.json`
- `site/data/listings.json`
- `site/data/relationships.json`

## UX Priorities

1. Make filters fast and predictable.
2. Make company/ticker/security identity clear.
3. Make reviewed manual data visibly distinct from provider enrichment and browser-local notes.
4. Make supply-chain maps useful for mapped themes.
5. Make review queues fast enough for weekly taxonomy work.
6. Make local user data safe to import/export.
7. Make keyboard and mobile use acceptable.

## Specific Requirements

### Search And Filtering

- Add grouped search results or grouped display cues for ticker, company, ISIN, industry, theme, layer, and note text.
- Add compound filters without making the toolbar unmanageable.
- Preserve debounced search and bounded row rendering.

### Detail Views

- Add company-level detail view separate from ticker-level detail.
- Add listing/security tabs or sections inside modal.
- Show whether each field came from manual override, provider enrichment, broker metadata, or browser-local user data where the data contract supports it.
- Improve keyboard focus: opening modal moves focus inside; Escape closes; close returns focus to trigger.

### Supply Chain

- Add layer summaries:
  - mapped count
  - average exposure score
  - confidence mix
  - stale/low-confidence indicators
- Add sorting controls:
  - exposure score
  - market cap
  - name
  - confidence
  - reviewed date
- Keep visual encodings consistent and documented.

### Local User Data

- Add import validation.
- Add merge vs replace import choice.
- Add export metadata with timestamp and schema version.
- Add clear-local-data action behind confirmation.
- Add note/tag filters where practical.

### Accessibility/Responsive

- Verify keyboard navigation for filters, tabs, tables, modal, local controls.
- Verify mobile and desktop layouts manually or with screenshots.
- Ensure colour is not the only signal.
- Add reduced-motion/high-contrast handling if needed.

## Acceptance Criteria

- Startup still loads `app_bootstrap.json` and not `catalogue.json`.
- Full live dataset remains responsive.
- Search/filter/detail/review/supply-chain workflows are usable with mapped data.
- Ticker, company, security, listing, relationship, source, review, and local data are understandable.
- `node --check site/assets/app.js` passes.
- `make test` passes.
- `make smoke` passes.
- `python3 scripts/data-status.py --require-live` passes.
- Manual QA checklist is updated and run.

## Report Back

Report:

- Frontend files changed.
- Data files loaded at startup and per view.
- UX workflows improved.
- Accessibility checks performed.
- Performance risks remaining.
- Readiness checklist items completed or still open.
