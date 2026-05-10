# Enrichment Provider Contract

Statos treats Trading 212 instrument metadata as the source universe. Enrichment providers are optional, replaceable helpers for improving fields such as Yahoo-compatible symbol, sector, industry, country, exchange, currency, and market cap.

## Provider Boundary

Providers implement `internal/enrichment.Provider`:

```go
Lookup(context.Context, enrichment.Request) (enrichment.Result, error)
```

`Request` is built from Trading 212 metadata. The builder groups instruments by identity before live/cache enrichment: `ISIN` is used where available, with ticker fallback only when ISIN is missing. Providers may use `Ticker`, `ISIN`, `Name`, `CurrencyCode`, `ExchangeCode`, and precomputed `CandidateSymbols`, but they must not redefine the source universe or write generated `site/data` files directly.

`Result` has explicit normalized fields:

- `profile`: the provider fields that can be applied when `status` is `hit`.
- `candidates`: plausible provider matches when lookup/search is ambiguous.
- `status`: `hit`, `failure`, `ambiguous`, `cache_miss`, or `unknown_schema`.
- `error`: a human-readable failure reason.
- `retrievedAt`: when the provider response or failure was cached.

Ambiguous matches must not be silently applied. Return `StatusAmbiguous`, preserve candidate symbols, and leave `Profile` empty unless there is a deterministic rule that makes exactly one candidate safe to apply.

## Cache Contract

The builder uses `CacheProvider` in front of live providers. Cache files are ignored under `data/cache/enrichment`, are keyed by ISIN identity when possible, and use a versioned envelope:

```json
{
  "schemaVersion": 1,
  "provider": "yahoo",
  "request": {
    "ticker": "VOD_L_EQ",
    "isin": "GB00BH4HKS39",
    "name": "Vodafone Group plc",
    "candidateSymbols": ["VOD.L", "VOD", "VOD_L_EQ"]
  },
  "profile": {
    "symbol": "VOD.L",
    "sector": "Communication Services",
    "industry": "Telecom Services",
    "exchange": "LSE",
    "currency": "GBp",
    "country": "United Kingdom",
    "marketCap": 123
  },
  "candidates": [],
  "status": "hit",
  "error": "",
  "retrievedAt": "2026-05-09T12:00:00Z"
}
```

Unknown schema versions are reported as `unknown_schema` failures and are not trusted. Stale entries are still usable by default, but stale counts and oldest/newest retrieval timestamps are written to `site/data/build_manifest.json`.

Cache hit/miss/stale counts are identity-level provider/cache observations. Enrichment failure rows remain ticker-level so each Trading 212 ticker still has an explicit review action when its shared identity lookup fails.

Yahoo `429 Too Many Requests` responses are treated as transient rate-limit failures and should not be cached by new runs. If older cache files contain 429 failures, run `make clean-rate-limited-enrichment-cache` before retrying enrichment. Large live runs can be paced with `STATOS_ENRICHMENT_DELAY`, such as `2s`.

## Yahoo-Compatible Provider

Yahoo Finance does not provide a stable official public developer API. The optional Yahoo-compatible provider is enrichment only. yfinance documents that it is not affiliated with, endorsed by, or vetted by Yahoo and that it uses publicly available APIs for research and educational purposes: <https://ranaroussi.github.io/yfinance/index.html>.

The Yahoo-compatible provider tries ISIN search first, then deterministic symbols derived from every Trading 212 ticker in the identity group. For non-US listings with a known Yahoo exchange suffix, the exchange-suffixed symbol is attempted before the base symbol and raw broker ticker. If search returns multiple plausible equity/fund candidates, the result is marked ambiguous and provider fields are not applied.

## Generated Diagnostics

Provider cache entries and raw responses stay out of `site/data`. The generated site only receives normalized diagnostics:

- `site/data/enrichment_failures.csv`
- manifest fields for cache schema version, provider, hit/miss/stale counts, ambiguous count, failure count, and oldest/newest retrieval timestamps

Manual overrides in `data/manual/ticker_overrides.csv` always win over provider profile fields.
