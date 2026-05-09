package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"statos/internal/catalogue"
	"statos/internal/taxonomy"
)

func TestReadRawSnapshotsMissingLatestFailsClearly(t *testing.T) {
	_, _, _, err := readRawSnapshots(t.TempDir())
	if err == nil {
		t.Fatal("expected missing raw replay error")
	}
	if !strings.Contains(err.Error(), "raw replay requested") || !strings.Contains(err.Error(), "instruments_latest.json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadRawSnapshotsFromLatestAliases(t *testing.T) {
	dir := t.TempDir()
	instruments, exchanges, _ := catalogue.SampleData()
	builtAt := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	written, err := writeRawSnapshots(dir, builtAt, instruments, exchanges)
	if err != nil {
		t.Fatal(err)
	}

	gotInstruments, gotExchanges, replayed, err := readRawSnapshots(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotInstruments) != len(instruments) || len(gotExchanges) != len(exchanges) {
		t.Fatalf("replayed counts = %d instruments/%d exchanges, want %d/%d", len(gotInstruments), len(gotExchanges), len(instruments), len(exchanges))
	}
	if replayed.Timestamp != written.Timestamp {
		t.Fatalf("replayed timestamp = %q, want %q", replayed.Timestamp, written.Timestamp)
	}
	if replayed.InstrumentsLatest == "" || replayed.ExchangesLatest == "" {
		t.Fatalf("latest aliases missing from replay summary: %#v", replayed)
	}
}

func TestRunNoFetchUsesRawSnapshotTimestamp(t *testing.T) {
	rawDir := t.TempDir()
	siteDataDir := t.TempDir()
	cacheDir := t.TempDir()
	instruments, exchanges, _ := catalogue.SampleData()
	builtAt := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	if _, err := writeRawSnapshots(rawDir, builtAt, instruments, exchanges); err != nil {
		t.Fatal(err)
	}

	err := run([]string{
		"refresh",
		"--no-fetch",
		"--input-raw-dir", rawDir,
		"--raw-dir", rawDir,
		"--site-data-dir", siteDataDir,
		"--manual-dir", filepath.Join("..", "..", "data", "manual"),
		"--cache-dir", cacheDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(siteDataDir, "build_manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		BuiltAt    string `json:"builtAt"`
		SourceMode string `json:"sourceMode"`
	}
	if err := json.Unmarshal(b, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.BuiltAt != "2026-05-09T12:00:00Z" || manifest.SourceMode != "raw_replay" {
		t.Fatalf("manifest = %#v, want deterministic raw replay timestamp/source", manifest)
	}
}

func TestTaxonomyCoverageCommandOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalogue.json")
	cat := catalogue.Catalogue{
		Tickers: []catalogue.Ticker{
			{Ticker: "ABC_US_EQ", CompanyID: "abc", Sector: "Technology", Industry: "Semiconductors", ThemeIDs: []string{"ai"}, LayerIDs: []string{"chips"}},
			{Ticker: "XYZ_US_EQ", CompanyID: "xyz", Sector: "", Industry: "", Unclassified: true},
		},
		Companies: []catalogue.Company{
			{ID: "abc", ThemeIDs: []string{"ai"}, LayerIDs: []string{"chips"}},
			{ID: "xyz"},
		},
		Themes: []taxonomy.Theme{{ID: "ai", Name: "AI"}},
		SupplyChains: []taxonomy.SupplyChain{{
			ThemeID: "ai",
			Name:    "AI chain",
			Layers: []taxonomy.SupplyChainLayer{
				{ID: "chips", Name: "Chips", Order: 10},
				{ID: "cloud", Name: "Cloud", Order: 20},
			},
		}},
		Exposures:    []taxonomy.Exposure{{ThemeID: "ai", LayerID: "chips", Ticker: "ABC_US_EQ", Confidence: "manual_high"}},
		Sectors:      []catalogue.GroupCount{{Name: "Technology", Count: 1}, {Name: "Unclassified", Count: 1}},
		Industries:   []catalogue.GroupCount{{Name: "Semiconductors", Count: 1}, {Name: "Unclassified", Count: 1}},
		Unclassified: []catalogue.UnclassifiedRow{{Ticker: "XYZ_US_EQ"}},
	}
	writeJSONFile(t, path, cat)

	var out bytes.Buffer
	if err := runTaxonomy([]string{"coverage", "--catalogue", path}, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"theme_id\ttheme_name\texposed_tickers\texposed_companies\tcovered_layers\ttotal_layers",
		"ai\tAI\t1\t1\t1\t2",
		"ai\tchips\tChips\t1\tmanual_high=1",
		"Technology\t1",
		"unclassified_count\n1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("coverage output missing %q:\n%s", want, text)
		}
	}
}

func TestTaxonomyExposureTemplateCommandOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unclassified.csv")
	mustWriteFile(t, path, "ticker,company_id,name,isin,reason\n"+
		"XYZ_US_EQ,xyz,XYZ Corp,US0000000002,missing theme exposure\n"+
		"ABC_US_EQ,abc,ABC Corp,US0000000001,missing theme exposure\n")

	var out bytes.Buffer
	if err := runTaxonomy([]string{"exposure-template", "--unclassified", path}, &out); err != nil {
		t.Fatal(err)
	}
	want := "theme_id,layer_id,ticker,isin,company_id,exposure_score,confidence,source_url,rationale,last_reviewed\n" +
		",,ABC_US_EQ,US0000000001,abc,,,,,\n" +
		",,XYZ_US_EQ,US0000000002,xyz,,,,,\n"
	if out.String() != want {
		t.Fatalf("template output = %q, want %q", out.String(), want)
	}
}

func TestManualValidationFailureDoesNotWriteGeneratedOutputs(t *testing.T) {
	manualDir := t.TempDir()
	siteDataDir := filepath.Join(t.TempDir(), "site-data")
	rawDir := t.TempDir()
	cacheDir := t.TempDir()
	writeBadManual(t, manualDir)

	err := run([]string{
		"sample",
		"--manual-dir", manualDir,
		"--site-data-dir", siteDataDir,
		"--raw-dir", rawDir,
		"--cache-dir", cacheDir,
	})
	if err == nil || !strings.Contains(err.Error(), "malformed exposure_score") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(siteDataDir, "build_manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("generated output was written despite manual validation failure: %v", err)
	}
}

func TestParseOptionalDuration(t *testing.T) {
	got, err := parseOptionalDuration("1500ms")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1500*time.Millisecond {
		t.Fatalf("duration = %s", got)
	}
	got, err = parseOptionalDuration("")
	if err != nil || got != 0 {
		t.Fatalf("blank duration = %s, err = %v", got, err)
	}
	if _, err := parseOptionalDuration("slow"); err == nil {
		t.Fatal("expected invalid duration error")
	}
}

func writeBadManual(t *testing.T, dir string) {
	t.Helper()
	mustWriteFile(t, filepath.Join(dir, "themes.yml"), `themes:
  - id: ai
    name: AI
`)
	mustWriteFile(t, filepath.Join(dir, "supply_chains.yml"), `supply_chains:
  - theme_id: ai
    name: AI chain
    layers:
      - id: chips
        name: Chips
        order: 10
`)
	mustWriteFile(t, filepath.Join(dir, "company_overrides.csv"), "company_id,name,sector,industry,country,source_url,last_reviewed\n")
	mustWriteFile(t, filepath.Join(dir, "ticker_overrides.csv"), "ticker,company_id,name,sector,industry,country,yahoo_symbol,market_cap,exchange,currency,source_url,last_reviewed\n")
	mustWriteFile(t, filepath.Join(dir, "classification_overrides.csv"), "target_type,ticker,isin,company_id,sector,industry,country,source_url,last_reviewed\n")
	mustWriteFile(t, filepath.Join(dir, "identity_overrides.csv"), "target_type,ticker,isin,security_id,company_id,override_security_id,override_company_id,category,flags,confidence,reason,source_url,last_reviewed\n")
	mustWriteFile(t, filepath.Join(dir, "relationships.csv"), "relationship_type,source_ticker,source_isin,source_company_id,target_ticker,target_isin,target_company_id,theme_id,layer_id,confidence,source_url,rationale,last_reviewed\n")
	mustWriteFile(t, filepath.Join(dir, "exposures.csv"), "theme_id,layer_id,ticker,isin,company_id,exposure_score,confidence,source_url,rationale,last_reviewed\n"+
		"ai,chips,ABC_US_EQ,,,bad,manual_high,https://example.com,test,2026-05-09\n")
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, path, string(b))
}

func mustWriteFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
