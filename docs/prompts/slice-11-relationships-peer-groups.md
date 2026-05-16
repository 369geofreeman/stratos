# Slice 11 Prompt: Relationships And Peer Groups

Use this prompt for the implementation/data agent assigned to Slice 11.

## Goal

Model source-backed relationships beyond sector/industry membership:

- peers
- substitutes
- upstream suppliers
- downstream customers
- related plays

Current state after the 2026-05-16 refresh:

- `relationshipCount=0`
- `data/manual/relationships.csv` exists but only has a header
- first-pass related tickers are inferred from same-company and same-industry grouping
- ticker detail modal can show related tickers and reviewed relationships if they exist

The goal is to make relationships useful and reviewable, not just inferred.

## Scope

Allowed work:

- Add source-backed rows to `data/manual/relationships.csv`.
- Improve relationship export/UI only where necessary to make the added data usable.
- Generate inverse relationships where appropriate, if the data model and tests support it.
- Add tests for relationship loading, validation, export, inverse generation, and frontend contract if changed.
- Update docs and readiness checklist.

Out of scope:

- Do not build a large graph visualization yet unless it is simple and static.
- Do not finish all frontend polish from Slice 7/13.
- Do not add relationships without a source URL and rationale.
- Do not use same-sector as a reviewed peer relationship unless a source or clear product/business overlap supports it.

## Input Files

Review:

- `data/manual/relationships.csv`
- `site/data/relationships.json`
- `site/data/tickers.csv`
- `site/data/companies.json`
- `site/data/exposures` via `catalogue.json` or `app_bootstrap.json`
- `site/data/review_queues.json`
- `docs/data-contract.md`
- `docs/taxonomy-workflow.md`
- `site/assets/app.js`

Output:

- `data/manual/relationships.csv`
- generated `site/data/relationships.json`
- generated `site/data/catalogue.json`

## Relationship CSV Contract

Use the exact header:

```csv
relationship_type,source_ticker,source_isin,source_company_id,target_ticker,target_isin,target_company_id,theme_id,layer_id,confidence,source_url,rationale,last_reviewed
```

Rules:

- `relationship_type`: one of `peer`, `substitute`, `upstream_supplier`, `downstream_customer`, `related_play`.
- Set exactly one source target.
- Set exactly one target target.
- `theme_id` and `layer_id` are optional, but if `layer_id` is present, `theme_id` must be present and the layer must belong to that theme.
- `confidence`, `source_url`, `rationale`, and `last_reviewed` are required.

## Relationship Semantics

- `peer`: similar business/economic exposure, commonly compared.
- `substitute`: one company/product can replace another in a customer decision.
- `upstream_supplier`: target supplies source, or source depends on target as an upstream input. Be clear in rationale.
- `downstream_customer`: target is a customer/end-market for source, or source sells into target. Be clear in rationale.
- `related_play`: connected investment thesis without direct peer/supplier/customer relationship.

If direction is ambiguous, prefer `related_play` or add two explicit rows with clear rationales.

## Priority Passes

Start with mapped themes because relationships are most useful inside supply-chain maps.

1. AI infrastructure:
   - accelerator peers: Nvidia, AMD, Broadcom, Marvell, Intel where relevant
   - foundry/equipment: TSMC, ASML, Applied Materials, Lam Research, KLA, Tokyo Electron if present
   - memory: Micron, SK Hynix/Samsung if present
   - cloud buyers: Microsoft, Alphabet, Amazon, Meta, Oracle
   - power/cooling/grid: Vertiv, Eaton, Schneider, ABB, Siemens, GE Vernova, Constellation, Vistra where present

2. Semiconductors:
   - equipment peers
   - EDA/IP peers
   - foundry/IDM peers
   - analog/power peers

3. Energy:
   - oil majors, E&Ps, services, LNG, uranium/nuclear, grid equipment, renewables

4. Defence:
   - primes, engines, shipbuilding, munitions, sensors/electronics, cyber/C4ISR

5. Healthcare, fintech, commodities:
   - source-backed peer groups and supply-chain links for top mapped names.

## Source Expectations

Use sources such as:

- company annual report or 10-K/20-F business/competition/customer/supplier sections;
- investor presentations;
- official customer/supplier announcements;
- ETF issuer pages for fund peer/substitute relationships;
- reputable exchange/company pages for company identity only.

Do not use unsourced assertions.

## Implementation Notes

If adding inverse generation:

- It must be deterministic.
- It must not create duplicate manual rows.
- It must preserve direction semantics.
- It must be documented in `docs/data-contract.md`.
- It must have tests.

If adding UI:

- Keep it focused: detail modal relationship list and simple filters/panels are enough.
- Do not fetch `catalogue.json` during startup.
- Keep relationship data lazy-loaded as it is now.

## Acceptance Criteria

- `relationshipCount` is greater than zero and meaningful.
- Key mapped companies have reviewed, source-backed peers or related plays.
- Supplier/customer relationships are directionally clear.
- `relationships.json` remains deterministic.
- Detail modal displays reviewed relationships with source/rationale where available.
- Tests cover validation/export/inverse behavior if code changes.
- `make test`, `make smoke`, and `python3 scripts/data-status.py --require-live` pass.

## Report Back

Report:

- Relationship count before and after.
- Rows added by relationship type.
- Themes/layers covered.
- Sources used.
- Any code/UI changes.
- Remaining relationship gaps.
