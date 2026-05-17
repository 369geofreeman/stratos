# Mapping Prompt 7: Part 2 Broaden Existing Pipelines

Use this prompt for the agent assigned to broadening existing pipeline coverage after the Part 1 biggest-gap pipeline pass.

## Goal

Broaden the existing pipeline maps that are still thin:

- `healthcare`: expand into pharma, biotech, life-science tools, diagnostics, devices, providers, distributors, and healthcare IT.
- `energy`: expand oil and gas, midstream, refining, LNG, uranium/nuclear, renewables, utilities, storage, and grid services.
- `defence`: expand aerospace suppliers, engines, naval, munitions, sensors/electronics, cyber/C4ISR, space, and autonomy/drones.
- `commodities`: expand gold, copper, lithium, uranium, steel, chemicals, agriculture, diversified miners, and commodity processors.
- `fintech`: decide whether to keep it separate from `financial_system` or fold part of its scope into `financial_system` as specialist layers.

The site already has useful broad category and pipeline navigation. This pass should make the existing pipelines materially useful for research, not just present.

## Current Context

Statos is a static GitHub Pages investment research site for Trading 212 Invest / Stocks ISA-compatible tickers.

After Part 1 and the classification cleanup on 2026-05-16, generated live/raw-replay data is approximately:

- 17,050 Trading 212 tickers
- 13,215 companies
- 12 active pipelines in generated data
- 7,473 unclassified stock rows
- 3,476 missing sector rows
- 3,476 missing industry rows
- 3,997 missing theme exposure rows
- 9,348 enrichment failures
- 0 identity issues

Existing Part 2 pipeline coverage is still thin:

- `healthcare`: 14 tickers, 7 companies, 4/4 layers.
- `energy`: 26 tickers, 13 companies, 7/7 layers.
- `defence`: 16 tickers, 8 companies, 4/4 layers.
- `commodities`: 20 tickers, 10 companies, 5/5 layers.
- `fintech`: 28 tickers, 15 companies, 5/5 layers.

Part 1 added these newer pipelines and they must not be broken:

- `financial_system`
- `enterprise_software`
- `industrial_automation`
- `consumer_brands`
- `transport_logistics`

## Hard Constraints

- Production remains fully static.
- Do not add a production server, database, or runtime API.
- Do not depend on the old Pluto repo.
- Do not commit `.env`, `data/raw`, `data/cache`, `.gocache`, `.venv`, provider cache, or secrets.
- Treat Trading 212 instrument metadata as the source universe.
- Treat Yahoo/yfinance as replaceable enrichment, not source of truth.
- Keep generated `site/data` committed when this task intentionally changes generated outputs.
- Keep source-backed manual taxonomy separate from raw snapshots and provider caches.
- Do not invent classifications, exposures, or relationships without a defensible source.

## Required Output Files

Manual taxonomy:

- `data/manual/themes.yml` only if a description/color correction is needed.
- `data/manual/supply_chains.yml`
- `data/manual/exposures.csv`
- `data/manual/classification_overrides.csv` if a newly exposed company still lacks sector/industry.
- optionally `data/manual/notes/*.md` if a pipeline needs explanatory context.

Generated static data:

- `site/data/supply_chains.json`
- `site/data/explorer_index.json`
- `site/data/review_summary.json`
- `site/data/review_queues.json`
- other generated `site/data` files touched by raw replay.

## Operating Rule

Each mapping pass must:

1. Measure current queue and pipeline coverage before changing anything.
2. Add focused manual taxonomy changes.
3. Rebuild from the latest raw snapshot without unnecessary network calls.
4. Measure queue and pipeline coverage after the change.
5. Report what improved and what remains.

Use raw replay for mapping work:

```sh
STATOS_ENRICHMENT_PROVIDER=cache GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh --no-fetch
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy coverage
make test
make smoke
python3 scripts/data-status.py --require-live
```

Do not run `make sample` at the end. The final generated `site/data/build_manifest.json` should remain live or live raw-replay derived.

## Quality Bar

- Manual rows must include `source_url` and `last_reviewed` where the file requires them.
- Use `manual_high` only for direct, source-backed, reviewed classifications/exposures.
- Use `manual_medium` when the source supports the direction but the exact exposure is estimated or the business is diversified.
- Use `rule_low` only for broad rule-based or name-derived mapping, and avoid it for this prompt unless a code rule is explicitly added and tested.
- Keep rationales short and specific.
- Do not use a theme/layer as a catch-all just to reduce queue counts.
- Prefer fewer high-quality mappings over large unsupported guesses.
- Prefer `company_id` for operating-company exposures so all listings attach to the same mapped company.
- Prefer `ticker` or `isin` only when the exposure is security-specific.
- Do not rename or remove existing layer IDs unless there is a migration reason and all existing exposure rows are updated deliberately.

## Work Order

1. Baseline measurement.
2. Review existing layer coverage for `healthcare`, `energy`, `defence`, `commodities`, and `fintech`.
3. Decide whether fintech remains standalone or whether some scope should move into `financial_system`.
4. Add missing layers where needed, without breaking existing layers.
5. Seed reviewed exposure rows in batches, starting with high-market-cap and high-review-queue clusters.
6. Add sector/industry classification overrides for newly exposed companies that still lack classification.
7. Rebuild and measure.
8. Add a second exposure batch if sources are clear.
9. Run verification.
10. Report coverage, fintech decision, and remaining gaps.

## Fintech Decision

Make this decision explicitly before editing `data/manual/exposures.csv`.

Recommended default: keep `fintech` as a standalone specialist pipeline for now.

Reasoning:

- `financial_system` maps the broad incumbent financial system: banks, insurers, asset managers, exchanges, brokers, credit/rating/data.
- `fintech` should map digital-native financial technology and software layers: payments, banking software, digital wallets, lending platforms, crypto rails, and embedded finance.
- Some companies can be cross-mapped when sources support both exposures.

Do not delete or fold `fintech` into `financial_system` during this prompt unless you also provide a migration plan, update existing rows, rebuild, and show that Explorer behavior is better.

## Suggested Layer Work

Existing layer IDs should remain stable. Add new layers only where they make Explorer more useful.

### healthcare

Current layers:

- `biopharma`
- `life_science_tools`
- `medical_devices`
- `care_delivery`

Suggested additions:

- `large_pharma`: diversified pharmaceutical companies.
- `biotech`: biotech drug developers and platform companies.
- `diagnostics_tools`: diagnostics, testing, instruments, and lab workflows.
- `healthcare_distributors`: drug distributors, medical distributors, pharmacies, and supply-chain services.
- `managed_care`: health insurers and managed-care platforms.
- `healthcare_it`: healthcare software, data, revenue-cycle, and clinical workflow platforms.

Use `biopharma` for broad drug-development exposure if you decide not to split into `large_pharma` and `biotech`. If splitting, keep existing rows valid and add new rows consistently.

Priority candidates and clusters:

- Drug Manufacturers - General
- Drug Manufacturers - Specialty & Generic
- Biotechnology
- Diagnostics & Research
- Medical Devices
- Medical Instruments & Supplies
- Medical Distribution
- Healthcare Plans
- Health Information Services
- Medical Care Facilities

### energy

Current layers:

- `integrated_oil_gas`
- `upstream`
- `oilfield_services`
- `power_generation`
- `grid_equipment`
- `renewables`
- `energy_storage`

Suggested additions:

- `midstream_lng`: pipelines, storage, LNG, terminals, and gas infrastructure.
- `refining_marketing`: refiners, fuels marketing, downstream operations.
- `uranium_nuclear`: uranium miners, nuclear fuel cycle, nuclear power specialists.
- `regulated_utilities`: regulated electric, gas, water, and multi-utilities.
- `renewable_developers`: renewable generation owners and developers.
- `solar_wind_equipment`: solar, wind, inverters, and renewable equipment.

Priority candidates and clusters:

- Oil & Gas Integrated
- Oil & Gas E&P
- Oil & Gas Midstream
- Oil & Gas Refining & Marketing
- Oil & Gas Equipment & Services
- Utilities - Regulated Electric
- Utilities - Diversified
- Utilities - Renewable
- Independent Power Producers
- Uranium
- Solar

### defence

Current layers:

- `primes`
- `aerospace_components`
- `defence_software`
- `cyber`

Suggested additions:

- `aerospace_engines`: engines, propulsion, aviation systems, aircraft systems.
- `sensors_electronics`: radars, sensors, avionics, electronics, communications.
- `missiles_munitions`: missiles, munitions, tactical systems, explosives.
- `shipbuilding_naval`: naval shipbuilding, marine systems, submarines.
- `space_defence`: launch, satellites, space systems, missile warning.
- `autonomy_drones`: autonomous systems, drones, robotics, unmanned platforms.
- `defence_services`: maintenance, logistics, training, and government services.

Priority candidates and clusters:

- Aerospace & Defense
- Communication Equipment
- Electronic Components
- Security & Protection Services
- Scientific & Technical Instruments
- Software - Infrastructure for defence/cyber where source-backed
- Specialty Industrial Machinery where defence exposure is direct

### commodities

Current layers:

- `diversified_mining`
- `copper`
- `lithium`
- `precious_metals`
- `agriculture_inputs`

Suggested additions:

- `uranium`: uranium miners and nuclear fuel-cycle material exposure.
- `steel_iron_ore`: steelmakers, iron ore, metallurgical coal where direct.
- `specialty_chemicals`: specialty chemical producers linked to industrial/agriculture/material supply.
- `aluminum`: aluminum miners, smelters, and processors.
- `fertilizer_agriculture`: fertilizer, seeds, crop nutrients, agricultural inputs.
- `commodity_processors`: commodity merchants, processors, and royalty/streaming companies where appropriate.

Priority candidates and clusters:

- Gold
- Other Precious Metals & Mining
- Copper
- Other Industrial Metals & Mining
- Uranium
- Steel
- Aluminum
- Specialty Chemicals
- Chemicals
- Agricultural Inputs
- Farm Products

### fintech

Current layers:

- `payments`
- `capital_markets`
- `consumer_finance`
- `crypto_rails`
- `banking_software`

Suggested additions if keeping standalone:

- `merchant_acquiring`: merchant acquirers, processors, POS, commerce payments.
- `digital_wallets`: wallets, super-app payments, digital account ecosystems.
- `embedded_finance`: embedded lending, payroll finance, commerce finance, BNPL where direct.
- `risk_identity_data`: fraud, identity, risk, compliance, and financial data tools.
- `wealth_brokerage_apps`: digital brokers and investing apps.
- `insurtech`: insurance technology platforms, if source-backed.

Priority candidates and clusters:

- Credit Services
- Software - Application
- Software - Infrastructure
- Information Technology Services
- Capital Markets when digital brokerage or trading technology is direct
- Financial Data & Stock Exchanges only when product is fintech infrastructure rather than broad financial system exposure

## Candidate Discovery

Use generated data to find source-backed candidates. Examples:

```sh
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy coverage
```

```sh
jq -r '.[] | select((.themeIds|not) and (.marketCap // 0) > 25000000000 and (.sector != "Funds")) | [.id,.name,.primaryTicker,.sector,.industry,((.marketCap//0)/1000000000|floor)] | @tsv' site/data/companies.json
```

```sh
jq -r '[.[] | select((.themeIds|not) and (.marketCap // 0) > 25000000000 and (.sector != "Funds")) | {sector, industry, marketCap: (.marketCap // 0)}] | group_by(.sector + "|" + .industry) | map({sector: .[0].sector, industry: .[0].industry, count: length, marketCap: (map(.marketCap) | add)}) | sort_by(-.marketCap) | .[:40][] | [.count, (.marketCap/1000000000|floor), .sector, .industry] | @tsv' site/data/companies.json
```

Find newly exposed companies that still lack classification:

```sh
python3 - <<'PY'
import csv, json
companies = {c["id"]: c for c in json.load(open("site/data/companies.json"))}
themes = {"healthcare", "energy", "defence", "commodities", "fintech"}
for i, row in enumerate(csv.DictReader(open("data/manual/exposures.csv", newline="")), start=2):
    if row["theme_id"] in themes and row["company_id"]:
        company = companies.get(row["company_id"], {})
        if not company.get("sector") or not company.get("industry"):
            print(i, row["theme_id"], row["layer_id"], row["company_id"], company.get("primaryTicker"), company.get("sector"), company.get("industry"))
PY
```

## Exposure Row Contract

Use the exact header:

```csv
theme_id,layer_id,ticker,isin,company_id,exposure_score,confidence,source_url,rationale,last_reviewed
```

Guidance:

- `theme_id`: one of the theme IDs in `data/manual/themes.yml`.
- `layer_id`: layer in `data/manual/supply_chains.yml` for that theme.
- `ticker`, `isin`, or `company_id`: provide the most stable target available.
- `exposure_score`: 0-5.
- `confidence`: `manual_high`, `manual_medium`, or `rule_low`.
- `source_url`: investor relations, annual report, company product/business page, issuer page, or other reputable source.
- `rationale`: one short sentence specific to the layer.
- `last_reviewed`: current review date in `YYYY-MM-DD`.

Exposure score guide:

- `5`: direct pure-play or essential leader in the layer.
- `4`: strong direct exposure, major segment, or major supplier/customer role.
- `3`: meaningful exposure but diversified or less direct.
- `2`: indirect exposure or secondary business line.
- `1`: watchlist/optionality exposure.
- `0`: explicit non-exposure only if useful for future exclusion logic.

## Classification Cleanup

When a newly mapped exposure target still lacks sector or industry after rebuild:

- Add a company-level row to `data/manual/classification_overrides.csv` if a source supports it.
- Use official investor relations, annual report, or company profile sources.
- Keep country blank only if the source does not clearly support it and existing generated data has no reliable country.
- Rebuild again and confirm the targeted blank-sector/industry check returns no rows.

Do not let this pass create a new tail of exposed-but-unclassified companies.

## Sourcing Expectations

Good sources:

- official investor relations pages;
- annual reports, 10-Ks, 20-Fs, registration statements;
- official business segment/product pages;
- official exchange/security pages for identity only;
- official issuer pages for funds or ETPs if any are mapped.

Avoid:

- unsupported web snippets;
- generic finance profile pages as the only basis for exposure when an official source is available;
- stale or unrelated product pages;
- mapping by name alone when the business line is not clear.

## Initial Coverage Targets

These are targets, not permission to guess:

- Add or refine layers only where they improve filtering.
- Seed every new layer with at least 2 reviewed exposure rows where possible.
- Add at least 25 reviewed exposure rows to each of `healthcare`, `energy`, `defence`, and `commodities` if sources are clear.
- Add at least 15 reviewed exposure rows to `fintech` if it remains standalone.
- Prefer a total Part 2 seed batch of 120-200 exposure rows if sourcing can be done cleanly.
- Reduce `missing_theme_exposure` materially.
- Keep existing Part 1 pipelines intact.

If a layer cannot be sourced in this pass, keep it only if it is structurally important and call it out in the report as a thin layer.

## Rebuild And Verification

After manual changes:

```sh
STATOS_ENRICHMENT_PROVIDER=cache GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh --no-fetch
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy coverage
make test
make smoke
python3 scripts/data-status.py --require-live
find site/data -type f -size +50M -print
git diff --check
```

If `find site/data -type f -size +50M -print` returns anything, report it and do not ignore it.

## Acceptance Criteria

- `healthcare`, `energy`, `defence`, `commodities`, and `fintech` have broader, source-backed exposure coverage.
- Fintech has a documented keep-vs-fold decision.
- Existing layer IDs are not broken.
- New layers, if added, are present in `site/data/supply_chains.json`.
- `site/data/explorer_index.json` includes clickable groups for updated themes and layers.
- `missing_theme_exposure` drops.
- Newly exposed operating companies do not remain blank for sector/industry without explanation.
- Existing Part 1 pipelines remain present and unchanged unless an explicit cross-map is source-backed.
- No generated `site/data` file exceeds GitHub's 50 MB recommended limit.
- `make test`, `make smoke`, and `python3 scripts/data-status.py --require-live` pass.

## Report Back

Report:

- Baseline counts before the pass.
- Counts after the pass.
- Fintech decision and rationale.
- Layers added or refined.
- Exposure rows added by theme and layer.
- Classification overrides added, if any.
- Representative sources used.
- Thin layers that still need more rows.
- Any duplicate identity problems discovered while mapping.
- Whether all verification commands passed.

Also include a recommended Part 2 follow-up list if the pass could not source every high-value company cleanly.
