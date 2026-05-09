#!/bin/sh
set -eu

expected_files="
site/data/catalogue.json
site/data/tickers.csv
site/data/companies.json
site/data/sectors.json
site/data/industries.json
site/data/themes.json
site/data/supply_chains.json
site/data/search_index.json
site/data/unclassified.csv
site/data/identity_issues.csv
site/data/enrichment_failures.csv
site/data/securities.csv
site/data/listings.csv
site/data/build_manifest.json
"

missing=0
for path in $expected_files; do
	if [ ! -f "$path" ]; then
		printf 'missing generated file: %s\n' "$path" >&2
		missing=1
	fi
done

if [ "$missing" -ne 0 ]; then
	exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
	printf 'python3 is required for JSON smoke validation\n' >&2
	exit 1
fi

python3 - <<'PY'
import json
import sys

ok = True
for path in ("site/data/catalogue.json", "site/data/build_manifest.json"):
    try:
        with open(path, "r", encoding="utf-8") as handle:
            json.load(handle)
    except Exception as exc:
        print(f"invalid JSON: {path}: {exc}", file=sys.stderr)
        ok = False

if not ok:
    sys.exit(1)
PY

printf 'smoke ok: generated site/data files are present and key JSON files parse\n'
