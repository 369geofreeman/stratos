# Mapping Prompt 6: Part 1 Biggest-Gap Pipelines

Use this prompt for the agent assigned to the first broad pipeline expansion pass.

## Goal

Add the next major research pipelines so the site can answer:

- Which tickers are linked by financial-system exposure?
- Which tickers are linked by enterprise software and IT infrastructure?
- Which tickers are linked by industrial automation, machinery, and electrification?
- Which tickers are linked by consumer brands and consumer channels?
- Which tickers are linked by transport, freight, and logistics infrastructure?

This prompt is intentionally broader than a single-theme cleanup pass. Define the full Part 1 pipeline vocabulary up front, then seed as many reviewed, source-backed exposure rows as possible in defensible batches.

## Current Context

Statos is a static GitHub Pages investment research site for Trading 212 Invest / Stocks ISA-compatible tickers.

After the latest mapping sprint on 2026-05-16, generated live/raw-replay data is approximately:

- 17,050 Trading 212 tickers
- 13,215 companies
- 7 active pipelines in generated data:
  - `ai_infrastructure`
  - `energy`
  - `defence`
  - `healthcare`
  - `fintech`
  - `semiconductors`
  - `commodities`
- 7,641 unclassified stock rows
- 3,492 missing sector rows
- 3,492 missing industry rows
- 4,149 missing theme exposure rows
- 9,348 enrichment failures

The largest remaining mapped-sector, missing-pipeline clusters include:

- Financial Services: banks, insurers, asset managers, capital markets, financial data and exchanges.
- Technology: SaaS, infrastructure software, IT services, cybersecurity, developer/data platforms.
- Industrials: machinery, automation, electrification, construction equipment, engineering services.
- Consumer: mass retail, specialty retail, luxury/apparel, beverages, household products, restaurants.
- Transport/logistics: rail, shipping, parcel/logistics, airlines, airports, freight and ports.

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
- optionally `data/manual/notes/*.md` if a pipeline needs explanatory context

Generated static data:

- `site/data/themes.json`
- `site/data/supply_chains.json`
- `site/data/explorer_index.json`
- `site/data/review_summary.json`
- `site/data/review_queues.json`
- other generated `site/data` files touched by the raw replay

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

## Work Order

1. Baseline measurement.
2. Add all five Part 1 themes.
3. Add supply-chain layers for all five themes.
4. Seed exposure rows in batches, starting with high-market-cap and high-review-queue clusters.
5. Rebuild and measure.
6. Add a second exposure batch if there is time and sources are clear.
7. Run verification.
8. Report coverage and remaining gaps.

## Part 1 Pipeline Definitions

Add these themes to `data/manual/themes.yml`.

Suggested IDs and names:

- `financial_system`: Financial system
- `enterprise_software`: Enterprise software
- `industrial_automation`: Industrial automation
- `consumer_brands`: Consumer brands
- `transport_logistics`: Transport and logistics

Keep names user-facing and concise. Colors should be distinct from existing themes and should not turn the UI into a one-hue palette.

## Suggested Layer Models

Layer IDs may be adjusted if a better local pattern emerges, but keep them stable, reusable, and specific enough for Explorer filtering.

### financial_system

Purpose: Banks, insurers, asset managers, exchanges, brokers, credit/rating/data, and market infrastructure.

Suggested layers:

- `diversified_banks`: global and national universal banks.
- `regional_banks`: regional and local banking groups.
- `insurance`: life, property/casualty, reinsurance, specialty insurance.
- `asset_management`: asset managers, alternative managers, wealth platforms.
- `capital_markets`: investment banks, brokers, trading platforms, market makers.
- `exchanges_data`: exchanges, market data, index/rating and financial analytics providers.
- `credit_payments`: credit card issuers, credit bureaus, and non-bank credit platforms.

Do not use this as a duplicate dump of `fintech`. Keep `fintech` focused on payments software, banking software, digital wallets, crypto rails, and digital-native financial platforms. Cross-map only when the business genuinely fits both.

### enterprise_software

Purpose: Enterprise SaaS, infrastructure software, developer tools, data/observability, cybersecurity, IT services, and vertical software.

Suggested layers:

- `erp_crm_workflow`: ERP, CRM, finance, HR, collaboration, productivity, workflow software.
- `developer_tools`: software development, DevOps, CI/CD, API, testing, and code platforms.
- `data_observability`: data platforms, analytics, monitoring, observability, search, and logging.
- `cybersecurity`: identity, endpoint, network, cloud, and application security.
- `it_services_consulting`: IT consulting, managed services, systems integration.
- `vertical_software`: industry-specific software platforms.
- `communications_collaboration`: enterprise communications and collaboration software.

Avoid mapping every technology company here. The source should support enterprise software, IT services, or cybersecurity exposure.

### industrial_automation

Purpose: Machinery, industrial automation, electrification, controls, construction/agriculture equipment, and industrial technology.

Suggested layers:

- `automation_controls`: factory automation, robotics, process controls, sensors, drives, and industrial controls.
- `electrification_power`: electrical equipment, power management, switchgear, motors, drives, and grid-adjacent industrial equipment.
- `machinery_equipment`: industrial machinery, compressors, pumps, tools, manufacturing equipment.
- `construction_agriculture_equipment`: construction, mining, agriculture, and heavy machinery.
- `industrial_software`: PLM, CAD/CAM, industrial analytics, simulation, automation software.
- `testing_measurement`: industrial testing, measurement, instrumentation, and quality systems.
- `engineering_services`: engineering, construction, maintenance, and industrial services.

Cross-mapping with `energy` and `ai_infrastructure` is allowed when the company has direct exposure to grid/power/data-centre buildout, but keep the rationale specific.

### consumer_brands

Purpose: Consumer retail, apparel/luxury, beverages, household/personal care, restaurants, and durable consumer franchises.

Suggested layers:

- `mass_retail`: supermarkets, warehouses, discounters, broadline retailers.
- `specialty_retail`: category specialists, home improvement, off-price, beauty, sporting goods, auto retail.
- `luxury_apparel`: luxury goods, apparel, footwear, accessories, cosmetics brands.
- `beverages_tobacco`: alcoholic beverages, non-alcoholic beverages, tobacco and nicotine.
- `household_personal_care`: household products, personal care, beauty, hygiene.
- `restaurants_foodservice`: restaurants, coffee chains, foodservice and franchised dining.
- `consumer_durables`: appliances, furnishings, leisure products, recreational vehicles.

Do not map every consumer cyclical company. Prioritize recognizable operating brands and companies with clear product/channel exposure.

### transport_logistics

Purpose: Rail, parcel/logistics, freight, shipping, airlines, airports, ports, and transport infrastructure.

Suggested layers:

- `railroads`: freight and passenger rail operators where listed.
- `parcel_logistics`: parcel delivery, contract logistics, postal/logistics networks.
- `shipping_ports`: marine shipping, container lines, port operators, terminals.
- `airlines_airports`: airlines, airport operators, aviation services.
- `trucking_freight`: trucking, freight brokerage, less-than-truckload, road freight.
- `freight_forwarding`: forwarding, customs, supply-chain management.
- `transport_infrastructure`: toll roads, logistics infrastructure, intermodal and transport concessions.

Avoid mapping transport-adjacent manufacturers here unless the company is primarily a transport operator or infrastructure/service provider. Put aircraft and equipment makers in `industrial_automation` or `defence` where appropriate.

## Candidate Discovery

Use generated data to find source-backed candidates. Examples:

```sh
jq -r '.[] | select((.themeIds|not) and (.marketCap // 0) > 25000000000 and (.sector != "Funds")) | [.id,.name,.primaryTicker,.sector,.industry,((.marketCap//0)/1000000000|floor)] | @tsv' site/data/companies.json
```

```sh
jq -r '[.[] | select((.themeIds|not) and (.marketCap // 0) > 25000000000 and (.sector != "Funds")) | {sector, industry, marketCap: (.marketCap // 0)}] | group_by(.sector + "|" + .industry) | map({sector: .[0].sector, industry: .[0].industry, count: length, marketCap: (map(.marketCap) | add)}) | sort_by(-.marketCap) | .[:40][] | [.count, (.marketCap/1000000000|floor), .sector, .industry] | @tsv' site/data/companies.json
```

Industry clusters to prioritize:

- Financial Services:
  - `Banks - Diversified`
  - `Banks - Regional`
  - `Insurance - Diversified`
  - `Insurance - Life`
  - `Insurance - Property & Casualty`
  - `Asset Management`
  - `Capital Markets`
  - `Financial Data & Stock Exchanges`
  - `Credit Services`
- Technology:
  - `Software - Application`
  - `Software - Infrastructure`
  - `Information Technology Services`
  - `Communication Equipment`
  - `Scientific & Technical Instruments`
- Industrials:
  - `Specialty Industrial Machinery`
  - `Farm & Heavy Construction Machinery`
  - `Electrical Equipment & Parts`
  - `Engineering & Construction`
  - `Industrial Distribution`
  - `Tools & Accessories`
- Consumer:
  - `Discount Stores`
  - `Specialty Retail`
  - `Apparel Retail`
  - `Luxury Goods`
  - `Beverages - Non-Alcoholic`
  - `Beverages - Brewers`
  - `Household & Personal Products`
  - `Restaurants`
  - `Home Improvement Retail`
- Transport and logistics:
  - `Railroads`
  - `Integrated Freight & Logistics`
  - `Marine Shipping`
  - `Airlines`
  - `Airports & Air Services`
  - `Trucking`

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

## Sourcing Expectations

Good sources:

- official investor relations pages;
- annual reports, 10-Ks, 20-Fs, registration statements;
- official business segment/product pages;
- official exchange/security pages for identity only;
- official issuer pages for funds or ETPs if any are mapped.

Avoid:

- unsourced web snippets;
- generic finance profile pages as the only basis for exposure when an official source is available;
- stale or unrelated product pages;
- mapping by name alone when the business line is not clear.

## Initial Coverage Targets

These are targets, not permission to guess:

- Define all 5 new themes.
- Define at least 5 useful layers for each new theme.
- Seed every new theme with at least 15 reviewed exposure rows if sources are clear.
- Seed every new layer with at least 1 reviewed row where possible.
- Prefer a total Part 1 seed batch of 100-200 exposure rows if sourcing can be done cleanly.
- If time is limited, prioritize high-market-cap operating companies and clusters with many `missing_theme_exposure` rows.

If a layer cannot be sourced in this pass, keep the layer if it is structurally important, but call it out in the report as a thin layer.

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

- `data/manual/themes.yml` contains the 5 new Part 1 pipeline themes.
- `data/manual/supply_chains.yml` contains useful layer maps for all 5 new pipelines.
- `data/manual/exposures.csv` contains source-backed seed rows across all 5 new pipelines.
- `site/data/themes.json` and `site/data/supply_chains.json` include the new pipelines after raw replay.
- `site/data/explorer_index.json` includes clickable groups for the new pipelines and their layers.
- `missing_theme_exposure` drops.
- Existing pipelines are not broken or renamed.
- No generated `site/data` file exceeds GitHub's 50 MB recommended limit.
- `make test`, `make smoke`, and `python3 scripts/data-status.py --require-live` pass.

## Report Back

Report:

- Baseline counts before the pass.
- Counts after the pass.
- Themes and layers added.
- Exposure rows added by theme and layer.
- Representative sources used.
- Thin layers that still need more rows.
- Any duplicate identity problems discovered while mapping.
- Whether all verification commands passed.

Also include a recommended Part 1 follow-up list if the pass could not source every high-value company cleanly.
