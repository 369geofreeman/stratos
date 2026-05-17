# Mapping Prompt 9: Part 4 Fund And Instrument Pipelines

Use this prompt for the agent assigned to adding fund, ETP, warrant, and structured-product pipeline coverage after the Part 3 economy pipeline pass.

## Goal

Add pipeline views for non-operating-company instruments so Explorer can group the Trading 212 universe by fund and product structure:

- `funds_core`: equity ETFs, bond ETFs, factor ETFs, covered-call ETFs, money-market funds, multi-asset funds, and investment trusts.
- `leveraged_structured`: leveraged ETPs, inverse ETPs, warrants, and structured products.
- `commodity_crypto_etps`: commodity ETPs, crypto ETPs, precious-metal products, broad commodity baskets, and single-commodity products.

This pass should make fund and instrument navigation useful without pretending these products are operating-company supply chains.

## Current Context

Statos is a static GitHub Pages investment research site for Trading 212 Invest / Stocks ISA-compatible tickers.

After Part 3 on 2026-05-17, generated live/raw-replay data is approximately:

- 17,050 Trading 212 tickers
- 13,215 companies
- 17 active pipelines in generated data
- 7,148 unclassified stock rows
- 3,423 missing sector rows
- 3,423 missing industry rows
- 3,725 missing theme exposure rows
- 9,348 enrichment failures
- 0 identity issues

Fund and structured-product coverage is already classified by sector/industry, but it has little or no pipeline exposure:

- `Funds`: 5,910 tickers
- `Structured Products`: 22 tickers
- `Equity ETF`: 3,061 tickers
- `Bond ETF`: 958 tickers
- `Leveraged ETP`: 508 tickers
- `Inverse ETP`: 453 tickers
- `Factor ETF`: 427 tickers
- `Commodity ETP`: 283 tickers
- `Covered Call ETF`: 173 tickers
- `Crypto ETP`: 37 tickers
- `Warrant`: 22 tickers
- `Money Market Fund`: 4 tickers
- `Multi-Asset Fund`: 3 tickers
- `Investment Trust`: 3 tickers

Important: current review logic does not put `Funds` or `Structured Products` into the missing-theme review queue when sector and industry are present. This pass is therefore primarily an Explorer coverage and product-navigation pass, not a normal missing-theme queue-reduction pass.

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
- `data/manual/exposures.csv` for reviewed exceptions or high-value seed rows.
- `data/manual/identity_overrides.csv` only if a product is misclassified by instrument category or structure flags.
- `data/manual/ticker_overrides.csv` only if a fund/security needs source-backed metadata correction.
- optionally `data/manual/notes/*.md` if the product taxonomy needs explanatory context.

Code, if needed for scalable product mapping:

- `internal/catalogue/*`
- `internal/taxonomy/*`
- `cmd/statos-build/*`
- associated tests under the same packages.

Generated static data:

- `site/data/themes.json`
- `site/data/supply_chains.json`
- `site/data/explorer_index.json`
- `site/data/review_summary.json`
- `site/data/review_queues.json`
- other generated `site/data` files touched by raw replay.

## Operating Rule

Each mapping pass must:

1. Measure current review queues and fund/instrument coverage before changing anything.
2. Decide whether the change is manual-only or requires deterministic rule/code mapping.
3. Add focused manual taxonomy and code changes.
4. Rebuild from the latest raw snapshot without unnecessary network calls.
5. Measure review queues and fund/instrument Explorer coverage after the change.
6. Report what improved and what remains.

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
- Use `manual_medium` when the source supports the direction but the exact exposure is estimated or the product is broad/diversified.
- Use `rule_low` only for broad rule-based mapping from trusted instrument metadata, and add tests for any rule code.
- Keep rationales short and specific.
- Do not use a theme/layer as a catch-all just to reduce queue counts.
- Prefer fewer high-quality manual rows over large unsupported guesses.
- Prefer `isin` for product-level exposure rows when the exposure is tied to the security and should cover all broker listings of the same security.
- Prefer `ticker` only when the exposure is broker-listing-specific.
- Avoid `company_id` for fund/product exposure unless you have confirmed the generated fund-like company identity represents exactly the intended security set.
- Do not rename or remove existing layer IDs unless there is a migration reason and all existing exposure rows are updated deliberately.

## Work Order

1. Baseline measurement.
2. Add the three Part 4 themes.
3. Add product layers for the three themes.
4. Inspect how Explorer currently receives theme/layer IDs for fund-like instruments.
5. Implement deterministic product rules if manual rows would otherwise require hundreds or thousands of rows.
6. Add targeted manual rows or overrides only where rules are insufficient.
7. Rebuild and measure.
8. Verify that operating-company pipelines remain unchanged.
9. Run verification.
10. Report coverage and remaining product gaps.

## Part 4 Pipeline Definitions

Add these themes to `data/manual/themes.yml`.

Suggested IDs and names:

- `funds_core`: Core funds
- `leveraged_structured`: Leveraged and structured products
- `commodity_crypto_etps`: Commodity and crypto ETPs

Keep names concise and user-facing. Colors should be distinct from existing themes and should not make the UI read as a one-hue palette.

## Product Mapping Strategy

This pass is different from operating-company mapping.

For operating companies, manual exposure rows attach a company to a pipeline. For funds and structured products, the pipeline usually belongs to the security itself:

- the same issuer may have many unrelated products;
- products with the same provider should not all inherit one exposure;
- leveraged, inverse, covered-call, crypto, and commodity products are product-structure classifications;
- multiple broker listings of the same ISIN should usually share the same product pipeline exposure.

Preferred approach:

1. Use `sector`, `industry`, `instrument_category`, `type`, `structure_flags`, `directionality`, `name`, and `isin` from generated Trading 212-derived data.
2. Add deterministic code rules where the industry/category already proves the product family.
3. Keep those rule assignments narrow and testable.
4. Use manual `isin` exposure rows only for exceptions, ambiguous products, or high-value products whose official issuer page supports a more specific layer.

Do not hand-enter thousands of fund rows if a tested deterministic rule can map them correctly.

## Suggested Layer Models

Layer IDs may be adjusted if a better local pattern emerges, but keep them stable, reusable, and specific enough for Explorer filtering.

### funds_core

Purpose: Long-only and income-oriented fund wrappers that users expect to browse as normal portfolio building blocks.

Suggested layers:

- `equity_etfs`: broad, sector, regional, thematic, and single-country equity ETFs.
- `bond_etfs`: government, corporate, aggregate, high-yield, inflation-linked, and duration bond ETFs.
- `factor_etfs`: quality, value, momentum, dividend, minimum-volatility, equal-weight, and factor-screened ETFs.
- `covered_call_etfs`: covered-call, buy-write, option-income, and derivative-income ETFs.
- `money_market_funds`: money-market and cash-management funds.
- `multi_asset_funds`: mixed-asset, allocation, balanced, and fund-of-funds products.
- `investment_trusts`: investment trusts and closed-end funds where present in the Trading 212 universe.

Priority generated industries:

- Equity ETF
- Bond ETF
- Factor ETF
- Covered Call ETF
- Money Market Fund
- Multi-Asset Fund
- Investment Trust

### leveraged_structured

Purpose: Instruments whose payoff profile, leverage, short exposure, or legal/product wrapper is materially different from a core fund.

Suggested layers:

- `leveraged_etps`: leveraged long ETPs and leveraged ETFs.
- `inverse_etps`: inverse, short, and bear ETPs.
- `leveraged_inverse_etps`: products that are both leveraged and inverse, if distinguishable from generated metadata.
- `warrants`: exchange-traded warrants.
- `structured_products`: structured notes, certificates, and other structured instruments.
- `complex_payoff_products`: products with path dependency, daily reset, or non-core payoff mechanics that are not better captured above.

Priority generated industries and flags:

- Leveraged ETP
- Inverse ETP
- Warrant
- Structured Products sector
- `structure_flags` containing `leveraged`, `inverse`, `short`, `warrant`, or similar local flags.
- `directionality` where it identifies inverse or leveraged direction.

### commodity_crypto_etps

Purpose: Exchange-traded products that give exposure to commodities, precious metals, baskets, or crypto assets.

Suggested layers:

- `broad_commodity_etps`: diversified commodity baskets and broad commodity index products.
- `precious_metals_etps`: precious-metals baskets and mixed precious-metals products.
- `gold_etps`: gold products.
- `silver_etps`: silver products.
- `energy_commodity_etps`: oil, gas, carbon, and energy-linked commodity products.
- `agriculture_commodity_etps`: agriculture, grains, livestock, and soft commodity products.
- `industrial_metals_etps`: copper, aluminium, nickel, battery metals, and industrial-metal products.
- `crypto_etps`: bitcoin, ether, basket crypto, and other crypto exchange-traded products.

Priority generated industries and names:

- Commodity ETP
- Crypto ETP
- Gold
- Silver
- Other Precious Metals & Mining only if the security is actually an ETP/product, not an operating miner.
- product names containing clearly sourced commodity/product exposure from an issuer page.

## Overlap Rules

Fund and instrument pipelines can overlap with operating-company categories, but they should not pollute operating-company pipeline maps.

- Do not map fund products into operating-company themes such as `energy`, `healthcare`, `semiconductors`, or `consumer_brands` merely because a thematic ETF references those sectors.
- If thematic fund exposure is needed later, create a separate fund-lookthrough design instead of pretending the fund issuer is an operating company.
- `funds_core` should not include leveraged, inverse, warrant, crypto, or commodity products.
- `leveraged_structured` can overlap with `commodity_crypto_etps` only when a commodity/crypto product is also leveraged or inverse and Explorer benefits from both filters.
- `commodity_crypto_etps` should include commodity/crypto products regardless of issuer, but not commodity operating companies.
- Keep the existing Part 1, Part 2, and Part 3 operating-company pipelines intact.

## Candidate Discovery

Use generated data to measure product families before editing.

```sh
python3 - <<'PY'
import csv, collections
rows = list(csv.DictReader(open("site/data/tickers.csv", newline="")))
for sector in ["Funds", "Structured Products"]:
    subset = [r for r in rows if r["sector"] == sector]
    print(sector, len(subset))
    print("industry", collections.Counter(r["industry"] or "<blank>" for r in subset).most_common())
    print("category", collections.Counter(r["instrument_category"] or "<blank>" for r in subset).most_common())
    print("type", collections.Counter(r["type"] or "<blank>" for r in subset).most_common())
PY
```

Find product rows without themes:

```sh
python3 - <<'PY'
import csv
rows = list(csv.DictReader(open("site/data/tickers.csv", newline="")))
for r in rows:
    if r["sector"] in {"Funds", "Structured Products"} and not r["themes"]:
        print(r["ticker"], r["isin"], r["name"], r["sector"], r["industry"], r["instrument_category"], r["structure_flags"])
PY
```

Measure Part 4 coverage after rebuild:

```sh
python3 - <<'PY'
import csv, collections
themes = {"funds_core", "leveraged_structured", "commodity_crypto_etps"}
rows = list(csv.DictReader(open("site/data/tickers.csv", newline="")))
counts = collections.Counter()
layers = collections.Counter()
for r in rows:
    row_themes = set(filter(None, r["themes"].split(";")))
    row_layers = set(filter(None, r["layers"].split(";")))
    for theme in themes & row_themes:
        counts[theme] += 1
        for layer in row_layers:
            layers[(theme, layer)] += 1
print("themes")
for item in counts.most_common():
    print(item)
print("layers")
for item in sorted(layers.items()):
    print(item)
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
- `source_url`: issuer product page, official issuer fund page, official exchange/security page for identity, or other reputable source.
- `rationale`: one short sentence specific to the product layer.
- `last_reviewed`: current review date in `YYYY-MM-DD`.

Exposure score guide for products:

- `5`: product is directly and primarily in the layer.
- `4`: product has strong direct exposure but is diversified or mixed.
- `3`: meaningful exposure but broad, mixed, or partially indirect.
- `2`: secondary or partial exposure.
- `1`: watchlist/edge-case exposure.
- `0`: explicit non-exposure only if useful for future exclusion logic.

## Code Rule Expectations

If adding deterministic product mapping rules:

- Keep the rules local, explicit, and tested.
- Base rules on generated Trading 212-derived fields already present in `Ticker`.
- Do not require network access at runtime.
- Do not add provider-specific runtime dependencies.
- Make rule confidence clear, normally `rule_low` for broad product-family rules.
- Ensure generated `site/data/explorer_index.json` receives the theme/layer groups.
- Ensure `go run ./cmd/statos-build taxonomy coverage` or an equivalent measurement reports the new coverage clearly.
- Add tests covering at least:
  - equity ETF to `funds_core/equity_etfs`;
  - bond ETF to `funds_core/bond_etfs`;
  - factor ETF to `funds_core/factor_etfs`;
  - covered-call ETF to `funds_core/covered_call_etfs`;
  - leveraged ETP to `leveraged_structured/leveraged_etps`;
  - inverse ETP to `leveraged_structured/inverse_etps`;
  - warrant to `leveraged_structured/warrants`;
  - commodity ETP to `commodity_crypto_etps`;
  - crypto ETP to `commodity_crypto_etps/crypto_etps`;
  - operating-company stocks are not accidentally mapped by these product rules.

If the current data model cannot represent rule-generated product exposures cleanly, report that as a blocker and propose the smallest data-contract-safe change.

## Sourcing Expectations

Good sources for manual product rows:

- official issuer product pages;
- official issuer fund factsheets;
- official prospectus pages;
- official exchange/security pages for identity;
- Trading 212 raw metadata as the source universe, surfaced through generated static data.

Avoid:

- unsupported web snippets;
- mapping by product name alone when metadata and issuer pages disagree;
- generic finance profile pages as the only basis when an issuer page exists;
- applying issuer-level exposure to all products from that issuer.

## Initial Coverage Targets

These are targets, not permission to guess:

- Define all 3 Part 4 themes.
- Define at least 5 useful layers for each new theme where the universe supports them.
- Map the bulk of `Funds` and `Structured Products` into one or more Part 4 Explorer groups using deterministic, tested logic where possible.
- At minimum, cover the generated industries listed in Current Context.
- Keep manual rows targeted to exceptions or representative high-value products.
- Do not increase identity issues.
- Do not create a new tail of exposed-but-misclassified fund products.

Because current review logic does not count classified funds as missing theme exposure, `missing_theme_exposure` may not drop materially. That is acceptable if Explorer coverage for fund and product groups improves materially and the report explains why.

## Rebuild And Verification

After manual/code changes:

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

- `data/manual/themes.yml` contains the 3 new Part 4 themes.
- `data/manual/supply_chains.yml` contains useful layer maps for all 3 new product pipelines.
- Explorer can filter funds and structured products by the new product pipelines.
- Core fund, leveraged/inverse/structured, commodity, crypto, and precious-metal products are separated into intuitive groups.
- Operating-company themes are not polluted by fund products.
- Existing Part 1, Part 2, and Part 3 pipelines remain present and are not broken.
- Generated `site/data/build_manifest.json` remains live/raw-replay derived.
- No generated `site/data` file exceeds GitHub's 50 MB recommended limit.
- `make test`, `make smoke`, and `python3 scripts/data-status.py --require-live` pass.

## Report Back

Report:

- Baseline fund/product counts before the pass.
- Counts after the pass.
- Themes and layers added.
- Whether deterministic code rules were added.
- Manual exposure rows or overrides added, if any.
- Representative sources used for manual exceptions.
- Product families still unmapped or ambiguous.
- Any identity/category issues discovered while mapping.
- Whether all verification commands passed.

Also include a recommended follow-up list if thematic ETF lookthrough, issuer grouping, or product-risk labeling should be handled in a later pass.
