#!/usr/bin/env python3
import argparse
import json
import sys
from pathlib import Path


def main() -> int:
    parser = argparse.ArgumentParser(description="Summarize and validate generated Statos site data.")
    parser.add_argument("--manifest", default="site/data/build_manifest.json")
    parser.add_argument("--require-live", action="store_true", help="fail unless generated data came from Trading 212")
    parser.add_argument("--min-instruments", type=int, default=14, help="minimum instrument count for --require-live")
    args = parser.parse_args()

    path = Path(args.manifest)
    if not path.exists():
        print(f"missing manifest: {path}", file=sys.stderr)
        return 1

    try:
        manifest = json.loads(path.read_text(encoding="utf-8"))
    except Exception as exc:
        print(f"invalid manifest JSON: {path}: {exc}", file=sys.stderr)
        return 1

    source_mode = manifest.get("sourceMode") or "unknown"
    environment = manifest.get("trading212Environment") or "unknown"
    enrichment_provider = manifest.get("enrichmentProvider") or "unknown"
    instrument_count = int(manifest.get("instrumentCount") or 0)
    company_count = int(manifest.get("companyCount") or 0)
    unclassified_count = int(manifest.get("unclassifiedCount") or 0)

    print(
        "site data:",
        f"sourceMode={source_mode}",
        f"environment={environment}",
        f"instruments={instrument_count}",
        f"companies={company_count}",
        f"unclassified={unclassified_count}",
        f"enrichmentProvider={enrichment_provider}",
    )

    if not args.require_live:
        return 0

    failures = []
    if source_mode != "live_fetch":
        failures.append(f"expected sourceMode=live_fetch, got {source_mode}")
    if environment not in {"demo", "live"}:
        failures.append(f"expected trading212Environment demo/live, got {environment}")
    if instrument_count < args.min_instruments:
        failures.append(f"expected at least {args.min_instruments} instruments, got {instrument_count}")

    if failures:
        print("live data check failed:", file=sys.stderr)
        for failure in failures:
            print(f"- {failure}", file=sys.stderr)
        print(
            "Check .env has TRADING212_API_KEY, TRADING212_API_SECRET, STATOS_SAMPLE=0, "
            "and the intended STATOS_TRADING212_ENV.",
            file=sys.stderr,
        )
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
