package taxonomy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadThemesAndSupplyChains(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "themes.yml"), `themes:
  - id: ai
    name: AI
    description: Test theme
    color: "#123456"
`)
	mustWrite(t, filepath.Join(dir, "supply_chains.yml"), `supply_chains:
  - theme_id: ai
    name: AI chain
    description: Chain
    layers:
      - id: chips
        name: Chips
        description: Silicon
        order: 20
      - id: power
        name: Power
        description: Energy
        order: 10
`)
	mustWrite(t, filepath.Join(dir, "company_overrides.csv"), "company_id,name,sector,industry,country,source_url,last_reviewed\n")
	mustWrite(t, filepath.Join(dir, "ticker_overrides.csv"), "ticker,company_id,name,sector,industry,country,yahoo_symbol,source_url,last_reviewed\n")
	mustWrite(t, filepath.Join(dir, "identity_overrides.csv"), "target_type,ticker,isin,security_id,company_id,override_security_id,override_company_id,category,flags,confidence,reason,source_url,last_reviewed\nticker,ABC_US_EQ,,,,isin:US0000000001,abc,stock,adr;fund_like,manual_high,test,https://example.com,2026-05-09\n")
	mustWrite(t, filepath.Join(dir, "exposures.csv"), "theme_id,layer_id,ticker,isin,company_id,exposure_score,confidence,source_url,rationale,last_reviewed\nai,power,ABC_US_EQ,,abc,3,manual_high,https://example.com,Power exposure,2026-05-09\n")
	if err := os.Mkdir(filepath.Join(dir, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}

	data, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(data); err != nil {
		t.Fatal(err)
	}
	if len(data.Themes) != 1 || data.Themes[0].ID != "ai" {
		t.Fatalf("themes = %#v", data.Themes)
	}
	if got := data.SupplyChains[0].Layers[0].ID; got != "power" {
		t.Fatalf("layers were not sorted by order: got %q", got)
	}
	if len(data.Exposures) != 1 || data.Exposures[0].ExposureScore != 3 {
		t.Fatalf("exposures = %#v", data.Exposures)
	}
	if len(data.IdentityOverrides) != 1 || data.IdentityOverrides[0].OverrideCompanyID != "abc" || len(data.IdentityOverrides[0].Flags) != 2 {
		t.Fatalf("identity overrides = %#v", data.IdentityOverrides)
	}
}

func TestValidateIdentityOverrides(t *testing.T) {
	data := ManualData{
		IdentityOverrides: []IdentityOverride{{
			TargetType: "ticker",
			Ticker:     "ABC_US_EQ",
			Category:   "not_a_category",
		}},
	}
	if err := Validate(data); err == nil {
		t.Fatal("expected invalid identity override category to fail validation")
	}
}

func TestLoadTickerOverridesParsesManualEnrichmentFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ticker_overrides.csv")
	mustWrite(t, path, "ticker,company_id,name,sector,industry,country,yahoo_symbol,market_cap,exchange,currency,source_url,last_reviewed\nABC_US_EQ,abc,ABC Corp,Technology,Software,United States,ABC,12345,NASDAQ,USD,https://example.com,2026-05-09\n")
	overrides, err := LoadTickerOverrides(path)
	if err != nil {
		t.Fatal(err)
	}
	override := overrides["ABC_US_EQ"]
	if override.MarketCap != 12345 || override.Exchange != "NASDAQ" || override.Currency != "USD" {
		t.Fatalf("override = %#v", override)
	}
}

func TestLoadTickerOverridesRejectsMalformedMarketCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ticker_overrides.csv")
	mustWrite(t, path, "ticker,market_cap\nABC_US_EQ,12.5\n")
	_, err := LoadTickerOverrides(path)
	if err == nil || !strings.Contains(err.Error(), "row 2") || !strings.Contains(err.Error(), "market_cap") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadCompanyOverridesRejectsMissingCompanyID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "company_overrides.csv")
	mustWrite(t, path, "company_id,name,sector,industry,country,source_url,last_reviewed\n"+
		",ABC Corp,Technology,Software,United States,https://example.com,2026-05-09\n")
	_, err := LoadCompanyOverrides(path)
	if err == nil || !strings.Contains(err.Error(), "row 2") || !strings.Contains(err.Error(), "requires company_id") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadTickerOverridesRejectsMissingTicker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ticker_overrides.csv")
	mustWrite(t, path, "ticker,company_id,name,sector,industry,country,yahoo_symbol,market_cap,exchange,currency,source_url,last_reviewed\n"+
		",abc,ABC Corp,Technology,Software,United States,ABC,,,,https://example.com,2026-05-09\n")
	_, err := LoadTickerOverrides(path)
	if err == nil || !strings.Contains(err.Error(), "row 2") || !strings.Contains(err.Error(), "requires ticker") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadTickerOverridesRejectsUnknownColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ticker_overrides.csv")
	mustWrite(t, path, "ticker,unexpected\nABC_US_EQ,value\n")
	_, err := LoadTickerOverrides(path)
	if err == nil || !strings.Contains(err.Error(), "unknown column") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadTickerOverridesRejectsDuplicateHeaders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ticker_overrides.csv")
	mustWrite(t, path, "ticker,ticker\nABC_US_EQ,XYZ_US_EQ\n")
	_, err := LoadTickerOverrides(path)
	requireErrContains(t, err, "duplicate header")
}

func TestLoadThemesRejectsDuplicateIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "themes.yml")
	mustWrite(t, path, `themes:
  - id: ai
    name: AI
  - id: ai
    name: Duplicate
`)
	_, err := LoadThemes(path)
	requireErrContains(t, err, "duplicate theme id")
}

func TestLoadThemesRejectsMissingName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "themes.yml")
	mustWrite(t, path, `themes:
  - id: ai
    description: Missing name
`)
	_, err := LoadThemes(path)
	requireErrContains(t, err, "empty name")
}

func TestLoadThemesRejectsInvalidColor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "themes.yml")
	mustWrite(t, path, `themes:
  - id: ai
    name: AI
    color: blue
`)
	_, err := LoadThemes(path)
	requireErrContains(t, err, "invalid color")
}

func TestLoadThemesRejectsUnknownYAMLField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "themes.yml")
	mustWrite(t, path, `themes:
  - id: ai
    name: AI
    unexpected: value
`)
	_, err := LoadThemes(path)
	requireErrContains(t, err, "unknown theme field")
}

func TestLoadSupplyChainsRejectsMissingAndDuplicateLayerIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "supply_chains.yml")
	mustWrite(t, path, `supply_chains:
  - theme_id: ai
    name: AI chain
    layers:
      - name: Missing ID
        order: 10
`)
	_, err := LoadSupplyChains(path)
	requireErrContains(t, err, "empty id")

	mustWrite(t, path, `supply_chains:
  - theme_id: ai
    name: AI chain
    layers:
      - id: chips
        name: Chips
        order: 10
      - id: chips
        name: Duplicate
        order: 20
`)
	_, err = LoadSupplyChains(path)
	requireErrContains(t, err, "duplicate layer id")
}

func TestLoadSupplyChainsRejectsMalformedLayerOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "supply_chains.yml")
	mustWrite(t, path, `supply_chains:
  - theme_id: ai
    name: AI chain
    layers:
      - id: chips
        name: Chips
        order: early
`)
	_, err := LoadSupplyChains(path)
	requireErrContains(t, err, "not an integer")
}

func TestLoadExposuresRejectsMalformedScore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exposures.csv")
	mustWrite(t, path, exposureHeader()+"ai,chips,ABC_US_EQ,,,not-a-number,manual_high,https://example.com,test,2026-05-09\n")
	_, err := LoadExposures(path)
	requireErrContains(t, err, "malformed exposure_score")
}

func TestLoadExposuresRejectsScoreOutOfRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exposures.csv")
	mustWrite(t, path, exposureHeader()+"ai,chips,ABC_US_EQ,,,6,manual_high,https://example.com,test,2026-05-09\n")
	_, err := LoadExposures(path)
	requireErrContains(t, err, "outside 0..5")
}

func TestLoadExposuresRejectsInvalidConfidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exposures.csv")
	mustWrite(t, path, exposureHeader()+"ai,chips,ABC_US_EQ,,,3,guess,https://example.com,test,2026-05-09\n")
	_, err := LoadExposures(path)
	requireErrContains(t, err, "unknown confidence")
}

func TestLoadExposuresRejectsMissingTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exposures.csv")
	mustWrite(t, path, exposureHeader()+"ai,chips,,,,3,manual_high,https://example.com,test,2026-05-09\n")
	_, err := LoadExposures(path)
	requireErrContains(t, err, "requires at least one")
}

func TestLoadExposuresRejectsBadDateAndURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exposures.csv")
	mustWrite(t, path, exposureHeader()+"ai,chips,ABC_US_EQ,,,3,manual_high,not-a-url,test,2026-05-09\n")
	_, err := LoadExposures(path)
	requireErrContains(t, err, "invalid source_url")

	mustWrite(t, path, exposureHeader()+"ai,chips,ABC_US_EQ,,,3,manual_high,https://example.com,test,09/05/2026\n")
	_, err = LoadExposures(path)
	requireErrContains(t, err, "invalid last_reviewed")
}

func TestLoadClassificationOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "classification_overrides.csv")
	mustWrite(t, path, "target_type,ticker,isin,company_id,sector,industry,country,source_url,last_reviewed\n"+
		"ticker,ABC_US_EQ,,,Manual Sector,Manual Industry,Manual Country,https://example.com,2026-05-09\n")
	overrides, err := LoadClassificationOverrides(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 1 || overrides[0].Ticker != "ABC_US_EQ" || overrides[0].Sector != "Manual Sector" {
		t.Fatalf("overrides = %#v", overrides)
	}
}

func TestLoadClassificationOverridesRejectsInvalidTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "classification_overrides.csv")
	mustWrite(t, path, "target_type,ticker,isin,company_id,sector,industry,country,source_url,last_reviewed\n"+
		"ticker,ABC_US_EQ,US0000000001,,Manual Sector,,,https://example.com,2026-05-09\n")
	_, err := LoadClassificationOverrides(path)
	requireErrContains(t, err, "must not set isin")
}

func TestLoadRelationshipsValidatesRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relationships.csv")
	mustWrite(t, path, "relationship_type,source_ticker,source_isin,source_company_id,target_ticker,target_isin,target_company_id,theme_id,layer_id,confidence,source_url,rationale,last_reviewed\n"+
		"peer,ABC_US_EQ,,,XYZ_US_EQ,,,ai,chips,manual_medium,https://example.com,Peers,2026-05-09\n")
	rows, err := LoadRelationships(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].RelationshipType != "peer" {
		t.Fatalf("relationships = %#v", rows)
	}

	mustWrite(t, path, "relationship_type,source_ticker,source_isin,source_company_id,target_ticker,target_isin,target_company_id,theme_id,layer_id,confidence,source_url,rationale,last_reviewed\n"+
		"peer,ABC_US_EQ,US0000000001,,XYZ_US_EQ,,,ai,chips,manual_medium,https://example.com,Peers,2026-05-09\n")
	_, err = LoadRelationships(path)
	requireErrContains(t, err, "exactly one source")
}

func TestLoadNotesValidatesFrontmatter(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "good.md"), `---
target_type: ticker
target_id: ABC_US_EQ
title: ABC note
tags: b, a, b
---

Body.
`)
	notes, err := LoadNotes(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || strings.Join(notes[0].Tags, ",") != "a,b" {
		t.Fatalf("notes = %#v", notes)
	}

	mustWrite(t, filepath.Join(dir, "bad.md"), `---
target_type: ticker
target_id: ABC_US_EQ
title: Bad note
unexpected: value
---

Body.
`)
	_, err = LoadNotes(dir)
	requireErrContains(t, err, "unknown note frontmatter key")
}

func exposureHeader() string {
	return "theme_id,layer_id,ticker,isin,company_id,exposure_score,confidence,source_url,rationale,last_reviewed\n"
}

func requireErrContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %v, want substring %q", err, want)
	}
}

func mustWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
