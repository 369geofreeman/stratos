# Mapping Prompt 2: Sector And Industry Mapping

Use this prompt for the agent assigned to sector/industry classification after identity cleanup.

## Goal

Reduce missing sector and industry rows using reviewed manual overrides and conservative rule-assisted classification.

Current queue after the 2026-05-16 refresh:

- 9,459 missing sector
- 9,459 missing industry
- 9,459 suggested classification override rows

The site needs sector/industry coverage for discovery, filtering, and detail views. Do not wait for perfect theme mapping before improving basic classification.

## Scope

Allowed work:

- Add reviewed rows to `data/manual/classification_overrides.csv`.
- Add focused helper scripts or CLI commands if they make manual review safer, but keep generated outputs deterministic.
- Add a documented classification vocabulary for funds/ETFs/ETPs if needed.
- Add tests for any new loader/validation/rule code.
- Rebuild generated `site/data`.

Out of scope:

- Do not add theme/layer exposures here except where necessary for validation.
- Do not invent sectors for individual operating companies without a source.
- Do not overwrite provider sector/industry values unless the override is reviewed and sourced.
- Do not use one generic sector for everything solely to reduce counts.

## Inputs

Review:

- `site/data/taxonomy_issues.csv`
- `site/data/suggested_classification_overrides.csv`
- `site/data/tickers.csv`
- `site/data/companies.json`
- `site/data/review_summary.json`
- `data/manual/classification_overrides.csv`
- `docs/taxonomy-workflow.md`
- `docs/data-contract.md`

Output file:

- `data/manual/classification_overrides.csv`

## Classification Principles

For operating companies:

- Prefer Yahoo-style sector names already present in `site/data/sectors.json`, unless the project documents a stronger internal vocabulary.
- Use sources such as company investor relations pages, annual reports, exchange pages, ETF issuer pages, or reputable financial profiles.
- Classify at company level when multiple tickers/securities represent the same business.

For funds, ETFs, ETPs, ETCs, investment trusts, and structured products:

- Do not pretend the fund issuer is the economic exposure unless the security truly represents the issuer.
- Use a fund/security classification vocabulary that makes the research site useful. Suggested examples:
  - sector: `Funds`
  - industries: `Equity ETF`, `Bond ETF`, `Commodity ETC`, `Crypto ETP`, `Leveraged ETP`, `Inverse ETP`, `Multi-Asset Fund`, `Money Market Fund`, `Investment Trust`, `Covered Call ETF`, `Thematic ETF`, `Sector ETF`, `Country ETF`, `Factor ETF`
- If you introduce this vocabulary, document it in `docs/taxonomy-workflow.md` or a new concise doc.

## Suggested Work Plan

1. Group missing rows by instrument category, issuer/name prefix, and structure flags.
2. Classify high-volume fund/ETP families first because they account for many missing rows.
3. Classify high-value operating companies next, prioritizing:
   - large market cap where available;
   - names likely to participate in target themes;
   - companies already enriched but missing one classification field;
   - companies appearing in watchlists/notes if present.
4. Use company-level overrides where possible to reduce duplicate ticker work.
5. Use ticker/ISIN-level overrides where fund share classes or listings need distinct classification.

Example row:

```csv
target_type,ticker,isin,company_id,sector,industry,country,source_url,last_reviewed
company,,,nvidia,Technology,Semiconductors,United States,https://investor.nvidia.com/,2026-05-16
```

## Do Not Overfit

Avoid brittle one-off name parsing in Go unless it will be tested and remains conservative. For large issuer families, a manual classification row by company/security may be safer than clever parsing.

## Acceptance Criteria

- Missing sector and missing industry counts drop materially.
- New classification rows have sources and reviewed dates.
- Existing provider/manual precedence remains deterministic.
- Fund/ETF/ETP classification is documented if added.
- `make test` passes.
- `make smoke` passes.
- `python3 scripts/data-status.py --require-live` passes.
- Review queues show the before/after reduction.

## Verification Commands

```sh
STATOS_ENRICHMENT_PROVIDER=cache GOCACHE="$PWD/.gocache" go run ./cmd/statos-build refresh --no-fetch
make test
make smoke
python3 scripts/data-status.py --require-live
python3 - <<'PY'
import json
m=json.load(open("site/data/build_manifest.json"))
print(m.get("reviewReasonCounts"))
PY
```

## Report Back

Report:

- Missing sector/industry counts before and after.
- Number of classification override rows added.
- Any new vocabulary introduced.
- Sources used.
- Remaining classification gaps by type or issuer family.
