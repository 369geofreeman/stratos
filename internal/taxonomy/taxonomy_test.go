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

func TestLoadTickerOverridesRejectsUnknownColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ticker_overrides.csv")
	mustWrite(t, path, "ticker,unexpected\nABC_US_EQ,value\n")
	_, err := LoadTickerOverrides(path)
	if err == nil || !strings.Contains(err.Error(), "unknown ticker override column") {
		t.Fatalf("err = %v", err)
	}
}

func mustWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
