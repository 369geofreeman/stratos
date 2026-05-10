#!/usr/bin/env python3
from __future__ import annotations

import argparse
from contextlib import contextmanager, redirect_stderr, redirect_stdout
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
import hashlib
import io
import json
import logging
import os
from pathlib import Path
import re
import sys
import time
from typing import Any
import unicodedata
import warnings


SCHEMA_VERSION = 1
PROVIDER = "yfinance"
DEFAULT_INPUT = "data/raw/trading212/instruments_latest.json"
DEFAULT_FALLBACK_INPUT = "site/data/tickers_index.json"
DEFAULT_EXCHANGES_INPUT = "data/raw/trading212/exchanges_latest.json"
DEFAULT_STATOS_CACHE_DIR = "data/cache/enrichment"
DEFAULT_PROVIDER_CACHE = "data/cache/yfinance/provider_cache.json"
DEFAULT_TZ_CACHE_DIR = "data/cache/yfinance/tz-cache"
DEFAULT_RATE_LIMIT_STATE = "data/cache/yfinance/rate-limit.json"
DEFAULT_RATE_LIMIT_LOCK = "data/cache/yfinance/rate-limit.lock"
DEFAULT_MIN_INTERVAL_SECONDS = 0.5
DEFAULT_CACHE_MAX_AGE_HOURS = 72
DEFAULT_PROGRESS_SECONDS = 60
LOOKUP_TIMEOUT_SECONDS = 30
LOOKUP_MAX_RESULTS = 8
SLEEP_BUFFER_SECONDS = 0.05

QUOTE_TYPES_BY_INSTRUMENT_TYPE = {
    "STOCK": {"EQUITY", "STOCK"},
    "ETF": {"ETF", "MUTUALFUND", "FUND"},
}

YAHOO_SUFFIX_BY_T212_MARKET = {
    "AT": ".VI",
    "AU": ".AX",
    "BE": ".BR",
    "CA": ".TO",
    "CH": ".SW",
    "CZ": ".PR",
    "DE": ".DE",
    "DK": ".CO",
    "ES": ".MC",
    "FI": ".HE",
    "FR": ".PA",
    "GB": ".L",
    "GR": ".AT",
    "HK": ".HK",
    "HU": ".BD",
    "IE": ".IR",
    "IT": ".MI",
    "JP": ".T",
    "NL": ".AS",
    "NO": ".OL",
    "NZ": ".NZ",
    "PL": ".WA",
    "PT": ".LS",
    "SE": ".ST",
    "SG": ".SI",
    "UK": ".L",
    "US": "",
    "L": ".L",
    "LN": ".L",
    "LSE": ".L",
    "XETRA": ".DE",
    "PA": ".PA",
    "AS": ".AS",
    "MI": ".MI",
    "MC": ".MC",
    "SW": ".SW",
    "NASDAQ": "",
    "NYSE": "",
}


class YFinanceEnrichmentError(RuntimeError):
    pass


class RateLimitedError(YFinanceEnrichmentError):
    pass


@dataclass(frozen=True)
class Instrument:
    ticker: str
    isin: str
    name: str
    short_name: str
    instrument_type: str
    currency_code: str
    working_schedule_id: int


@dataclass(frozen=True)
class Request:
    ticker: str
    isin: str
    name: str
    instrument_type: str
    currency_code: str
    candidate_symbols: tuple[str, ...]


@dataclass(frozen=True)
class Candidate:
    symbol: str
    quote_type: str | None
    query: str
    method: str
    name: str | None = None
    exchange: str | None = None
    currency: str | None = None
    rank: tuple[int, int, int, str] = (1, 9999, 1, "")


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    instruments = load_instruments(Path(args.input), Path(args.fallback_input))
    schedule_suffixes = load_schedule_suffixes(Path(args.exchanges))
    all_groups = build_identity_groups(instruments)
    groups = all_groups
    if args.limit > 0:
        groups = groups[: args.limit]
    if args.dry_run:
        print(
            f"yfinance dry run: instruments={len(instruments)} identities={len(all_groups)} selected={len(groups)} "
            f"cache_dir={args.cache_dir}"
        )
        return 0

    try:
        backend = load_yfinance_backend(Path(args.tz_cache_dir))
    except YFinanceEnrichmentError as exc:
        print(str(exc), file=sys.stderr)
        return 2
    provider_cache = ProviderCache(Path(args.provider_cache), max_age=timedelta(hours=args.cache_max_age_hours))
    limiter = PersistentRateLimiter(
        Path(args.rate_limit_state),
        Path(args.rate_limit_lock),
        intervals={
            "lookup": args.lookup_interval,
            "info": args.info_interval,
        },
    )
    runner = Runner(
        backend=backend,
        provider_cache=provider_cache,
        limiter=limiter,
        statos_cache_dir=Path(args.cache_dir),
        schedule_suffixes=schedule_suffixes,
        max_age=timedelta(hours=args.cache_max_age_hours),
        force=args.force,
        progress_seconds=args.progress_seconds,
    )
    try:
        runner.run(groups)
    except RateLimitedError as exc:
        print(f"yfinance rate limited; stopped without caching the rate-limit failure: {exc}", file=sys.stderr)
        return 75
    except YFinanceEnrichmentError as exc:
        print(str(exc), file=sys.stderr)
        return 2
    finally:
        provider_cache.write()
    return 0


def parse_args(argv: list[str] | None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Populate Statos enrichment cache using optional local yfinance calls."
    )
    parser.add_argument("--input", default=os.getenv("STATOS_YFINANCE_INPUT", DEFAULT_INPUT))
    parser.add_argument("--fallback-input", default=DEFAULT_FALLBACK_INPUT)
    parser.add_argument("--exchanges", default=os.getenv("STATOS_YFINANCE_EXCHANGES", DEFAULT_EXCHANGES_INPUT))
    parser.add_argument("--cache-dir", default=os.getenv("STATOS_YFINANCE_STATOS_CACHE_DIR", DEFAULT_STATOS_CACHE_DIR))
    parser.add_argument("--provider-cache", default=os.getenv("STATOS_YFINANCE_PROVIDER_CACHE", DEFAULT_PROVIDER_CACHE))
    parser.add_argument("--tz-cache-dir", default=os.getenv("STATOS_YFINANCE_TZ_CACHE_DIR", DEFAULT_TZ_CACHE_DIR))
    parser.add_argument("--rate-limit-state", default=os.getenv("STATOS_YFINANCE_RATE_LIMIT_STATE", DEFAULT_RATE_LIMIT_STATE))
    parser.add_argument("--rate-limit-lock", default=os.getenv("STATOS_YFINANCE_RATE_LIMIT_LOCK", DEFAULT_RATE_LIMIT_LOCK))
    parser.add_argument(
        "--min-interval",
        type=float,
        default=float(os.getenv("STATOS_YFINANCE_MIN_INTERVAL", DEFAULT_MIN_INTERVAL_SECONDS)),
        help="default seconds between lookup/info operations when per-operation values are not set",
    )
    parser.add_argument("--lookup-interval", type=float, default=None)
    parser.add_argument("--info-interval", type=float, default=None)
    parser.add_argument(
        "--cache-max-age-hours",
        type=int,
        default=int(os.getenv("STATOS_YFINANCE_CACHE_MAX_AGE_HOURS", DEFAULT_CACHE_MAX_AGE_HOURS)),
    )
    parser.add_argument("--progress-seconds", type=int, default=DEFAULT_PROGRESS_SECONDS)
    parser.add_argument("--limit", type=int, default=0, help="process only the first N identity groups")
    parser.add_argument("--force", action="store_true", help="refresh even when a fresh Statos cache entry exists")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)
    args.lookup_interval = args.lookup_interval if args.lookup_interval is not None else args.min_interval
    args.info_interval = args.info_interval if args.info_interval is not None else args.min_interval
    if args.lookup_interval < 0 or args.info_interval < 0:
        parser.error("rate-limit intervals must be non-negative")
    if args.cache_max_age_hours < 1:
        parser.error("--cache-max-age-hours must be at least 1")
    return args


class Runner:
    def __init__(
        self,
        *,
        backend: Any,
        provider_cache: "ProviderCache",
        limiter: "PersistentRateLimiter",
        statos_cache_dir: Path,
        schedule_suffixes: dict[int, str],
        max_age: timedelta,
        force: bool,
        progress_seconds: int,
    ) -> None:
        self.backend = backend
        self.provider_cache = provider_cache
        self.limiter = limiter
        self.statos_cache_dir = statos_cache_dir
        self.schedule_suffixes = schedule_suffixes
        self.max_age = max_age
        self.force = force
        self.progress_seconds = progress_seconds
        self.stats = {
            "processed": 0,
            "skipped": 0,
            "hits": 0,
            "failures": 0,
            "ambiguous": 0,
            "provider_cache_hits": 0,
            "provider_cache_misses": 0,
            "written": 0,
        }

    def run(self, groups: list[tuple[str, list[Instrument]]]) -> None:
        started = time.monotonic()
        next_progress = started + self.progress_seconds
        total_tickers = sum(len(items) for _, items in groups)
        print(
            f"yfinance enrichment started: identities={len(groups)} tickers={total_tickers} "
            f"cache_dir={self.statos_cache_dir}",
            flush=True,
        )
        for index, (_, instruments) in enumerate(groups, start=1):
            request = request_for_group(instruments, self.schedule_suffixes)
            statos_path = statos_cache_path(self.statos_cache_dir, request)
            if not self.force and statos_cache_is_fresh(statos_path, self.max_age):
                self.stats["skipped"] += 1
                continue
            if self.force:
                self.provider_cache.delete_mapping(provider_cache_key(request))
            result = enrich_request(request, self.backend, self.provider_cache, self.limiter, self.stats)
            write_statos_cache(statos_path, request, result)
            self.stats["written"] += 1
            if self.stats["written"] % 25 == 0:
                self.provider_cache.write()
            self.stats["processed"] += 1
            status = result["status"]
            if status == "hit":
                self.stats["hits"] += 1
            elif status == "ambiguous":
                self.stats["ambiguous"] += 1
            else:
                self.stats["failures"] += 1
            now = time.monotonic()
            if now >= next_progress:
                self.log_progress(index, len(groups), started)
                next_progress = now + self.progress_seconds
        self.log_progress(len(groups), len(groups), started)

    def log_progress(self, identity_index: int, identity_total: int, started: float) -> None:
        elapsed = int(time.monotonic() - started)
        print(
            "yfinance enrichment progress: "
            f"identities={identity_index}/{identity_total} "
            f"processed={self.stats['processed']} skipped={self.stats['skipped']} "
            f"hits={self.stats['hits']} failures={self.stats['failures']} ambiguous={self.stats['ambiguous']} "
            f"provider_cache_hits={self.stats['provider_cache_hits']} "
            f"provider_cache_misses={self.stats['provider_cache_misses']} "
            f"written={self.stats['written']} elapsed={elapsed}s",
            flush=True,
        )


def enrich_request(
    request: Request,
    backend: Any,
    provider_cache: "ProviderCache",
    limiter: "PersistentRateLimiter",
    stats: dict[str, int],
) -> dict[str, Any]:
    identity = provider_cache_key(request)
    mapping = provider_cache.get_mapping(identity)
    if mapping is not None:
        stats["provider_cache_hits"] += 1
        return result_from_mapping(request, mapping, backend, provider_cache, limiter, stats)
    stats["provider_cache_misses"] += 1

    candidates = build_candidates(request, backend, limiter, include_name_lookup=False)
    result = match_candidates(request, candidates, backend, provider_cache, limiter)
    if result is not None:
        provider_cache.store_mapping(identity, result)
        return result

    name_candidates = build_candidates(request, backend, limiter, include_name_lookup=True)
    candidates = merge_candidates(candidates, name_candidates)
    result = match_candidates(request, name_candidates, backend, provider_cache, limiter)
    if result is not None:
        provider_cache.store_mapping(identity, result)
        return result

    if not candidates:
        result = failure_result(request, "no yfinance symbol candidates found")
        provider_cache.store_mapping(identity, result)
        return result

    result = failure_result(
        request,
        "unable to verify a yfinance symbol for this identity",
        candidates=candidates,
    )
    provider_cache.store_mapping(identity, result)
    return result


def match_candidates(
    request: Request,
    candidates: list[Candidate],
    backend: Any,
    provider_cache: "ProviderCache",
    limiter: "PersistentRateLimiter",
) -> dict[str, Any] | None:
    last_error = ""
    for candidate in candidates:
        try:
            fundamentals = load_fundamentals(candidate.symbol, backend, provider_cache, limiter)
        except RateLimitedError:
            raise
        except Exception as exc:
            last_error = str(exc)
            continue
        if not is_candidate_compatible(request, candidate, fundamentals):
            continue
        return hit_result(request, candidate, fundamentals)
    return failure_result(request, last_error, candidates=[]) if last_error and not candidates else None


def result_from_mapping(
    request: Request,
    mapping: dict[str, Any],
    backend: Any,
    provider_cache: "ProviderCache",
    limiter: "PersistentRateLimiter",
    stats: dict[str, int],
) -> dict[str, Any]:
    symbol = optional_string(mapping.get("yahoo_symbol"))
    if symbol is None:
        return failure_result(request, optional_string(mapping.get("reason")) or "no yfinance symbol candidates found")
    try:
        fundamentals = load_fundamentals(symbol, backend, provider_cache, limiter)
    except RateLimitedError:
        raise
    except Exception as exc:
        return failure_result(request, str(exc))
    candidate = Candidate(
        symbol=symbol,
        quote_type=optional_string(mapping.get("yahoo_quote_type")),
        query=optional_string(mapping.get("query")) or request.ticker,
        method=optional_string(mapping.get("lookup_method")) or "cache",
    )
    if not is_candidate_compatible(request, candidate, fundamentals):
        stats["provider_cache_misses"] += 1
        return enrich_request_without_mapping(request, backend, provider_cache, limiter)
    return hit_result(request, candidate, fundamentals)


def enrich_request_without_mapping(
    request: Request,
    backend: Any,
    provider_cache: "ProviderCache",
    limiter: "PersistentRateLimiter",
) -> dict[str, Any]:
    temp_stats = {"provider_cache_hits": 0, "provider_cache_misses": 0}
    identity = provider_cache_key(request)
    provider_cache.delete_mapping(identity)
    return enrich_request(request, backend, provider_cache, limiter, temp_stats)


def build_candidates(
    request: Request,
    backend: Any,
    limiter: "PersistentRateLimiter",
    *,
    include_name_lookup: bool,
) -> list[Candidate]:
    expected_quote_types = QUOTE_TYPES_BY_INSTRUMENT_TYPE.get(request.instrument_type.upper(), set())
    candidates: list[Candidate] = []
    if include_name_lookup:
        candidates.extend(lookup_candidates(request, backend, limiter, request.name, "name_lookup", expected_quote_types))
    else:
        if request.isin:
            candidates.extend(lookup_candidates(request, backend, limiter, request.isin, "isin_lookup", expected_quote_types))
        for order, symbol in enumerate(request.candidate_symbols):
            candidates.append(
                ranked_candidate(
                    symbol=symbol,
                    quote_type=None,
                    query=request.ticker,
                    method="ticker_derivation",
                    request=request,
                    expected_quote_types=expected_quote_types,
                    candidate_order=order,
                )
            )

    deduped: dict[str, Candidate] = {}
    for candidate in candidates:
        key = candidate.symbol.upper()
        if key not in deduped or candidate.rank < deduped[key].rank:
            deduped[key] = candidate
    return sorted(deduped.values(), key=lambda item: item.rank)


def lookup_candidates(
    request: Request,
    backend: Any,
    limiter: "PersistentRateLimiter",
    query: str,
    method: str,
    expected_quote_types: set[str],
) -> list[Candidate]:
    if not query or not hasattr(backend, "Lookup"):
        return []
    limiter.wait_for_slot("lookup")
    try:
        lookup = quiet_yfinance_call(
            backend.Lookup,
            query=query,
            timeout=LOOKUP_TIMEOUT_SECONDS,
            raise_errors=True,
        )
    except Exception as exc:
        if is_rate_limited_message(str(exc)):
            raise RateLimitedError(str(exc)) from exc
        return []

    candidates: list[Candidate] = []
    for row in lookup_rows(lookup):
        symbol = optional_string(row.get("symbol"))
        if symbol is None:
            continue
        candidates.append(
            ranked_candidate(
                symbol=symbol,
                quote_type=optional_string(
                    row.get("quoteType") or row.get("typeDisp") or row.get("type") or row.get("quoteTypeDisp")
                ),
                query=query,
                method=method,
                request=request,
                expected_quote_types=expected_quote_types,
                candidate_order=candidate_symbol_order(symbol, request),
                name=optional_string(row.get("longname") or row.get("shortname") or row.get("name")),
                exchange=optional_string(row.get("exchDisp") or row.get("exchange")),
                currency=optional_string(row.get("currency")),
            )
        )
    return candidates


def merge_candidates(existing: list[Candidate], additions: list[Candidate]) -> list[Candidate]:
    merged = list(existing)
    seen = {item.symbol.upper() for item in merged}
    for candidate in additions:
        key = candidate.symbol.upper()
        if key not in seen:
            merged.append(candidate)
            seen.add(key)
    return merged


def ranked_candidate(
    *,
    symbol: str,
    quote_type: str | None,
    query: str,
    method: str,
    request: Request,
    expected_quote_types: set[str],
    candidate_order: int = 9999,
    name: str | None = None,
    exchange: str | None = None,
    currency: str | None = None,
) -> Candidate:
    resolved_quote_type = normalise_quote_type(quote_type)
    symbol_upper = symbol.upper()
    exact_quote_type_match = 0 if not expected_quote_types or resolved_quote_type in expected_quote_types else 1
    derived_match = 0 if symbol_upper in {item.upper() for item in request.candidate_symbols} else 1
    name_match = 0 if symbolise_text(request.name) == symbol_upper else 1
    return Candidate(
        symbol=symbol,
        quote_type=resolved_quote_type,
        query=query,
        method=method,
        name=name,
        exchange=exchange,
        currency=currency,
        rank=(exact_quote_type_match, candidate_order if derived_match == 0 else 9999, name_match, symbol_upper),
    )


def load_fundamentals(
    symbol: str,
    backend: Any,
    provider_cache: "ProviderCache",
    limiter: "PersistentRateLimiter",
) -> dict[str, Any]:
    cached = provider_cache.get_fundamentals(symbol)
    if cached is not None:
        return cached
    limiter.wait_for_slot("info")
    try:
        ticker = backend.Ticker(symbol)
        info = coerce_mapping(quiet_yfinance_call(call_ticker_accessor, ticker, "get_info", "info"))
        matched_isin = optional_string(quiet_yfinance_call(call_ticker_accessor, ticker, "get_isin", "isin"))
        if matched_isin == "-":
            matched_isin = None
    except Exception as exc:
        if is_rate_limited_message(str(exc)):
            raise RateLimitedError(str(exc)) from exc
        raise YFinanceEnrichmentError(f"yfinance fundamentals failed for symbol {symbol!r}: {exc}") from exc

    market_cap = optional_int(info.get("marketCap"))
    if market_cap is None:
        fast_info = coerce_mapping(quiet_yfinance_call(call_ticker_accessor, ticker, "get_fast_info", "fast_info"))
        market_cap = optional_int(fast_info.get("marketCap") or fast_info.get("market_cap"))
    payload = {
        "cached_at": utc_now(),
        "symbol": symbol,
        "quote_type": optional_string(info.get("quoteType")),
        "name": optional_string(info.get("longName") or info.get("shortName")),
        "sector": optional_string(info.get("sector")),
        "industry": optional_string(info.get("industry")),
        "country": optional_string(info.get("country")),
        "market_cap": market_cap,
        "market_cap_currency": optional_string(info.get("currency") or info.get("financialCurrency")),
        "exchange_name": optional_string(info.get("fullExchangeName") or info.get("exchange")),
        "matched_isin": matched_isin,
    }
    provider_cache.store_fundamentals(symbol, payload)
    return payload


def is_candidate_compatible(request: Request, candidate: Candidate, fundamentals: dict[str, Any]) -> bool:
    expected_quote_types = QUOTE_TYPES_BY_INSTRUMENT_TYPE.get(request.instrument_type.upper(), set())
    resolved_quote_type = normalise_quote_type(fundamentals.get("quote_type") or candidate.quote_type)
    if expected_quote_types and resolved_quote_type is not None and resolved_quote_type not in expected_quote_types:
        return False
    expected_isin = normalise_isin(request.isin)
    matched_isin = normalise_isin(optional_string(fundamentals.get("matched_isin")))
    if expected_isin is not None and matched_isin is not None and expected_isin != matched_isin:
        return is_confident_exchange_suffix_match(request, candidate, fundamentals)
    if expected_isin is not None and candidate.method in {"ticker_derivation", "name_lookup"} and matched_isin is None:
        return is_confident_exchange_suffix_match(request, candidate, fundamentals)
    return True


def candidate_symbol_order(symbol: str, request: Request) -> int:
    symbol_upper = symbol.upper()
    for index, candidate in enumerate(request.candidate_symbols):
        if candidate.upper() == symbol_upper:
            return index
    return 9999


def is_confident_exchange_suffix_match(request: Request, candidate: Candidate, fundamentals: dict[str, Any]) -> bool:
    symbol_upper = candidate.symbol.upper()
    candidate_symbols = {symbol.upper() for symbol in request.candidate_symbols}
    if symbol_upper not in candidate_symbols:
        return False
    if "." not in candidate.symbol:
        return False
    if optional_string(fundamentals.get("sector")) is None:
        return False
    provider_name = optional_string(fundamentals.get("name"))
    return names_are_plausible(request.name, provider_name)


def names_are_plausible(expected: str, actual: str | None) -> bool:
    expected_tokens = company_name_tokens(expected)
    actual_tokens = company_name_tokens(actual or "")
    if not expected_tokens or not actual_tokens:
        return False
    if expected_tokens.issubset(actual_tokens) or actual_tokens.issubset(expected_tokens):
        return True
    shared = expected_tokens & actual_tokens
    if any(len(token) >= 4 for token in shared):
        return True
    if len(expected_tokens) == 1 and len(actual_tokens) == 1 and shared:
        return True
    return len(shared) >= min(2, len(expected_tokens), len(actual_tokens))


def hit_result(request: Request, candidate: Candidate, fundamentals: dict[str, Any]) -> dict[str, Any]:
    profile = {
        "symbol": optional_string(fundamentals.get("symbol")) or candidate.symbol,
        "name": optional_string(fundamentals.get("name")) or "",
        "sector": optional_string(fundamentals.get("sector")) or "",
        "industry": optional_string(fundamentals.get("industry")) or "",
        "exchange": optional_string(fundamentals.get("exchange_name")) or "",
        "currency": optional_string(fundamentals.get("market_cap_currency")) or "",
        "country": optional_string(fundamentals.get("country")) or "",
        "marketCap": optional_int(fundamentals.get("market_cap")) or 0,
        "source": PROVIDER,
        "retrievedAt": utc_now(),
    }
    return {
        "provider": PROVIDER,
        "request": request_snapshot(request),
        "profile": strip_empty(profile),
        "candidates": [],
        "status": "hit",
        "error": "" if profile.get("sector") else "missing yfinance sector",
        "retrievedAt": utc_now(),
        "mapping": {
            "query": candidate.query,
            "lookup_method": candidate.method,
            "yahoo_symbol": candidate.symbol,
            "yahoo_quote_type": fundamentals.get("quote_type") or candidate.quote_type,
            "reason": "" if profile.get("sector") else "missing yfinance sector",
        },
    }


def failure_result(request: Request, message: str, candidates: list[Candidate] | None = None) -> dict[str, Any]:
    return {
        "provider": PROVIDER,
        "request": request_snapshot(request),
        "profile": {},
        "candidates": [candidate_payload(item) for item in (candidates or [])],
        "status": "failure",
        "error": message,
        "retrievedAt": utc_now(),
        "mapping": {
            "query": request.isin or request.ticker,
            "lookup_method": "not_resolved",
            "yahoo_symbol": "",
            "yahoo_quote_type": "",
            "reason": message,
        },
    }


def write_statos_cache(path: Path, request: Request, result: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = {
        "schemaVersion": SCHEMA_VERSION,
        "provider": result["provider"],
        "request": request_snapshot(request),
        "profile": result.get("profile") or {},
        "candidates": result.get("candidates") or [],
        "status": result["status"],
        "error": result.get("error") or "",
        "retrievedAt": result.get("retrievedAt") or utc_now(),
    }
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    temporary.replace(path)


class ProviderCache:
    def __init__(self, path: Path, *, max_age: timedelta) -> None:
        self.path = path
        self.max_age = max_age
        self.payload = self._load()

    def _load(self) -> dict[str, Any]:
        if not self.path.exists():
            return {"version": 1, "mappings": {}, "fundamentals_by_symbol": {}}
        try:
            payload = json.loads(self.path.read_text(encoding="utf-8"))
        except Exception:
            return {"version": 1, "mappings": {}, "fundamentals_by_symbol": {}}
        if not isinstance(payload, dict):
            return {"version": 1, "mappings": {}, "fundamentals_by_symbol": {}}
        return {
            "version": payload.get("version", 1),
            "mappings": payload.get("mappings", {}) if isinstance(payload.get("mappings"), dict) else {},
            "fundamentals_by_symbol": (
                payload.get("fundamentals_by_symbol", {})
                if isinstance(payload.get("fundamentals_by_symbol"), dict)
                else {}
            ),
        }

    def get_mapping(self, key: str) -> dict[str, Any] | None:
        entry = self.payload["mappings"].get(key)
        if not isinstance(entry, dict):
            return None
        if not entry.get("yahoo_symbol") and not is_fresh(entry.get("cached_at"), self.max_age):
            return None
        return entry

    def delete_mapping(self, key: str) -> None:
        self.payload["mappings"].pop(key, None)

    def store_mapping(self, key: str, result: dict[str, Any]) -> None:
        mapping = result.get("mapping") or {}
        self.payload["mappings"][key] = {
            "cached_at": utc_now(),
            "query": mapping.get("query") or "",
            "lookup_method": mapping.get("lookup_method") or "",
            "yahoo_symbol": mapping.get("yahoo_symbol") or "",
            "yahoo_quote_type": mapping.get("yahoo_quote_type") or "",
            "reason": mapping.get("reason") or result.get("error") or "",
        }

    def get_fundamentals(self, symbol: str) -> dict[str, Any] | None:
        entry = self.payload["fundamentals_by_symbol"].get(symbol)
        if isinstance(entry, dict) and is_fresh(entry.get("cached_at"), self.max_age):
            return entry
        return None

    def store_fundamentals(self, symbol: str, payload: dict[str, Any]) -> None:
        self.payload["fundamentals_by_symbol"][symbol] = payload

    def write(self) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        temporary = self.path.with_suffix(self.path.suffix + ".tmp")
        temporary.write_text(json.dumps(self.payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        temporary.replace(self.path)


class PersistentRateLimiter:
    def __init__(self, state_path: Path, lock_path: Path, *, intervals: dict[str, float]) -> None:
        self.state_path = state_path
        self.lock_path = lock_path
        self.intervals = intervals

    def wait_for_slot(self, operation: str) -> None:
        interval = self.intervals.get(operation)
        if interval is None:
            return
        wait_seconds = self.reserve(operation, interval)
        if wait_seconds > 0:
            time.sleep(wait_seconds + SLEEP_BUFFER_SECONDS)

    def reserve(self, operation: str, interval: float) -> float:
        with file_lock(self.lock_path):
            state = read_rate_limit_state(self.state_path)
            now = time.time()
            entry = state.setdefault("operations", {}).get(operation, {})
            next_allowed = float(entry.get("next_allowed_at_unix") or 0)
            request_at = max(now, next_allowed)
            wait_seconds = max(0.0, request_at - now)
            state["operations"][operation] = {
                "next_allowed_at_unix": request_at + interval,
                "period_seconds": interval,
                "updated_at_unix": now,
            }
            write_rate_limit_state(self.state_path, state)
        return wait_seconds


@contextmanager
def file_lock(path: Path):
    path.parent.mkdir(parents=True, exist_ok=True)
    deadline = time.monotonic() + 5
    descriptor = None
    while descriptor is None:
        try:
            descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL)
            os.write(descriptor, json.dumps({"pid": os.getpid(), "acquired_at_unix": time.time()}).encode("utf-8"))
            os.close(descriptor)
            descriptor = -1
        except FileExistsError:
            if time.monotonic() >= deadline:
                raise YFinanceEnrichmentError(f"rate-limit lock already held: {path}")
            time.sleep(0.05)
    try:
        yield
    finally:
        try:
            path.unlink()
        except FileNotFoundError:
            pass


def read_rate_limit_state(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {"operations": {}}
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        return {"operations": {}}
    return payload if isinstance(payload, dict) else {"operations": {}}


def write_rate_limit_state(path: Path, state: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(state, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    temporary.replace(path)


def load_yfinance_backend(tz_cache_dir: Path) -> Any:
    warnings.filterwarnings("ignore", message=".*Timestamp.utcnow is deprecated.*")
    logging.getLogger("yfinance").setLevel(logging.CRITICAL)
    logging.getLogger("yfinance").propagate = False
    try:
        import yfinance as yf  # type: ignore
    except ModuleNotFoundError as exc:
        raise YFinanceEnrichmentError(
            "optional yfinance dependency is missing; install with "
            "`python3 -m pip install -r requirements-enrichment.txt`"
        ) from exc
    try:
        if hasattr(yf, "set_tz_cache_location"):
            tz_cache_dir.mkdir(parents=True, exist_ok=True)
            yf.set_tz_cache_location(str(tz_cache_dir))
    except Exception:
        pass
    try:
        config = getattr(yf, "config", None)
        network = getattr(config, "network", None)
        if network is not None and hasattr(network, "retries"):
            network.retries = 2
    except Exception:
        pass
    return yf


def load_instruments(input_path: Path, fallback_path: Path) -> list[Instrument]:
    if input_path.exists():
        return instruments_from_payload(json.loads(input_path.read_text(encoding="utf-8")))
    if fallback_path.exists():
        payload = json.loads(fallback_path.read_text(encoding="utf-8"))
        if isinstance(payload, dict) and isinstance(payload.get("tickers"), list):
            return instruments_from_payload(payload["tickers"])
    raise YFinanceEnrichmentError(
        f"missing enrichment input: {input_path}; run make refresh first or pass --input"
    )


def instruments_from_payload(payload: Any) -> list[Instrument]:
    if not isinstance(payload, list):
        raise YFinanceEnrichmentError("instrument input must be a JSON array or tickers_index object")
    instruments = []
    for row in payload:
        if not isinstance(row, dict):
            continue
        ticker = optional_string(row.get("ticker")) or ""
        if not ticker:
            continue
        instruments.append(
            Instrument(
                ticker=ticker,
                isin=optional_string(row.get("isin")) or "",
                name=optional_string(row.get("name")) or "",
                short_name=optional_string(row.get("shortName") or row.get("short_name")) or "",
                instrument_type=optional_string(row.get("type") or row.get("instrumentCategory")) or "",
                currency_code=optional_string(row.get("currencyCode") or row.get("currency")) or "",
                working_schedule_id=optional_int(row.get("workingScheduleId") or row.get("working_schedule_id")) or 0,
            )
        )
    return instruments


def load_schedule_suffixes(path: Path) -> dict[int, str]:
    if not path.exists():
        return {}
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        return {}
    if not isinstance(payload, list):
        return {}
    out: dict[int, str] = {}
    for exchange in payload:
        if not isinstance(exchange, dict):
            continue
        suffix = yahoo_suffix_for_exchange_name(optional_string(exchange.get("name")) or "")
        if suffix is None:
            continue
        schedules = exchange.get("workingSchedules")
        if not isinstance(schedules, list):
            continue
        for schedule in schedules:
            if not isinstance(schedule, dict):
                continue
            schedule_id = optional_int(schedule.get("id"))
            if schedule_id is not None:
                out[schedule_id] = suffix
    return out


def yahoo_suffix_for_exchange_name(name: str) -> str | None:
    normalized = name.strip().casefold()
    if normalized in {"deutsche börse xetra", "gettex"}:
        return ".DE"
    if normalized in {"london stock exchange", "london stock exchange aim", "london stock exchange non-isa"}:
        return ".L"
    if normalized == "euronext amsterdam":
        return ".AS"
    if normalized == "euronext brussels":
        return ".BR"
    if normalized == "euronext paris":
        return ".PA"
    if normalized == "euronext lisbon":
        return ".LS"
    if normalized == "wiener börse":
        return ".VI"
    if normalized == "bolsa de madrid":
        return ".MC"
    if normalized == "borsa italiana":
        return ".MI"
    if normalized == "six swiss exchange":
        return ".SW"
    if normalized == "toronto stock exchange":
        return ".TO"
    if normalized in {"nasdaq", "nyse", "otc markets"}:
        return ""
    return None


def build_identity_groups(instruments: list[Instrument]) -> list[tuple[str, list[Instrument]]]:
    groups: dict[str, list[Instrument]] = {}
    for instrument in instruments:
        groups.setdefault(identity_key(instrument), []).append(instrument)
    out = []
    for key, items in groups.items():
        out.append((key, sorted(items, key=lambda item: item.ticker.upper())))
    return sorted(out, key=lambda item: item[0])


def identity_key(instrument: Instrument) -> str:
    if instrument.isin.strip():
        return "isin:" + instrument.isin.strip().upper()
    return "ticker:" + instrument.ticker.strip().upper()


def request_for_group(instruments: list[Instrument], schedule_suffixes: dict[int, str] | None = None) -> Request:
    first = instruments[0]
    symbols: list[str] = []
    for instrument in instruments:
        symbols = append_unique(symbols, candidate_symbols_for_instrument(instrument, schedule_suffixes or {}))
    return Request(
        ticker=first.ticker,
        isin=first.isin,
        name=first.name,
        instrument_type=first.instrument_type,
        currency_code=first.currency_code,
        candidate_symbols=tuple(symbols),
    )


def candidate_symbols(ticker: str) -> list[str]:
    ticker = ticker.strip()
    if not ticker:
        return []
    candidates: list[str] = []
    parts = ticker.split("_")
    if len(parts) >= 3:
        base = "_".join(parts[:-2])
        exchange = parts[-2].upper()
        suffix = YAHOO_SUFFIX_BY_T212_MARKET.get(exchange)
        if suffix is not None:
            candidates.append(f"{base}{suffix}")
        if base:
            candidates.append(base)
    return append_unique([], candidates)


def candidate_symbols_for_instrument(instrument: Instrument, schedule_suffixes: dict[int, str]) -> list[str]:
    candidates = candidate_symbols(instrument.ticker)
    if candidates:
        return candidates
    parts = instrument.ticker.strip().split("_")
    if len(parts) != 2 or parts[1].upper() != "EQ":
        return []
    base = compact_symbol_base(parts[0], instrument.short_name)
    if base == "":
        return []
    out: list[str] = []
    suffix = schedule_suffixes.get(instrument.working_schedule_id)
    if suffix is not None:
        out.append(base + suffix)
    out.append(base)
    return append_unique([], out)


def compact_symbol_base(raw_symbol: str, short_name: str) -> str:
    short_name = short_name.strip()
    if short_name:
        return short_name
    raw_symbol = raw_symbol.strip()
    if len(raw_symbol) > 1 and raw_symbol[-1].islower():
        return raw_symbol[:-1]
    return raw_symbol


def statos_cache_path(cache_dir: Path, request: Request) -> Path:
    return cache_dir / (statos_cache_key(request) + ".json")


def statos_cache_key(request: Request) -> str:
    if request.isin.strip():
        value = "ISIN|" + request.isin.strip().upper()
    else:
        value = "|".join([request.ticker.upper(), request.name.upper()])
    return hashlib.sha1(value.encode("utf-8")).hexdigest()


def provider_cache_key(request: Request) -> str:
    if request.isin.strip():
        return "isin:" + request.isin.strip().upper()
    return "ticker:" + request.ticker.strip().upper()


def statos_cache_is_fresh(path: Path, max_age: timedelta) -> bool:
    if not path.exists():
        return False
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        return False
    if not isinstance(payload, dict) or payload.get("schemaVersion") != SCHEMA_VERSION:
        return False
    if payload.get("status") not in {"hit", "failure", "ambiguous"}:
        return False
    return is_fresh(payload.get("retrievedAt"), max_age)


def request_snapshot(request: Request) -> dict[str, Any]:
    return strip_empty(
        {
            "ticker": request.ticker,
            "isin": request.isin,
            "name": request.name,
            "candidateSymbols": list(request.candidate_symbols),
        }
    )


def candidate_payload(candidate: Candidate) -> dict[str, Any]:
    return strip_empty(
        {
            "symbol": candidate.symbol,
            "name": candidate.name or "",
            "exchange": candidate.exchange or "",
            "currency": candidate.currency or "",
            "quoteType": candidate.quote_type or "",
            "source": candidate.method,
        }
    )


def lookup_rows(lookup: Any) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for method_name, property_name in (("get_stock", "stock"), ("get_etf", "etf"), ("get_all", "all")):
        payload = None
        method = getattr(lookup, method_name, None)
        if callable(method):
            payload = method(count=LOOKUP_MAX_RESULTS)
        elif hasattr(lookup, property_name):
            payload = getattr(lookup, property_name)
        rows.extend(coerce_rows(payload))
    return rows


def coerce_rows(payload: Any) -> list[dict[str, Any]]:
    if isinstance(payload, list):
        return [item for item in payload if isinstance(item, dict)]
    if isinstance(payload, dict):
        return [payload]
    to_dict = getattr(payload, "to_dict", None)
    if callable(to_dict):
        try:
            records = to_dict(orient="records")
        except TypeError:
            records = to_dict()
        if isinstance(records, list):
            return [item for item in records if isinstance(item, dict)]
        if isinstance(records, dict):
            rows = []
            for symbol, row in records.items():
                if isinstance(row, dict):
                    item = {str(key): value for key, value in row.items()}
                    item.setdefault("symbol", str(symbol))
                    rows.append(item)
            return rows
    return []


def call_ticker_accessor(ticker: Any, method_name: str, property_name: str) -> Any:
    method = getattr(ticker, method_name, None)
    if callable(method):
        return method()
    return getattr(ticker, property_name, None)


def quiet_yfinance_call(callable_object: Any, *args: Any, **kwargs: Any) -> Any:
    with warnings.catch_warnings():
        warnings.filterwarnings("ignore", message=".*Timestamp.utcnow is deprecated.*")
        with redirect_stdout(io.StringIO()), redirect_stderr(io.StringIO()):
            return callable_object(*args, **kwargs)


def coerce_mapping(value: Any) -> dict[str, Any]:
    return value if isinstance(value, dict) else {}


def strip_empty(payload: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in payload.items() if value not in ("", None, [], {})}


def optional_string(value: Any) -> str | None:
    if value is None:
        return None
    text = str(value).strip()
    return text if text else None


def optional_int(value: Any) -> int | None:
    if value is None:
        return None
    if isinstance(value, bool):
        return None
    if isinstance(value, (int, float)):
        return int(value)
    try:
        return int(float(str(value).strip()))
    except ValueError:
        return None


def normalise_quote_type(value: Any) -> str | None:
    text = optional_string(value)
    return text.upper() if text else None


def normalise_isin(value: str | None) -> str | None:
    text = optional_string(value)
    if text in {None, "-"}:
        return None
    return text.upper()


def symbolise_text(value: str) -> str:
    return "".join(character for character in value.upper() if character.isalnum())


def company_name_tokens(value: str) -> set[str]:
    stopwords = {
        "A",
        "AG",
        "AKTIENGESELLSCHAFT",
        "AND",
        "CO",
        "COMP",
        "CORP",
        "CORPORATION",
        "INC",
        "LTD",
        "PLC",
        "SA",
        "SE",
        "THE",
    }
    normalized = unicodedata.normalize("NFKD", value)
    ascii_text = normalized.encode("ascii", "ignore").decode("ascii")
    tokens = {raw.upper() for raw in re.split(r"[^A-Za-z0-9]+", ascii_text) if raw}
    return {token for token in tokens if len(token) >= 2 and token not in stopwords}


def append_unique(existing: list[str], additions: list[str] | tuple[str, ...]) -> list[str]:
    seen = {item.upper() for item in existing}
    for value in additions:
        text = value.strip()
        key = text.upper()
        if text and key not in seen:
            existing.append(text)
            seen.add(key)
    return existing


def is_fresh(value: Any, max_age: timedelta) -> bool:
    text = optional_string(value)
    if text is None:
        return False
    try:
        parsed = datetime.fromisoformat(text.replace("Z", "+00:00"))
    except ValueError:
        return False
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return datetime.now(timezone.utc) - parsed.astimezone(timezone.utc) <= max_age


def is_rate_limited_message(message: str) -> bool:
    lowered = message.lower()
    return "429 too many requests" in lowered or "rate limit" in lowered or "rate-limited" in lowered


def utc_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


if __name__ == "__main__":
    raise SystemExit(main())
