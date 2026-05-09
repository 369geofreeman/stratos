package export

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"statos/internal/catalogue"
	"statos/internal/taxonomy"
)

func TestWriteSiteData(t *testing.T) {
	dir := t.TempDir()
	cat := &catalogue.Catalogue{
		Manifest:  catalogue.BuildManifest{BuiltAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)},
		Tickers:   []catalogue.Ticker{{Ticker: "ABC_US_EQ", Name: "ABC Corp", CompanyID: "abc", SecurityID: "isin:US0000000001"}},
		Companies: []catalogue.Company{{ID: "abc", Name: "ABC Corp", TickerIDs: []string{"ABC_US_EQ"}}},
		Themes:    []taxonomy.Theme{{ID: "ai", Name: "AI"}},
	}
	if err := WriteSiteData(dir, cat); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"catalogue.json", "tickers.csv", "build_manifest.json", "search_index.json", "unclassified.csv"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}
}
