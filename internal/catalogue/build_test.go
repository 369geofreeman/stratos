package catalogue

import (
	"testing"
	"time"

	"statos/internal/enrichment"
	"statos/internal/taxonomy"
	"statos/internal/trading212"
)

func TestDetectDirectionality(t *testing.T) {
	tests := map[string]string{
		"UltraPro Short QQQ -3X Daily ETF": "inverse_or_short",
		"Daily 2X Long Tesla":              "leveraged_long",
		"Apple Inc":                        "long_or_unlevered",
	}
	for input, want := range tests {
		if got := DetectDirectionality(input); got != want {
			t.Fatalf("DetectDirectionality(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBuildGroupsByISINAndAppliesExposure(t *testing.T) {
	manual := taxonomy.ManualData{
		Themes: []taxonomy.Theme{{ID: "ai", Name: "AI"}},
		SupplyChains: []taxonomy.SupplyChain{{
			ThemeID: "ai",
			Name:    "AI chain",
			Layers:  []taxonomy.SupplyChainLayer{{ID: "chips", Name: "Chips", Order: 10}},
		}},
		Exposures: []taxonomy.Exposure{{ThemeID: "ai", LayerID: "chips", Ticker: "ABC_US_EQ", CompanyID: "abc", ExposureScore: 4}},
		TickerOverrides: map[string]taxonomy.TickerOverride{
			"ABC_US_EQ": {Ticker: "ABC_US_EQ", CompanyID: "abc", Sector: "Technology", Industry: "Semiconductors"},
		},
		CompanyOverrides: map[string]taxonomy.CompanyOverride{},
	}
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "ABC_US_EQ", Name: "ABC Corp", ISIN: "US0000000001", Type: "STOCK", CurrencyCode: "USD"},
			{Ticker: "ABC_L_EQ", Name: "ABC Corp", ISIN: "US0000000001", Type: "STOCK", CurrencyCode: "GBP"},
		},
		Profiles: map[string]enrichment.Profile{},
		Manual:   manual,
		BuiltAt:  time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Securities) != 1 {
		t.Fatalf("securities = %#v", cat.Securities)
	}
	if len(cat.Companies) != 1 || cat.Companies[0].ID != "abc" {
		t.Fatalf("companies = %#v", cat.Companies)
	}
	if len(cat.Tickers[0].ThemeIDs) == 0 {
		t.Fatalf("exposure was not applied to ticker: %#v", cat.Tickers[0])
	}
}
