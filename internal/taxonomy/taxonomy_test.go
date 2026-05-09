package taxonomy

import (
	"os"
	"path/filepath"
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
}

func mustWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
