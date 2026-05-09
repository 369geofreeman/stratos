#!/usr/bin/env python3
import argparse
import json
from pathlib import Path


def is_rate_limited(entry: dict) -> bool:
    message = str(entry.get("error") or "").lower()
    return (
        entry.get("status") == "failure"
        and (
            "429 too many requests" in message
            or "rate limit" in message
            or "rate-limited" in message
        )
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="Clean selected ignored enrichment cache entries.")
    parser.add_argument("--cache-dir", default="data/cache/enrichment")
    parser.add_argument("--rate-limited", action="store_true", help="select provider rate-limit failures")
    parser.add_argument("--apply", action="store_true", help="delete matching files; default is dry-run")
    args = parser.parse_args()

    if not args.rate_limited:
        parser.error("choose a selector, for example --rate-limited")

    cache_dir = Path(args.cache_dir)
    if not cache_dir.exists():
        print(f"cache directory not found: {cache_dir}")
        return 0

    matched = []
    unreadable = 0
    for path in sorted(cache_dir.glob("*.json")):
        try:
            entry = json.loads(path.read_text(encoding="utf-8"))
        except Exception:
            unreadable += 1
            continue
        if args.rate_limited and is_rate_limited(entry):
            matched.append(path)

    if args.apply:
        for path in matched:
            path.unlink()
        action = "removed"
    else:
        action = "would remove"

    print(f"{action} {len(matched)} rate-limited enrichment cache entries")
    if unreadable:
        print(f"skipped {unreadable} unreadable cache files")
    if not args.apply and matched[:5]:
        print("sample matches:")
        for path in matched[:5]:
            print(path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
