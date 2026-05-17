# Mapping Prompt 8: Part 3 Missing Economy Pipelines

Use this prompt for the agent assigned to adding the next economy-level pipeline families after Part 1 and Part 2.

## Goal

Add missing economy pipelines that make the catalogue useful outside technology, financials, energy, healthcare, defence, commodities, and industrial maps:

- `digital_platforms`: marketplaces, ads, streaming, gaming, social, and internet retail.
- `real_estate_infrastructure`: REITs, towers, logistics property, storage, property services, and infrastructure-like real estate.
- `food_agriculture`: food producers, ingredients, agriculture machinery, fertiliser, groceries, and food supply chains.
- `mobility_ev`: autos, EVs, batteries, charging, components, ride-hailing, dealers, and mobility services.
- `building_housing`: homebuilders, building materials, tools, HVAC, furnishings, home improvement, and housing infrastructure.

This prompt should add the full Part 3 pipeline vocabulary and seed enough reviewed exposure rows to make each new pipeline usable in Explorer.

## Current Context

Statos is a static GitHub Pages investment research site for Trading 212 Invest / Stocks ISA-compatible tickers.

After Part 2 on 2026-05-17, generated live/raw-replay data is approximately:

- 17,050 Trading 212 tickers
- 13,215 companies
- 12 active pipelines in generated data
- 7,217 unclassified stock rows
- 3,461 missing sector rows
- 3,461 missing industry rows
- 3,756 missing theme exposure rows
- 9,348 enrichment failures
- 0 identity issues

Existing pipeline families include:

- `ai_infrastructure`
- `semiconductors`
- `energy`
- `defence`
- `healthcare`
- `fintech`
- `commodities`
- `financial_system`
- `enterprise_software`
- `industrial_automation`
- `consumer_brands`
- `transport_logistics`

Part 3 should not duplicate those pipelines. It should add missing economy views and use cross-mapping only where a company genuinely belongs in multiple pipelines.

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

- `data/manual/themes.yml`
- `data/manual/supply_chains.yml`
- `data/manual/exposures.csv`
- `data/manual/classification_overrides.csv` if a newly exposed company still lacks sector/industry.
- optionally `data/manual/notes/*.md` if a pipeline needs explanatory context.

Generated static data:

- `site/data/themes.json`
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
2. Add all five Part 3 themes.
3. Add supply-chain layers for all five Part 3 themes.
4. Seed reviewed exposure rows in batches, starting with high-market-cap and high-review-queue clusters.
5. Add sector/industry classification overrides for newly exposed companies that still lack classification.
6. Rebuild and measure.
7. Add a second exposure batch if sources are clear.
8. Run verification.
9. Report coverage and remaining gaps.

## Part 3 Pipeline Definitions

Add these themes to `data/manual/themes.yml`.

Suggested IDs and names:

- `digital_platforms`: Digital platforms
- `real_estate_infrastructure`: Real estate infrastructure
- `food_agriculture`: Food and agriculture
- `mobility_ev`: Mobility and EV
- `building_housing`: Building and housing

Keep names concise and user-facing. Colors should be distinct from existing themes and should not make the UI read as a one-hue palette.

## Overlap Rules

Part 3 naturally overlaps with existing pipelines. Use these rules to keep Explorer useful:

- Cross-map only when the company has direct source-backed exposure to both pipelines.
- Do not move existing rows out of existing themes unless there is a clear migration reason.
- `consumer_brands` maps broad consumer brands and retail channels; `digital_platforms` maps internet-native platform economics.
- `commodities` maps raw material and processor exposure; `food_agriculture` maps food production, distribution, ingredients, grocery, and agriculture operating chains.
- `industrial_automation` maps industrial equipment and automation; `building_housing` maps housing, construction materials, home improvement, HVAC, furnishings, and building products.
- `energy` maps energy generation and infrastructure; `mobility_ev` maps vehicle electrification, autos, charging, ride-hailing, and auto retail.
- `ai_infrastructure` already has `data_centres`; `real_estate_infrastructure` can include data-centre REITs only if the intent is real-estate/infrastructure exposure, not cloud or AI compute.

## Suggested Layer Models

Layer IDs may be adjusted if a better local pattern emerges, but keep them stable, reusable, and specific enough for Explorer filtering.

### digital_platforms

Purpose: Internet-native marketplaces, advertising, social, streaming, gaming, app stores, and ecommerce platforms.

Suggested layers:

- `marketplaces_ecommerce`: ecommerce marketplaces, internet retail, marketplace operators.
- `digital_advertising`: digital ad platforms, ad tech, performance marketing, and app-monetization platforms.
- `social_platforms`: social networks, creator platforms, messaging, and community platforms.
- `streaming_media`: streaming video, music, subscription media, and connected TV platforms.
- `gaming_interactive`: video games, interactive entertainment, game engines, and esports platforms.
- `travel_local_platforms`: online travel, local services, delivery, classifieds, and booking platforms.
- `platform_infrastructure`: app stores, platform services, cloud-linked marketplace infrastructure, and ecosystem operators.

Priority candidates and clusters:

- Internet Content & Information
- Internet Retail
- Entertainment
- Electronic Gaming & Multimedia
- Advertising Agencies
- Travel Services
- Software - Application where platform economics are direct
- Communication Services companies with social, streaming, or ad exposure

### real_estate_infrastructure

Purpose: Real-estate operating companies and infrastructure-like property exposures.

Suggested layers:

- `industrial_logistics_reits`: logistics, warehouse, industrial, and fulfilment property.
- `tower_reits`: cell towers, communications real estate, and tower infrastructure.
- `data_centre_reits`: data-centre real estate and colocation property platforms.
- `self_storage`: self-storage REITs and storage operators.
- `residential_reits`: apartments, single-family rentals, manufactured housing, and student housing.
- `retail_office_healthcare_reits`: retail, office, healthcare, hotel, and specialty property REITs.
- `property_services`: real estate services, brokerage, property data, property management, and facilities services.

Priority candidates and clusters:

- REIT - Industrial
- REIT - Specialty
- REIT - Retail
- REIT - Residential
- REIT - Office
- REIT - Healthcare Facilities
- REIT - Hotel & Motel
- REIT - Diversified
- REIT - Mortgage only where operating/property exposure is clear
- Real Estate Services
- Real Estate - Development
- Infrastructure Operations

### food_agriculture

Purpose: Food production, agriculture inputs, ingredients, grocery, farm equipment, and food distribution.

Suggested layers:

- `packaged_foods`: packaged food, snacks, meals, and consumer food brands.
- `protein_fresh_food`: meat, dairy, seafood, produce, and fresh-food producers.
- `ingredients_flavors`: ingredients, flavours, enzymes, sweeteners, and specialty food inputs.
- `agriculture_machinery`: tractors, crop equipment, precision agriculture hardware, and farm machinery.
- `fertiliser_crop_inputs`: fertilizer, crop protection, seeds, and farm inputs.
- `grocery_distribution`: grocery retailers, food distributors, wholesalers, and foodservice distributors.
- `agriculture_processing`: grain merchants, oilseed processors, commodity processors, and farm products.

Priority candidates and clusters:

- Packaged Foods
- Farm Products
- Food Distribution
- Grocery Stores
- Agricultural Inputs
- Beverages where food-system exposure is direct
- Specialty Chemicals where crop/food inputs are direct
- Farm & Heavy Construction Machinery where agriculture equipment is a major segment

### mobility_ev

Purpose: Autos, electric vehicles, batteries, charging, auto components, dealers, ride-hailing, and mobility services.

Suggested layers:

- `auto_oems`: incumbent passenger vehicle, truck, and commercial vehicle manufacturers.
- `ev_pure_play`: EV-first vehicle makers and electrified mobility manufacturers.
- `batteries_powertrain`: battery cells, battery systems, drivetrains, power electronics, and fuel cells.
- `charging_infrastructure`: EV charging networks, charging equipment, and fleet charging infrastructure.
- `auto_components`: auto parts, tyres, safety systems, electronics, interiors, and component suppliers.
- `dealers_retail_services`: auto dealers, auto service, rental, fleet, auctions, and aftermarket platforms.
- `ride_hailing_mobility`: ride-hailing, delivery mobility, car sharing, and mobility platforms.

Priority candidates and clusters:

- Auto Manufacturers
- Auto Parts
- Auto & Truck Dealerships
- Recreational Vehicles where mobility exposure is direct
- Rental & Leasing Services for vehicle rental and fleet platforms
- Internet Content/Application names with ride-hailing or delivery mobility exposure
- Electrical Equipment & Parts where EV charging or vehicle electrification is direct
- Specialty Chemicals and lithium/battery materials only when mobility battery exposure is direct and source-backed

### building_housing

Purpose: Homebuilders, building materials, home improvement, HVAC, tools, furnishings, fixtures, and housing services.

Suggested layers:

- `homebuilders_developers`: homebuilders, residential developers, and manufactured housing builders.
- `building_materials`: cement, aggregates, insulation, roofing, wallboard, timber, and building products.
- `home_improvement_retail`: home improvement retailers, pro channels, and building supply retail.
- `hvac_building_systems`: HVAC, controls, elevators, fire/security, water systems, and building efficiency.
- `tools_hardware`: tools, hardware, fasteners, locks, and professional construction tools.
- `furnishings_fixtures`: furnishings, fixtures, appliances, flooring, cabinets, and home goods.
- `housing_services`: property services, repair/remodel, rental services, and residential services platforms.

Priority candidates and clusters:

- Residential Construction
- Building Materials
- Building Products & Equipment
- Home Improvement Retail
- Furnishings, Fixtures & Appliances
- Tools & Accessories
- Lumber & Wood Production
- Real Estate - Development
- Specialty Retail where home improvement or housing is direct
- Specialty Industrial Machinery where HVAC/building systems are direct

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
themes = {"digital_platforms", "real_estate_infrastructure", "food_agriculture", "mobility_ev", "building_housing"}
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

- Define all 5 new Part 3 themes.
- Define at least 6 useful layers for each new theme.
- Seed every new theme with at least 20 reviewed exposure rows if sources are clear.
- Seed every new layer with at least 2 reviewed rows where possible.
- Prefer a total Part 3 seed batch of 120-200 exposure rows if sourcing can be done cleanly.
- Reduce `missing_theme_exposure` materially.
- Keep existing Part 1 and Part 2 pipelines intact.

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

- `data/manual/themes.yml` contains the 5 new Part 3 pipeline themes.
- `data/manual/supply_chains.yml` contains useful layer maps for all 5 new pipelines.
- `data/manual/exposures.csv` contains source-backed seed rows across all 5 new pipelines.
- `site/data/themes.json` and `site/data/supply_chains.json` include the new pipelines after raw replay.
- `site/data/explorer_index.json` includes clickable groups for the new pipelines and their layers.
- `missing_theme_exposure` drops.
- Newly exposed operating companies do not remain blank for sector/industry without explanation.
- Existing Part 1 and Part 2 pipelines remain present and are not broken.
- No generated `site/data` file exceeds GitHub's 50 MB recommended limit.
- `make test`, `make smoke`, and `python3 scripts/data-status.py --require-live` pass.

## Report Back

Report:

- Baseline counts before the pass.
- Counts after the pass.
- Themes and layers added.
- Exposure rows added by theme and layer.
- Classification overrides added, if any.
- Representative sources used.
- Thin layers that still need more rows.
- Any duplicate identity problems discovered while mapping.
- Whether all verification commands passed.

Also include a recommended Part 3 follow-up list if the pass could not source every high-value company cleanly.
