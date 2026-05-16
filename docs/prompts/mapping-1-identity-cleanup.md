# Mapping Prompt 1: Identity Cleanup Pass

Use this prompt for the agent assigned to identity cleanup before taxonomy mapping.

## Goal

Reduce identity noise so sector, industry, theme, and relationship mapping attaches to the right companies/securities.

Current queue after the 2026-05-16 refresh:

- 15,334 identity issues
- 9,378 `broker_ticker_parse_uncertain`
- 5,956 `low_confidence_company_identity`
- Most parser uncertainty is `missing_broker_asset_code`
- Most low-confidence identity rows are fund-like or depositary receipt cases

## Scope

Focus on systematic identity improvements, not one-off manual mapping for thousands of tickers.

Allowed work:

- Improve Trading 212 broker ticker parsing for observed full-universe patterns.
- Add focused tests for observed ticker suffix/pattern classes.
- Add manual identity overrides for high-impact ADR/GDR, dual-listing, fund, trust, or ETP cases where rule logic cannot safely infer identity.
- Add or refine category/flag detection where current issues are caused by funds, ETFs, trusts, ADRs, GDRs, leveraged/inverse instruments, synthetic instruments, accumulating/distributing classes, or hedged variants.
- Update docs/readiness checklist only for completed work.

Out of scope:

- Do not classify sectors/industries here.
- Do not add theme exposures here.
- Do not build relationship graphs here.
- Do not suppress identity issues without making the underlying rule or override more correct.

## Inputs

Review:

- `site/data/identity_issues.csv`
- `site/data/review_queues.json` filtered to `queue=identity`
- `site/data/suggested_identity_overrides.csv`
- `site/data/securities.csv`
- `site/data/listings.csv`
- `site/data/tickers.csv`
- `internal/catalogue/identity.go`
- `internal/catalogue/build.go`
- `internal/catalogue/*_test.go`

Manual output file:

- `data/manual/identity_overrides.csv`

## Suggested Analysis

Start by grouping identity issues:

```sh
python3 - <<'PY'
import csv, collections
rows=list(csv.DictReader(open("site/data/identity_issues.csv")))
print(collections.Counter(r["issue_code"] for r in rows).most_common())
print(collections.Counter(r["reason"] for r in rows).most_common(30))
for r in rows[:25]:
    print(r["ticker"], r["isin"], r["name"], r["issue_code"], r["reason"])
PY
```

Look for high-volume patterns such as:

- compact exchange/currency tickers ending in `d_EQ`, `l_EQ`, `s_EQ`, `p_EQ`, `m_EQ`
- tickers where the broker symbol is present but asset code is not
- duplicate exchange/currency listings for the same ISIN
- ADR/GDR names or tickers
- fund-like names from iShares, Vanguard, Amundi, SPDR, WisdomTree, Xtrackers, UBS, Invesco, Leverage Shares, YieldMax, IncomeShares, Bitwise, 21Shares, VanEck, Global X, L&G, HSBC, JPMorgan, Fidelity
- leveraged, inverse, short, physical commodity, crypto, bond, money-market, factor, and sector ETF/ETP families

## Implementation Guidance

Prefer code improvements when a pattern is systematic and safe.

Prefer manual `identity_overrides.csv` when:

- a security should merge or split contrary to ISIN/name rules;
- an ADR/GDR should map to a specific company identity;
- a fund/ETP brand should keep a distinct security/company ID despite similar names;
- category/flags require explicit reviewed override.

Identity override rows must use the exact header:

```csv
target_type,ticker,isin,security_id,company_id,override_security_id,override_company_id,category,flags,confidence,reason,source_url,last_reviewed
```

Use:

- `manual_high` for reviewed direct overrides with a source.
- `manual_medium` for reviewed but less direct identity choices.
- `rule_low` only when adding explicit low-confidence rows for visibility.

## Acceptance Criteria

- Identity issue count is reduced, or the remaining identity issue reasons are materially more precise.
- Common ticker parser uncertainty is reduced through parser logic and tests.
- Fund/ETF/ADR/GDR categories and structure flags improve without collapsing distinct instruments incorrectly.
- No instrument is dropped silently.
- `make test` passes.
- `make smoke` passes.
- Raw replay works:

```sh
STATOS_ENRICHMENT_PROVIDER=cache GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh --no-fetch
```

## Report Back

Report:

- Identity issue counts before and after.
- Patterns fixed in code.
- Manual identity overrides added.
- Tests added.
- Any remaining high-volume patterns requiring a later pass.
