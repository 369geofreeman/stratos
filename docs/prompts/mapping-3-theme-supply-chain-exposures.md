# Mapping Prompt 3: Theme And Supply-Chain Exposure Mapping

Use this prompt for the agent assigned to theme and supply-chain mapping.

## Goal

Turn the catalogue from a ticker list into a research map by adding source-backed exposures to themes and supply-chain layers.

Current queue after the 2026-05-16 refresh:

- 17,029 missing theme exposure rows
- 11 existing manual exposure rows
- Current themes: `ai_infrastructure`, `energy`, `defence`, `healthcare`, `fintech`, `semiconductors`, `commodities`
- Only `ai_infrastructure` currently has supply-chain layers defined

This is the largest mapping gap.

## Hard Rule

Do not attempt to fill all 17,029 rows with unsupported guesses. Build high-quality coverage in layers:

1. AI infrastructure operating companies and critical suppliers.
2. Semiconductors theme and semiconductor-specific layers.
3. Energy theme and energy-specific layers.
4. Defence theme.
5. Healthcare theme.
6. Fintech theme.
7. Commodities theme and commodity funds/ETCs.
8. Broad funds/ETFs where exposure can be mapped conservatively by fund objective.

## Inputs

Review:

- `site/data/suggested_exposures.csv`
- `site/data/taxonomy_issues.csv`
- `site/data/tickers.csv`
- `site/data/companies.json`
- `site/data/sectors.json`
- `site/data/industries.json`
- `data/manual/themes.yml`
- `data/manual/supply_chains.yml`
- `data/manual/exposures.csv`
- `docs/taxonomy-workflow.md`

Output files:

- `data/manual/supply_chains.yml`
- `data/manual/exposures.csv`
- optionally `data/manual/notes/*.md` for thesis context

## Exposure Row Contract

Use the exact header:

```csv
theme_id,layer_id,ticker,isin,company_id,exposure_score,confidence,source_url,rationale,last_reviewed
```

Guidance:

- `theme_id`: one of the theme IDs in `data/manual/themes.yml`.
- `layer_id`: layer in `data/manual/supply_chains.yml` for that theme.
- `ticker`, `isin`, or `company_id`: provide the most stable target available. Prefer `company_id` for operating companies where multiple listings exist. Prefer `isin` or ticker for funds/ETPs where the security itself is the exposure.
- `exposure_score`: 0-5.
- `confidence`: `manual_high`, `manual_medium`, or `rule_low`.
- `source_url`: investor relations, annual report, product page, issuer page, or other reputable source.
- `rationale`: one sentence, specific to the layer.
- `last_reviewed`: current review date in `YYYY-MM-DD`.

## Exposure Score Guide

- `5`: direct pure-play or essential leader in the layer.
- `4`: strong direct exposure, major segment, or major supplier/customer role.
- `3`: meaningful exposure but diversified or less direct.
- `2`: indirect exposure or secondary business line.
- `1`: watchlist/optionality exposure.
- `0`: explicit non-exposure only if useful for future exclusion logic.

## Supply-Chain Layer Work

Only `ai_infrastructure` currently has layers. Add layers for other themes before adding exposure rows to them.

Suggested initial layer models:

### semiconductors

- `equipment`
- `materials`
- `eda_ip`
- `foundry`
- `memory`
- `analog_power`
- `logic_processors`
- `packaging_test`
- `distribution_services`

### energy

- `power_generation`
- `oil_gas_upstream`
- `oil_gas_midstream`
- `refining_marketing`
- `lng`
- `uranium_nuclear`
- `renewables`
- `grid_equipment`
- `storage`
- `services_equipment`

### defence

- `primes_platforms`
- `aerospace_engines`
- `missiles_munitions`
- `sensors_electronics`
- `cyber_c4isr`
- `shipbuilding`
- `space`
- `autonomy_drones`
- `materials_supply`

### healthcare

- `large_pharma`
- `biotech`
- `medical_devices`
- `diagnostics_tools`
- `life_science_tools`
- `healthcare_services`
- `managed_care`
- `healthcare_it`

### fintech

- `payments_networks`
- `processors_acquirers`
- `banking_software`
- `capital_markets_infrastructure`
- `exchanges_data`
- `lending_credit`
- `digital_wallets`
- `insurtech`

### commodities

- `diversified_miners`
- `precious_metals`
- `copper`
- `lithium_battery_metals`
- `uranium`
- `steel_iron_ore`
- `agriculture`
- `energy_commodities`
- `commodity_etcs`

Layer IDs may differ if you choose better names, but document them and keep them reusable.

## Prioritization

Start with high-impact names already present in enriched sectors/industries:

- AI infrastructure: Technology, Semiconductors, Utilities, Industrials, Electrical Equipment, Specialty Industrial Machinery, Software Infrastructure, Telecom/Networking, REITs/Data Centres.
- Energy: Energy sector, Utilities, uranium/nuclear, LNG, grid equipment, power equipment, renewables, oilfield services.
- Defence: Aerospace & Defense, shipbuilding, cyber/security software, sensors/electronics.
- Healthcare: Healthcare sector, biotech, pharma, devices, diagnostics/tools.
- Fintech: Financial Services, payments, exchanges, processors, banking software, capital markets.
- Commodities: Basic Materials, miners, commodity ETC/ETP securities, agriculture funds, energy commodity funds.

## Source Expectations

Use sources that can defend the claim:

- company annual report, investor presentation, or investor relations overview;
- ETF/fund issuer product page for fund objective and holdings;
- exchange/security page for listing identity;
- reputable official source for segment/business role.

Do not use unsupported web snippets. Do not use stale or unrelated sources.

## Acceptance Criteria

- `data/manual/supply_chains.yml` has useful layers for all seven current themes, unless a theme is explicitly deferred with explanation.
- `data/manual/exposures.csv` grows materially with reviewed, source-backed rows.
- The AI infrastructure map is useful beyond the seed 11 rows.
- Other themes have first-pass coverage rather than empty maps.
- `missing_theme_exposure` drops materially.
- No theme/layer is used as a dumping ground.
- `go run ./cmd/statos-build taxonomy coverage` shows improved theme/layer coverage.
- `make test` and `make smoke` pass.

## Verification

```sh
STATOS_ENRICHMENT_PROVIDER=cache GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh --no-fetch
GOCACHE="$PWD/.gocache" go run ./cmd/statos-build taxonomy coverage
make test
make smoke
python3 scripts/data-status.py --require-live
```

## Report Back

Report:

- Exposure count before and after.
- Missing theme exposure count before and after.
- Themes/layers added.
- Exposures added by theme/layer.
- Source strategy.
- Known weak or low-confidence areas.
