package export

import (
	"encoding/csv"
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
		Manifest:       catalogue.BuildManifest{BuiltAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)},
		Tickers:        []catalogue.Ticker{{Ticker: "ABC_US_EQ", Name: "ABC Corp", CompanyID: "abc", SecurityID: "isin:US0000000001", InstrumentCategory: catalogue.CategoryStock, IdentityConfidence: "rule_high"}},
		Securities:     []catalogue.Security{{ID: "isin:US0000000001", ISIN: "US0000000001", Name: "ABC Corp", CompanyID: "abc", TickerIDs: []string{"ABC_US_EQ"}, IdentityConfidence: "rule_high"}},
		Listings:       []catalogue.Listing{{ID: "ABC_US_EQ", Ticker: "ABC_US_EQ", CompanyID: "abc", SecurityID: "isin:US0000000001"}},
		Companies:      []catalogue.Company{{ID: "abc", Name: "ABC Corp", TickerIDs: []string{"ABC_US_EQ"}}},
		IdentityIssues: []catalogue.IdentityIssue{{IssueCode: "missing_isin", Ticker: "ABC_US_EQ", Reason: "test"}},
		EnrichmentFailures: []catalogue.EnrichmentFailure{{
			Ticker:           "ABC_US_EQ",
			ISIN:             "US0000000001",
			Name:             "ABC Corp",
			Provider:         "cache",
			AttemptedSymbols: "ABC;ABC_US_EQ",
			Status:           "cache_miss",
			Error:            "enrichment cache miss",
			NextAction:       "populate enrichment cache",
		}},
		Themes: []taxonomy.Theme{{ID: "ai", Name: "AI"}},
	}
	if err := WriteSiteData(dir, cat); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"catalogue.json", "tickers.csv", "securities.csv", "listings.csv", "identity_issues.csv", "enrichment_failures.csv", "build_manifest.json", "search_index.json", "unclassified.csv"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}

	file, err := os.Open(filepath.Join(dir, "enrichment_failures.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0][0] != "ticker" || records[1][0] != "ABC_US_EQ" || records[1][5] != "cache_miss" {
		t.Fatalf("enrichment_failures.csv records = %#v", records)
	}
}
