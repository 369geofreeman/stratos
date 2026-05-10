#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest


MODULE_PATH = Path(__file__).with_name("enrich_yfinance.py")
SPEC = importlib.util.spec_from_file_location("enrich_yfinance", MODULE_PATH)
assert SPEC is not None and SPEC.loader is not None
enrich_yfinance = importlib.util.module_from_spec(SPEC)
sys.modules["enrich_yfinance"] = enrich_yfinance
SPEC.loader.exec_module(enrich_yfinance)


class YFinanceHelperTests(unittest.TestCase):
    def test_candidate_symbols_uses_exchange_suffix_priority(self) -> None:
        self.assertEqual(
            enrich_yfinance.candidate_symbols("ABRA_CA_EQ"),
            ["ABRA.TO", "ABRA"],
        )
        self.assertEqual(
            enrich_yfinance.candidate_symbols("VOD_L_EQ"),
            ["VOD.L", "VOD"],
        )

    def test_identity_grouping_and_request_candidates(self) -> None:
        instruments = [
            enrich_yfinance.Instrument("ABC_US_EQ", "US0000000001", "ABC Corp", "", "STOCK", "USD", 0),
            enrich_yfinance.Instrument("ABC_L_EQ", "US0000000001", "ABC Corp", "", "STOCK", "GBP", 0),
            enrich_yfinance.Instrument("XYZ_US_EQ", "US0000000002", "XYZ Corp", "", "STOCK", "USD", 0),
        ]
        groups = enrich_yfinance.build_identity_groups(instruments)
        self.assertEqual(len(groups), 2)
        request = enrich_yfinance.request_for_group(groups[0][1])
        self.assertEqual(request.ticker, "ABC_L_EQ")
        self.assertEqual(
            list(request.candidate_symbols),
            ["ABC.L", "ABC"],
        )

    def test_compact_ticker_uses_short_name_and_working_schedule_suffix(self) -> None:
        instrument = enrich_yfinance.Instrument(
            "SANTd_EQ",
            "AT0000A0E9W5",
            "Kontron",
            "KTN",
            "STOCK",
            "EUR",
            54,
        )
        self.assertEqual(
            enrich_yfinance.candidate_symbols_for_instrument(instrument, {54: ".DE"}),
            ["KTN.DE", "KTN"],
        )

    def test_exchange_schedule_suffixes_load_from_trading212_metadata(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "exchanges.json"
            path.write_text(
                json.dumps(
                    [
                        {"name": "Wiener Börse", "workingSchedules": [{"id": 73}]},
                        {"name": "Deutsche Börse Xetra", "workingSchedules": [{"id": 54}]},
                    ]
                ),
                encoding="utf-8",
            )
            self.assertEqual(enrich_yfinance.load_schedule_suffixes(path), {73: ".VI", 54: ".DE"})

    def test_statos_cache_key_is_identity_level_when_isin_exists(self) -> None:
        first = enrich_yfinance.Request("ABC_US_EQ", "US0000000001", "ABC Corp", "STOCK", "USD", ())
        second = enrich_yfinance.Request("ABC_L_EQ", "US0000000001", "ABC Corp London", "STOCK", "GBP", ())
        third = enrich_yfinance.Request("ABC_L_EQ", "", "ABC Corp London", "STOCK", "GBP", ())
        self.assertEqual(enrich_yfinance.statos_cache_key(first), enrich_yfinance.statos_cache_key(second))
        self.assertNotEqual(enrich_yfinance.statos_cache_key(first), enrich_yfinance.statos_cache_key(third))

    def test_write_statos_cache_uses_current_schema(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            request = enrich_yfinance.Request(
                "ABC_US_EQ",
                "US0000000001",
                "ABC Corp",
                "STOCK",
                "USD",
                ("ABC",),
            )
            result = enrich_yfinance.failure_result(request, "not found")
            path = enrich_yfinance.statos_cache_path(Path(tmp), request)
            enrich_yfinance.write_statos_cache(path, request, result)
            payload = json.loads(path.read_text(encoding="utf-8"))
            self.assertEqual(payload["schemaVersion"], enrich_yfinance.SCHEMA_VERSION)
            self.assertEqual(payload["provider"], "yfinance")
            self.assertEqual(payload["status"], "failure")
            self.assertEqual(payload["request"]["candidateSymbols"], ["ABC"])

    def test_ticker_derivation_prefers_exchange_suffix_order(self) -> None:
        request = enrich_yfinance.Request(
            "EBS_AT_EQ",
            "AT0000652011",
            "Erste Group Bank",
            "STOCK",
            "EUR",
            ("EBS.VI", "EBS"),
        )
        candidates = enrich_yfinance.build_candidates(request, object(), _NoopLimiter(), include_name_lookup=False)
        self.assertEqual([item.symbol for item in candidates], ["EBS.VI", "EBS"])

    def test_exchange_suffix_can_match_when_yfinance_isin_is_missing(self) -> None:
        request = enrich_yfinance.Request(
            "EBS_AT_EQ",
            "AT0000652011",
            "Erste Group Bank",
            "STOCK",
            "EUR",
            ("EBS.VI", "EBS"),
        )
        fundamentals = {
            "quote_type": "EQUITY",
            "name": "Erste Group Bank AG",
            "sector": "Financial Services",
            "matched_isin": None,
        }
        good = enrich_yfinance.Candidate("EBS.VI", "EQUITY", "EBS_AT_EQ", "ticker_derivation")
        base = enrich_yfinance.Candidate("EBS", "EQUITY", "EBS_AT_EQ", "ticker_derivation")

        self.assertTrue(enrich_yfinance.is_candidate_compatible(request, good, fundamentals))
        self.assertFalse(enrich_yfinance.is_candidate_compatible(request, base, fundamentals))

    def test_exchange_suffix_can_match_same_company_when_yfinance_isin_differs(self) -> None:
        request = enrich_yfinance.Request(
            "STR_AT_EQ",
            "AT000000STR1",
            "Strabag",
            "STOCK",
            "EUR",
            ("STR.VI", "STR"),
        )
        fundamentals = {
            "quote_type": "EQUITY",
            "name": "Strabag SE",
            "sector": "Industrials",
            "matched_isin": "AT0000A36HJ5",
        }
        good = enrich_yfinance.Candidate("STR.VI", "EQUITY", "STR_AT_EQ", "ticker_derivation")
        base = enrich_yfinance.Candidate("STR", "EQUITY", "STR_AT_EQ", "ticker_derivation")

        self.assertTrue(enrich_yfinance.is_candidate_compatible(request, good, fundamentals))
        self.assertFalse(enrich_yfinance.is_candidate_compatible(request, base, fundamentals))

    def test_later_exchange_suffix_candidate_can_match_same_company(self) -> None:
        request = enrich_yfinance.Request(
            "BHP1d_EQ",
            "AU000000BHP4",
            "BHP Group",
            "STOCK",
            "EUR",
            ("BHP1.DE", "BHP1", "BHP.L", "BHP"),
        )
        fundamentals = {
            "quote_type": "EQUITY",
            "name": "BHP Group Limited",
            "sector": "Basic Materials",
            "matched_isin": None,
        }
        good = enrich_yfinance.Candidate("BHP.L", "EQUITY", "BHP1d_EQ", "ticker_derivation")
        base = enrich_yfinance.Candidate("BHP", "EQUITY", "BHP1d_EQ", "ticker_derivation")

        self.assertTrue(enrich_yfinance.is_candidate_compatible(request, good, fundamentals))
        self.assertFalse(enrich_yfinance.is_candidate_compatible(request, base, fundamentals))

    def test_name_matching_handles_diacritics_and_shared_distinctive_tokens(self) -> None:
        self.assertTrue(enrich_yfinance.names_are_plausible("Mayr Melnhof Karton", "Mayr-Melnhof Karton AG"))
        self.assertTrue(enrich_yfinance.names_are_plausible("Oesterreichische Post", "Österreichische Post AG"))
        self.assertTrue(enrich_yfinance.names_are_plausible("EVN", "EVN AG"))

    def test_persistent_rate_limiter_writes_state(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            limiter = enrich_yfinance.PersistentRateLimiter(
                root / "rate-limit.json",
                root / "rate-limit.lock",
                intervals={"lookup": 0},
            )
            limiter.wait_for_slot("lookup")
            payload = json.loads((root / "rate-limit.json").read_text(encoding="utf-8"))
            self.assertIn("lookup", payload["operations"])


class _NoopLimiter:
    def wait_for_slot(self, operation: str) -> None:
        return None


if __name__ == "__main__":
    unittest.main()
