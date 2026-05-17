package catalogue

import (
	"fmt"
	"testing"
	"time"

	"statos/internal/enrichment"
	"statos/internal/taxonomy"
	"statos/internal/trading212"
)

func TestParseBrokerTickerTrading212Patterns(t *testing.T) {
	tests := []struct {
		ticker       string
		symbol       string
		exchangeCode string
		assetCode    string
		uncertain    bool
	}{
		{ticker: "NVDA_US_EQ", symbol: "NVDA", exchangeCode: "US", assetCode: "EQ"},
		{ticker: "3SQQQ_US_EQ", symbol: "3SQQQ", exchangeCode: "US", assetCode: "EQ"},
		{ticker: "BRK.B_US_EQ", symbol: "BRK.B", exchangeCode: "US", assetCode: "EQ"},
		{ticker: "VUSA_L_ETF", symbol: "VUSA", exchangeCode: "L", assetCode: "ETF"},
		{ticker: "BHP1d_EQ", symbol: "BHP1", assetCode: "EQ"},
		{ticker: "SANTd_EQ", symbol: "SANT", assetCode: "EQ"},
		{ticker: "ABBNsEQ", symbol: "ABBN", assetCode: "EQ"},
		{ticker: "AVAV__US_EQ", symbol: "AVAV", exchangeCode: "US", assetCode: "EQ"},
		{ticker: "ELF_EQ_US", symbol: "ELF", exchangeCode: "US", assetCode: "EQ"},
		{ticker: "YUMC", symbol: "YUMC"},
		{ticker: "ABC_US", symbol: "ABC", exchangeCode: "US", uncertain: true},
		{ticker: "RAW-TICKER", symbol: "RAW-TICKER", uncertain: true},
	}
	for _, tt := range tests {
		got := ParseBrokerTicker(tt.ticker)
		if got.Symbol != tt.symbol || got.ExchangeCode != tt.exchangeCode || got.AssetCode != tt.assetCode || got.Uncertain != tt.uncertain {
			t.Fatalf("ParseBrokerTicker(%q) = %#v", tt.ticker, got)
		}
	}
}

func TestBuildISINBackedFundLikeIdentityStaysReviewableButNotLowConfidence(t *testing.T) {
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "VUSA_L_EQ", Name: "Vanguard S&P 500 UCITS ETF Dist", ISIN: "IE00B3XXRP09", Type: "ETF", CurrencyCode: "GBP"},
		},
		Manual:  emptyManual(),
		BuiltAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ticker := findTicker(t, cat, "VUSA_L_EQ")
	if ticker.IdentityConfidence != "rule_medium" {
		t.Fatalf("ticker identity confidence = %q, want rule_medium: %#v", ticker.IdentityConfidence, ticker)
	}
	if hasIdentityIssue(cat, "low_confidence_company_identity") {
		t.Fatalf("ISIN-backed fund-like identity should not require low-confidence company review: %#v", cat.IdentityIssues)
	}
	if !containsString(ticker.IdentityReasons, "fund_like_company_identity_from_isin") {
		t.Fatalf("identity reasons = %#v, want fund_like_company_identity_from_isin", ticker.IdentityReasons)
	}
}

func TestDetectDirectionality(t *testing.T) {
	tests := map[string]string{
		"UltraPro Short QQQ -3X Daily ETF": "inverse_or_short",
		"Daily 2X Long Tesla":              "leveraged_long",
		"3SQQQ_US_EQ":                      "inverse_or_short",
		"Apple Inc":                        "long_or_unlevered",
	}
	for input, want := range tests {
		if got := DetectDirectionality(input); got != want {
			t.Fatalf("DetectDirectionality(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDetectStructureFlags(t *testing.T) {
	flags := DetectStructureFlags(CategoryETF, "3SQQQ_US_EQ", "UltraPro Short QQQ -3X Daily ETF")
	for _, want := range []string{"inverse", "short", "leveraged", "fund_like"} {
		if !containsString(flags, want) {
			t.Fatalf("flags = %#v, want %q", flags, want)
		}
	}
	flags = DetectStructureFlags(CategoryETF, "IUSE_L_EQ", "iShares S&P 500 GBP Hedged Acc UCITS ETF")
	for _, want := range []string{"hedged", "accumulating", "fund_like"} {
		if !containsString(flags, want) {
			t.Fatalf("flags = %#v, want %q", flags, want)
		}
	}
	flags = DetectStructureFlags(CategoryETF, "VUSA_L_EQ", "Vanguard S&P 500 UCITS ETF Dist")
	if !containsString(flags, "distributing") {
		t.Fatalf("flags = %#v, want distributing", flags)
	}
	flags = DetectStructureFlags(CategoryStock, "TSM_US_EQ", "Taiwan Semiconductor Manufacturing Company Limited ADR")
	if !containsString(flags, "adr") {
		t.Fatalf("flags = %#v, want adr", flags)
	}
	flags = DetectStructureFlags(CategoryStock, "ABC_L_EQ", "ABC Global Depositary Receipt GDR")
	if !containsString(flags, "gdr") {
		t.Fatalf("flags = %#v, want gdr", flags)
	}
	flags = DetectStructureFlags(CategoryStock, "ADS_US_EQ", "Bread Financial")
	if containsString(flags, "adr") {
		t.Fatalf("ticker symbol alone should not create adr flag: %#v", flags)
	}
	flags = DetectStructureFlags(CategoryETF, "CEBBd_EQ", "iShares MSCI Russia ADR/GDR (Acc)")
	if containsString(flags, "adr") || containsString(flags, "gdr") {
		t.Fatalf("ETF underlying ADR/GDR wording should not create depositary receipt flags: %#v", flags)
	}
}

func TestClassifyInstrumentCategory(t *testing.T) {
	tests := []struct {
		raw  trading212.Instrument
		want string
	}{
		{raw: trading212.Instrument{Ticker: "ABC_US_EQ", Name: "ABC Corp", Type: "STOCK"}, want: CategoryStock},
		{raw: trading212.Instrument{Ticker: "VUSA_L_EQ", Name: "Vanguard S&P 500 UCITS ETF", Type: "ETF"}, want: CategoryETF},
		{raw: trading212.Instrument{Ticker: "SMT_L_EQ", Name: "Scottish Mortgage Investment Trust plc", Type: "STOCK"}, want: CategoryInvestmentTrust},
		{raw: trading212.Instrument{Ticker: "BTC_US_EQ", Name: "Bitcoin Tracker", Type: ""}, want: CategoryCrypto},
		{raw: trading212.Instrument{Ticker: "UNK_US_EQ", Name: "Unclear Instrument", Type: "CERTIFICATE"}, want: CategoryOther},
	}
	for _, tt := range tests {
		got, _ := ClassifyInstrumentCategory(tt.raw)
		if got != tt.want {
			t.Fatalf("ClassifyInstrumentCategory(%#v) = %q, want %q", tt.raw, got, tt.want)
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
		Profiles:              map[string]enrichment.Profile{},
		Manual:                manual,
		BuiltAt:               time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
		SourceMode:            "live_fetch",
		Trading212Environment: "demo",
		Trading212BaseURL:     trading212.DemoBaseURL,
		Trading212FetchAt:     "2026-05-09T12:00:00Z",
		RawSnapshotAt:         "2026-05-09T12:00:00Z",
		RawSnapshots: RawSnapshotSummary{
			Timestamp:         "2026-05-09T12:00:00Z",
			InstrumentsLatest: "data/raw/trading212/instruments_latest.json",
			ExchangesLatest:   "data/raw/trading212/exchanges_latest.json",
		},
		HTTPDiagnostics: []trading212.EndpointDiagnostic{{
			EndpointName: "instruments",
			Path:         "/equity/metadata/instruments",
			StatusCode:   200,
			RateLimit:    trading212.RateLimitHeaders{Limit: "1", Period: "50", Remaining: "0"},
		}},
		RateLimits: []trading212.RateLimitObservation{{
			EndpointName: "instruments",
			Path:         "/equity/metadata/instruments",
			Limit:        "1",
			Period:       "50",
			Remaining:    "0",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Securities) != 1 {
		t.Fatalf("securities = %#v", cat.Securities)
	}
	if got := cat.Securities[0].TickerIDs; len(got) != 2 || got[0] != "ABC_L_EQ" || got[1] != "ABC_US_EQ" {
		t.Fatalf("security ticker ids = %#v", got)
	}
	if cat.Manifest.DuplicateISINCount != 1 {
		t.Fatalf("DuplicateISINCount = %d, want 1", cat.Manifest.DuplicateISINCount)
	}
	if len(cat.Companies) != 1 || cat.Companies[0].ID != "abc" {
		t.Fatalf("companies = %#v", cat.Companies)
	}
	if len(cat.Tickers[0].ThemeIDs) == 0 {
		t.Fatalf("exposure was not applied to ticker: %#v", cat.Tickers[0])
	}
	if cat.Manifest.SourceMode != "live_fetch" || cat.Manifest.Trading212BaseURL != trading212.DemoBaseURL || len(cat.Manifest.Trading212HTTPDiagnostics) != 1 || len(cat.Manifest.Trading212RateLimits) != 1 {
		t.Fatalf("manifest did not preserve Trading 212 diagnostics: %#v", cat.Manifest)
	}
}

func TestBuildMissingISINFallbackIdentity(t *testing.T) {
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "NOISIN_US_EQ", Name: "No ISIN plc", Type: "STOCK", CurrencyCode: "USD"},
		},
		Manual:  emptyManual(),
		BuiltAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ticker := findTicker(t, cat, "NOISIN_US_EQ")
	if ticker.SecurityID != "ticker:NOISIN_US_EQ" || ticker.IdentityConfidence != "rule_low" {
		t.Fatalf("ticker identity = %#v", ticker)
	}
	if cat.Manifest.MissingISINCount != 1 || !hasIdentityIssue(cat, "missing_isin") {
		t.Fatalf("manifest/issues did not report missing isin: %#v %#v", cat.Manifest, cat.IdentityIssues)
	}
}

func TestBuildManualIdentityOverrideMergeAndSplit(t *testing.T) {
	manual := emptyManual()
	manual.TickerOverrides = map[string]taxonomy.TickerOverride{
		"ACME_US_EQ": {Ticker: "ACME_US_EQ", CompanyID: "acme"},
		"BETA_US_EQ": {Ticker: "BETA_US_EQ", CompanyID: "beta"},
	}
	manual.IdentityOverrides = []taxonomy.IdentityOverride{
		{
			TargetType:         "ticker",
			Ticker:             "ACME_L_EQ",
			OverrideSecurityID: "isin:US0000000001",
			OverrideCompanyID:  "acme",
			Confidence:         "manual_high",
			Reason:             "same line listed without isin",
		},
		{
			TargetType:         "ticker",
			Ticker:             "BETA_L_EQ",
			OverrideSecurityID: "isin:GB0000000002:split",
			OverrideCompanyID:  "beta_london_line",
			Category:           CategoryInvestmentTrust,
			Flags:              []string{"fund_like"},
			Confidence:         "manual_high",
			Reason:             "shared isin is misleading",
		},
	}
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "ACME_US_EQ", Name: "Acme Corp", ISIN: "US0000000001", Type: "STOCK", CurrencyCode: "USD"},
			{Ticker: "ACME_L_EQ", Name: "Acme Corp London", Type: "STOCK", CurrencyCode: "GBP"},
			{Ticker: "BETA_US_EQ", Name: "Beta Corp", ISIN: "GB0000000002", Type: "STOCK", CurrencyCode: "USD"},
			{Ticker: "BETA_L_EQ", Name: "Beta Investment Trust", ISIN: "GB0000000002", Type: "STOCK", CurrencyCode: "GBP"},
		},
		Manual:  manual,
		BuiltAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	acmeLondon := findTicker(t, cat, "ACME_L_EQ")
	if acmeLondon.SecurityID != "isin:US0000000001" || acmeLondon.CompanyID != "acme" || acmeLondon.IdentityConfidence != "manual_high" {
		t.Fatalf("manual merge failed: %#v", acmeLondon)
	}
	betaLondon := findTicker(t, cat, "BETA_L_EQ")
	if betaLondon.SecurityID != "isin:GB0000000002:split" || betaLondon.CompanyID != "beta_london_line" || betaLondon.InstrumentCategory != CategoryInvestmentTrust || !containsString(betaLondon.StructureFlags, "fund_like") {
		t.Fatalf("manual split failed: %#v", betaLondon)
	}
	if cat.Manifest.IdentityOverrideCount != 2 {
		t.Fatalf("IdentityOverrideCount = %d, want 2", cat.Manifest.IdentityOverrideCount)
	}
}

func TestBuildCompanyOwnsMultipleSecuritiesListingsAndTickers(t *testing.T) {
	manual := emptyManual()
	manual.TickerOverrides = map[string]taxonomy.TickerOverride{
		"ONE_US_EQ": {Ticker: "ONE_US_EQ", CompanyID: "multi"},
		"ONE_L_EQ":  {Ticker: "ONE_L_EQ", CompanyID: "multi"},
		"TWO_US_EQ": {Ticker: "TWO_US_EQ", CompanyID: "multi"},
	}
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "ONE_US_EQ", Name: "Multi Corp Ordinary", ISIN: "US0000000001", Type: "STOCK", CurrencyCode: "USD"},
			{Ticker: "ONE_L_EQ", Name: "Multi Corp Ordinary", ISIN: "US0000000001", Type: "STOCK", CurrencyCode: "GBP"},
			{Ticker: "TWO_US_EQ", Name: "Multi Corp Preferred", ISIN: "US0000000002", Type: "STOCK", CurrencyCode: "USD"},
		},
		Manual:  manual,
		BuiltAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Companies) != 1 || cat.Companies[0].ID != "multi" {
		t.Fatalf("companies = %#v", cat.Companies)
	}
	if len(cat.Companies[0].SecurityIDs) != 2 || len(cat.Companies[0].ListingIDs) != 3 || len(cat.Companies[0].TickerIDs) != 3 {
		t.Fatalf("company relationships = %#v", cat.Companies[0])
	}
}

func TestBuildIdentityIssuesAndManifestCounts(t *testing.T) {
	manual := emptyManual()
	manual.TickerOverrides = map[string]taxonomy.TickerOverride{
		"COLLA_US_EQ": {Ticker: "COLLA_US_EQ", CompanyID: "company_a"},
		"COLLB_US_EQ": {Ticker: "COLLB_US_EQ", CompanyID: "company_b"},
	}
	manual.IdentityOverrides = []taxonomy.IdentityOverride{
		{TargetType: "ticker", Ticker: "MISSING_US_EQ", Category: CategoryStock},
	}
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "", Name: "Empty ticker", ISIN: "GB0000000001", Type: "STOCK"},
			{Ticker: "DUP_US_EQ", Name: "Duplicate one", ISIN: "US0000000001", Type: "STOCK"},
			{Ticker: "DUP_US_EQ", Name: "Duplicate two", ISIN: "US0000000002", Type: "STOCK"},
			{Ticker: "NOISIN_US_EQ", Name: "No ISIN", Type: "CERTIFICATE"},
			{Ticker: "COLLA_US_EQ", Name: "Collision A", ISIN: "US0000000003", Type: "STOCK"},
			{Ticker: "COLLB_US_EQ", Name: "Collision B", ISIN: "US0000000003", Type: "STOCK"},
		},
		Manual:  manual,
		BuiltAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if cat.Manifest.EmptyTickerCount != 1 || cat.Manifest.DuplicateTickerCount != 1 || cat.Manifest.MissingISINCount != 1 || cat.Manifest.DuplicateISINCount != 1 {
		t.Fatalf("identity manifest counts = %#v", cat.Manifest)
	}
	if cat.Manifest.InstrumentCategoryCounts[CategoryStock] != 3 || cat.Manifest.InstrumentCategoryCounts[CategoryOther] != 1 {
		t.Fatalf("category counts = %#v", cat.Manifest.InstrumentCategoryCounts)
	}
	for _, code := range []string{"missing_ticker", "duplicate_ticker", "missing_isin", "unknown_instrument_category", "shared_isin_multiple_companies", "manual_override_unknown_ticker"} {
		if !hasIdentityIssue(cat, code) {
			t.Fatalf("identity issues missing %q: %#v", code, cat.IdentityIssues)
		}
	}
	if cat.Manifest.IdentityCollisionCount == 0 || cat.Manifest.IdentityIssueCount != len(cat.IdentityIssues) {
		t.Fatalf("identity issue manifest fields = %#v issues=%#v", cat.Manifest, cat.IdentityIssues)
	}
}

func TestBuildManualEnrichmentOverridePrecedence(t *testing.T) {
	manual := emptyManual()
	manual.TickerOverrides = map[string]taxonomy.TickerOverride{
		"ABC_US_EQ": {
			Ticker:      "ABC_US_EQ",
			CompanyID:   "abc",
			Sector:      "Manual Sector",
			Industry:    "Manual Industry",
			Country:     "Manual Country",
			YahooSymbol: "MANUAL",
			MarketCap:   222,
			Exchange:    "Manual Exchange",
			Currency:    "GBP",
		},
	}
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "ABC_US_EQ", Name: "ABC Corp", ISIN: "US0000000001", Type: "STOCK", CurrencyCode: "USD", WorkingScheduleID: 1},
		},
		Exchanges: []trading212.Exchange{{ID: 1, Name: "Broker Exchange"}},
		Profiles: map[string]enrichment.Profile{
			"ABC_US_EQ": {Symbol: "PROVIDER", Sector: "Provider Sector", Industry: "Provider Industry", Country: "Provider Country", Exchange: "Provider Exchange", Currency: "EUR", MarketCap: 111, Source: "provider"},
		},
		Manual:  manual,
		BuiltAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ticker := findTicker(t, cat, "ABC_US_EQ")
	if ticker.Sector != "Manual Sector" || ticker.Industry != "Manual Industry" || ticker.Country != "Manual Country" || ticker.YahooSymbol != "MANUAL" {
		t.Fatalf("manual classification fields did not win: %#v", ticker)
	}
	if ticker.MarketCap != 222 || ticker.ExchangeName != "Manual Exchange" || ticker.CurrencyCode != "GBP" {
		t.Fatalf("manual market/listing fields did not win: %#v", ticker)
	}
	if cat.Listings[0].ExchangeName != "Manual Exchange" || cat.Listings[0].CurrencyCode != "GBP" {
		t.Fatalf("listing = %#v", cat.Listings[0])
	}
	if cat.Companies[0].MarketCap != 222 {
		t.Fatalf("company = %#v", cat.Companies[0])
	}
}

func TestBuildClassificationOverridePrecedence(t *testing.T) {
	manual := emptyManual()
	manual.TickerOverrides = map[string]taxonomy.TickerOverride{
		"ABC_US_EQ": {
			Ticker:    "ABC_US_EQ",
			CompanyID: "abc",
			Sector:    "Old Manual Sector",
			Industry:  "Old Manual Industry",
			Country:   "Old Manual Country",
		},
	}
	manual.CompanyOverrides = map[string]taxonomy.CompanyOverride{
		"abc": {CompanyID: "abc", Sector: "Company Sector", Industry: "Company Industry", Country: "Company Country"},
	}
	manual.ClassificationOverrides = []taxonomy.ClassificationOverride{
		{
			TargetType:   "ticker",
			Ticker:       "ABC_US_EQ",
			Sector:       "Classification Sector",
			Industry:     "Classification Industry",
			Country:      "Classification Country",
			SourceURL:    "https://example.com/classification",
			LastReviewed: "2026-05-09",
		},
	}
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "ABC_US_EQ", Name: "ABC Corp", ISIN: "US0000000001", Type: "STOCK", CurrencyCode: "USD"},
		},
		Profiles: map[string]enrichment.Profile{
			"ABC_US_EQ": {Sector: "Provider Sector", Industry: "Provider Industry", Country: "Provider Country"},
		},
		Manual:  manual,
		BuiltAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ticker := findTicker(t, cat, "ABC_US_EQ")
	if ticker.Sector != "Classification Sector" || ticker.Industry != "Classification Industry" || ticker.Country != "Classification Country" {
		t.Fatalf("classification override did not win: %#v", ticker)
	}
	if ticker.LastReviewed != "2026-05-09" {
		t.Fatalf("last reviewed = %q", ticker.LastReviewed)
	}
	foundSource := false
	for _, source := range ticker.Sources {
		if source.Kind == "manual_classification_override" {
			foundSource = true
		}
	}
	if !foundSource {
		t.Fatalf("classification source missing: %#v", ticker.Sources)
	}
}

func TestBuildRuleClassificationForFundLikeAndStructuredProducts(t *testing.T) {
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "VUSA_L_EQ", Name: "Vanguard S&P 500 UCITS ETF Dist", ISIN: "IE00B3XXRP09", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "AGGH_L_EQ", Name: "iShares Core Global Aggregate Bond UCITS ETF", ISIN: "IE00BDBRDM35", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "3LNVDA_L_EQ", Name: "Leverage Shares 3x Long NVIDIA ETP", ISIN: "XS2820604853", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "ABCW_US_EQ", Name: "ABC Corp Warrant", ISIN: "US0000000004", Type: "WARRANT", CurrencyCode: "USD"},
		},
		Manual:  emptyManual(),
		BuiltAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	for tickerID, want := range map[string][2]string{
		"VUSA_L_EQ":   {"Funds", "Equity ETF"},
		"AGGH_L_EQ":   {"Funds", "Bond ETF"},
		"3LNVDA_L_EQ": {"Funds", "Leveraged ETP"},
		"ABCW_US_EQ":  {"Structured Products", "Warrant"},
	} {
		ticker := findTicker(t, cat, tickerID)
		if ticker.Sector != want[0] || ticker.Industry != want[1] {
			t.Fatalf("%s classification = %q/%q, want %q/%q", tickerID, ticker.Sector, ticker.Industry, want[0], want[1])
		}
	}
}

func TestBuildProductRuleExposuresForPart4Pipelines(t *testing.T) {
	manual := productPipelineManual()
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "VUSA_L_EQ", Name: "Vanguard S&P 500 UCITS ETF Dist", ISIN: "IE00B3XXRP09", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "AGGH_L_EQ", Name: "iShares Core Global Aggregate Bond UCITS ETF", ISIN: "IE00BDBRDM35", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "3SAGGH_L_EQ", Name: "iShares Core Global Aggregate Bond UCITS ETF", ISIN: "IE00SHORTBND", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "STAP_L_EQ", Name: "iShares Consumer Staples Sector UCITS ETF", ISIN: "IE00CONFLICT", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "3SSTAP_L_EQ", Name: "iShares Consumer Staples Sector UCITS ETF", ISIN: "IE00CONFLICT", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "QUAL_L_EQ", Name: "iShares MSCI World Quality Factor UCITS ETF", ISIN: "IE00QUALITY1", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "QYLD_L_EQ", Name: "Global X Nasdaq 100 Covered Call UCITS ETF", ISIN: "IE00COVERED1", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "BLCN_L_EQ", Name: "Global X Blockchain UCITS ETF", ISIN: "IE00BLOCKCH", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "3LNVDA_L_EQ", Name: "Leverage Shares 3x Long NVIDIA ETP", ISIN: "XS2820604853", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "SQQQ_L_EQ", Name: "ProShares Short QQQ ETF", ISIN: "US000SHORT01", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "ABCW_US_EQ", Name: "ABC Corp Warrant", ISIN: "US0000000004", Type: "WARRANT", CurrencyCode: "USD"},
			{Ticker: "GOLD_L_EQ", Name: "WisdomTree Physical Gold ETC", ISIN: "JE00GOLD0001", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "BTC_L_EQ", Name: "21Shares Bitcoin ETP", ISIN: "CH000BITCO1", Type: "ETF", CurrencyCode: "GBP"},
			{Ticker: "GFI_US_EQ", Name: "Gold Fields Limited", ISIN: "US0000000009", Type: "STOCK", CurrencyCode: "USD"},
		},
		Profiles: map[string]enrichment.Profile{
			"GFI_US_EQ": {Sector: "Basic Materials", Industry: "Gold"},
		},
		Manual:  manual,
		BuiltAt: time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	assertTickerThemeLayer(t, cat, "VUSA_L_EQ", "funds_core", "equity_etfs")
	assertTickerThemeLayer(t, cat, "AGGH_L_EQ", "funds_core", "bond_etfs")
	assertTickerThemeLayer(t, cat, "3SAGGH_L_EQ", "leveraged_structured", "inverse_etps")
	assertTickerThemeLayer(t, cat, "STAP_L_EQ", "funds_core", "equity_etfs")
	assertTickerThemeLayer(t, cat, "3SSTAP_L_EQ", "leveraged_structured", "inverse_etps")
	assertTickerThemeLayer(t, cat, "QUAL_L_EQ", "funds_core", "factor_etfs")
	assertTickerThemeLayer(t, cat, "QYLD_L_EQ", "funds_core", "covered_call_etfs")
	assertTickerThemeLayer(t, cat, "BLCN_L_EQ", "funds_core", "equity_etfs")
	assertTickerThemeLayer(t, cat, "3LNVDA_L_EQ", "leveraged_structured", "leveraged_etps")
	assertTickerThemeLayer(t, cat, "SQQQ_L_EQ", "leveraged_structured", "inverse_etps")
	assertTickerThemeLayer(t, cat, "ABCW_US_EQ", "leveraged_structured", "warrants")
	assertTickerThemeLayer(t, cat, "GOLD_L_EQ", "commodity_crypto_etps", "gold_etps")
	assertTickerThemeLayer(t, cat, "BTC_L_EQ", "commodity_crypto_etps", "crypto_etps")

	operatingCompany := findTicker(t, cat, "GFI_US_EQ")
	for _, themeID := range []string{"funds_core", "leveraged_structured", "commodity_crypto_etps"} {
		if containsString(operatingCompany.ThemeIDs, themeID) {
			t.Fatalf("operating company was mapped to product theme %q: %#v", themeID, operatingCompany)
		}
	}
	blockchainETF := findTicker(t, cat, "BLCN_L_EQ")
	if containsString(blockchainETF.ThemeIDs, "commodity_crypto_etps") {
		t.Fatalf("blockchain equity ETF should not be treated as a crypto ETP: %#v", blockchainETF)
	}
	if !hasExposure(cat, "funds_core", "equity_etfs", "IE00B3XXRP09") {
		t.Fatalf("expected ISIN-targeted rule exposure for VUSA: %#v", cat.Exposures)
	}
	shortBond := findTicker(t, cat, "3SAGGH_L_EQ")
	if containsString(shortBond.ThemeIDs, "funds_core") {
		t.Fatalf("ISIN-scoped product exposure leaked through shared company identity: %#v", shortBond)
	}
	conflictingListing := findTicker(t, cat, "STAP_L_EQ")
	if containsString(conflictingListing.ThemeIDs, "leveraged_structured") ||
		containsString(conflictingListing.LayerIDs, "inverse_etps") ||
		containsString(conflictingListing.LayerIDs, "leveraged_inverse_etps") {
		t.Fatalf("conflicted same-ISIN inverse exposure leaked into normal listing: %#v", conflictingListing)
	}
}

func TestBuildStockRuleExposuresForClassifiedOperatingCompanies(t *testing.T) {
	manual := stockPipelineManual()
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "TD_US_EQ", Name: "The Toronto-Dominion Bank", ISIN: "US0000000101", Type: "STOCK", CurrencyCode: "USD"},
			{Ticker: "TMUS_US_EQ", Name: "T-Mobile US Inc", ISIN: "US0000000102", Type: "STOCK", CurrencyCode: "USD"},
			{Ticker: "AMGN_US_EQ", Name: "Amgen Inc", ISIN: "US0000000103", Type: "STOCK", CurrencyCode: "USD"},
			{Ticker: "INTU_US_EQ", Name: "Intuit Inc", ISIN: "US0000000104", Type: "STOCK", CurrencyCode: "USD"},
			{Ticker: "COIN_US_EQ", Name: "Coinbase Global Inc", ISIN: "US0000000105", Type: "STOCK", CurrencyCode: "USD"},
			{Ticker: "LMND_US_EQ", Name: "Lemonade Inc", ISIN: "US0000000106", Type: "STOCK", CurrencyCode: "USD"},
		},
		Profiles: map[string]enrichment.Profile{
			"TD_US_EQ":   {Sector: "Financial Services", Industry: "Banks - Diversified"},
			"TMUS_US_EQ": {Sector: "Communication Services", Industry: "Telecom Services"},
			"AMGN_US_EQ": {Sector: "Healthcare", Industry: "Drug Manufacturers - General"},
			"INTU_US_EQ": {Sector: "Technology", Industry: "Software - Application"},
			"COIN_US_EQ": {Sector: "Financial Services", Industry: "Capital Markets"},
			"LMND_US_EQ": {Sector: "Financial Services", Industry: "Insurance - Property & Casualty"},
		},
		Manual:  manual,
		BuiltAt: time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	assertTickerThemeLayer(t, cat, "TD_US_EQ", "financial_system", "diversified_banks")
	assertTickerThemeLayer(t, cat, "TMUS_US_EQ", "connectivity_infrastructure", "telecom_operators")
	assertTickerThemeLayer(t, cat, "AMGN_US_EQ", "healthcare", "large_pharma")
	assertTickerThemeLayer(t, cat, "INTU_US_EQ", "enterprise_software", "erp_crm_workflow")
	assertTickerThemeLayer(t, cat, "COIN_US_EQ", "fintech", "crypto_rails")
	assertTickerThemeLayer(t, cat, "LMND_US_EQ", "fintech", "insurtech")
	assertTickerThemeLayer(t, cat, "LMND_US_EQ", "financial_system", "insurance")
	if hasReviewReason(cat.ReviewQueues, ReviewQueueTaxonomy, ReasonMissingThemeExposure) {
		t.Fatalf("stock rule exposures should clear missing theme review rows: %#v", cat.ReviewQueues)
	}
	if !hasExposure(cat, "financial_system", "diversified_banks", "US0000000101") {
		t.Fatalf("expected ISIN-targeted stock rule exposure for TD: %#v", cat.Exposures)
	}
}

func TestBuildStockRuleExposuresYieldToResolvedManualExposures(t *testing.T) {
	manual := stockPipelineManual()
	manual.Exposures = []taxonomy.Exposure{{
		ThemeID:       "financial_system",
		LayerID:       "diversified_banks",
		Ticker:        "TD_US_EQ",
		ExposureScore: 5,
		Confidence:    "manual_high",
		SourceURL:     "https://example.com/td",
		LastReviewed:  "2026-05-17",
	}}
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "TD_US_EQ", Name: "The Toronto-Dominion Bank", ISIN: "US0000000101", Type: "STOCK", CurrencyCode: "USD"},
			{Ticker: "TDBd_EQ", Name: "The Toronto-Dominion Bank", ISIN: "US0000000101", Type: "STOCK", CurrencyCode: "EUR"},
		},
		Profiles: map[string]enrichment.Profile{
			"TD_US_EQ": {Sector: "Financial Services", Industry: "Banks - Diversified"},
			"TDBd_EQ":  {Sector: "Financial Services", Industry: "Banks - Diversified"},
		},
		Manual:  manual,
		BuiltAt: time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	assertTickerThemeLayer(t, cat, "TD_US_EQ", "financial_system", "diversified_banks")
	assertTickerThemeLayer(t, cat, "TDBd_EQ", "financial_system", "diversified_banks")
	for _, exposure := range cat.Exposures {
		if exposure.ThemeID == "financial_system" &&
			exposure.LayerID == "diversified_banks" &&
			exposure.ISIN == "US0000000101" &&
			exposure.Confidence == confidenceStockRule {
			t.Fatalf("stock rule emitted ISIN exposure that would duplicate the reviewed ticker exposure: %#v", exposure)
		}
	}
	if !hasTickerExposure(cat, "financial_system", "diversified_banks", "TDBd_EQ", confidenceStockRule) {
		t.Fatalf("expected stock rule to keep non-shadowed same-ISIN listing covered: %#v", cat.Exposures)
	}
}

func TestBuildCapsGeneratedRelatedTickersForBroadIndustries(t *testing.T) {
	instruments := make([]trading212.Instrument, 0, maxRelatedTickersPerIndustry+5)
	for i := 0; i < maxRelatedTickersPerIndustry+5; i++ {
		instruments = append(instruments, trading212.Instrument{
			Ticker:       fmt.Sprintf("ETF%02d_L_EQ", i),
			Name:         fmt.Sprintf("Example Equity ETF %02d", i),
			ISIN:         fmt.Sprintf("IE000000%04d", i),
			Type:         "ETF",
			CurrencyCode: "GBP",
		})
	}
	cat, err := Build(BuildInput{
		Instruments: instruments,
		Manual:      emptyManual(),
		BuiltAt:     time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, ticker := range cat.Tickers {
		if len(ticker.RelatedTickers) > maxRelatedTickersPerIndustry {
			t.Fatalf("%s has %d related tickers, want at most %d", ticker.Ticker, len(ticker.RelatedTickers), maxRelatedTickersPerIndustry)
		}
	}
}

func TestBuildManifestEnrichmentDiagnostics(t *testing.T) {
	diagnostics := EnrichmentDiagnostics{
		CacheSchemaVersion: 1,
		Provider:           "cache",
		CacheHitCount:      2,
		CacheMissCount:     1,
		CacheStaleCount:    1,
		AmbiguousCount:     1,
		FailureCount:       1,
		FailureCSV:         "site/data/enrichment_failures.csv",
		OldestRetrievedAt:  "2026-05-01T12:00:00Z",
		NewestRetrievedAt:  "2026-05-09T12:00:00Z",
	}
	failures := []EnrichmentFailure{{
		Ticker:           "ABC_US_EQ",
		ISIN:             "US0000000001",
		Name:             "ABC Corp",
		Provider:         "cache",
		AttemptedSymbols: "ABC;ABC_US_EQ",
		Status:           "cache_miss",
		Error:            "enrichment cache miss",
		NextAction:       "populate cache",
	}}
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "ABC_US_EQ", Name: "ABC Corp", ISIN: "US0000000001", Type: "STOCK", CurrencyCode: "USD"},
		},
		Manual:                emptyManual(),
		BuiltAt:               time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
		EnrichmentAttempted:   1,
		EnrichmentFailed:      1,
		EnrichmentDiagnostics: diagnostics,
		EnrichmentFailures:    failures,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := cat.Manifest
	if manifest.EnrichmentCacheSchemaVersion != 1 ||
		manifest.EnrichmentProvider != "cache" ||
		manifest.EnrichmentCacheHitCount != 2 ||
		manifest.EnrichmentCacheMissCount != 1 ||
		manifest.EnrichmentCacheStaleCount != 1 ||
		manifest.EnrichmentAmbiguousCount != 1 ||
		manifest.EnrichmentFailureCount != 1 ||
		manifest.EnrichmentFailureCSV != "site/data/enrichment_failures.csv" ||
		manifest.EnrichmentOldestRetrievedAt != "2026-05-01T12:00:00Z" ||
		manifest.EnrichmentNewestRetrievedAt != "2026-05-09T12:00:00Z" {
		t.Fatalf("manifest enrichment diagnostics = %#v", manifest)
	}
	if len(cat.EnrichmentFailures) != 1 || cat.EnrichmentFailures[0].Ticker != "ABC_US_EQ" {
		t.Fatalf("enrichment failures = %#v", cat.EnrichmentFailures)
	}
}

func TestBuildExportsManualRelationshipsInContract(t *testing.T) {
	manual := emptyManual()
	manual.Themes = []taxonomy.Theme{{ID: "ai", Name: "AI"}}
	manual.SupplyChains = []taxonomy.SupplyChain{{
		ThemeID: "ai",
		Name:    "AI chain",
		Layers:  []taxonomy.SupplyChainLayer{{ID: "chips", Name: "Chips", Order: 10}},
	}}
	manual.Relationships = []taxonomy.Relationship{
		{
			RelationshipType: "substitute",
			SourceTicker:     "ABC_US_EQ",
			TargetTicker:     "DEF_US_EQ",
			ThemeID:          "ai",
			LayerID:          "chips",
			Confidence:       "manual_low",
			SourceURL:        "https://example.com/substitute",
			LastReviewed:     "2026-05-09",
		},
		{
			RelationshipType: "peer",
			SourceTicker:     "ABC_US_EQ",
			TargetTicker:     "XYZ_US_EQ",
			ThemeID:          "ai",
			LayerID:          "chips",
			Confidence:       "manual_medium",
			SourceURL:        "https://example.com/peer",
			LastReviewed:     "2026-05-09",
		},
	}
	cat, err := Build(BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "ABC_US_EQ", Name: "ABC Corp", ISIN: "US0000000001", Type: "STOCK", CurrencyCode: "USD"},
		},
		Manual:  manual,
		BuiltAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if cat.DataContractVersion != DataContractVersion || cat.SchemaVersion != DataContractSchemaVersion {
		t.Fatalf("catalogue contract versions = %d/%d", cat.DataContractVersion, cat.SchemaVersion)
	}
	if cat.Manifest.DataContractVersion != DataContractVersion || cat.Manifest.SchemaVersion != DataContractSchemaVersion {
		t.Fatalf("manifest contract versions = %d/%d", cat.Manifest.DataContractVersion, cat.Manifest.SchemaVersion)
	}
	if cat.Manifest.RelationshipCount != 2 || len(cat.Relationships) != 2 {
		t.Fatalf("relationship contract fields = count %d rows %#v", cat.Manifest.RelationshipCount, cat.Relationships)
	}
	if cat.Relationships[0].RelationshipType != "peer" || cat.Relationships[1].RelationshipType != "substitute" {
		t.Fatalf("relationships not sorted deterministically: %#v", cat.Relationships)
	}
}

func emptyManual() taxonomy.ManualData {
	return taxonomy.ManualData{
		CompanyOverrides: map[string]taxonomy.CompanyOverride{},
		TickerOverrides:  map[string]taxonomy.TickerOverride{},
	}
}

func productPipelineManual() taxonomy.ManualData {
	manual := emptyManual()
	manual.Themes = []taxonomy.Theme{
		{ID: "funds_core", Name: "Core funds"},
		{ID: "leveraged_structured", Name: "Leveraged and structured products"},
		{ID: "commodity_crypto_etps", Name: "Commodity and crypto ETPs"},
	}
	manual.SupplyChains = []taxonomy.SupplyChain{
		{
			ThemeID: "funds_core",
			Name:    "Core funds product map",
			Layers: []taxonomy.SupplyChainLayer{
				{ID: "equity_etfs", Name: "Equity ETFs", Order: 10},
				{ID: "bond_etfs", Name: "Bond ETFs", Order: 20},
				{ID: "factor_etfs", Name: "Factor ETFs", Order: 30},
				{ID: "covered_call_etfs", Name: "Covered-call ETFs", Order: 40},
				{ID: "money_market_funds", Name: "Money-market funds", Order: 50},
				{ID: "multi_asset_funds", Name: "Multi-asset funds", Order: 60},
				{ID: "investment_trusts", Name: "Investment trusts", Order: 70},
			},
		},
		{
			ThemeID: "leveraged_structured",
			Name:    "Leveraged and structured product map",
			Layers: []taxonomy.SupplyChainLayer{
				{ID: "leveraged_etps", Name: "Leveraged ETPs", Order: 10},
				{ID: "inverse_etps", Name: "Inverse ETPs", Order: 20},
				{ID: "leveraged_inverse_etps", Name: "Leveraged inverse ETPs", Order: 30},
				{ID: "warrants", Name: "Warrants", Order: 40},
				{ID: "structured_products", Name: "Structured products", Order: 50},
				{ID: "complex_payoff_products", Name: "Complex payoff products", Order: 60},
			},
		},
		{
			ThemeID: "commodity_crypto_etps",
			Name:    "Commodity and crypto ETP map",
			Layers: []taxonomy.SupplyChainLayer{
				{ID: "broad_commodity_etps", Name: "Broad commodity ETPs", Order: 10},
				{ID: "precious_metals_etps", Name: "Precious-metals ETPs", Order: 20},
				{ID: "gold_etps", Name: "Gold ETPs", Order: 30},
				{ID: "silver_etps", Name: "Silver ETPs", Order: 40},
				{ID: "energy_commodity_etps", Name: "Energy commodity ETPs", Order: 50},
				{ID: "agriculture_commodity_etps", Name: "Agriculture commodity ETPs", Order: 60},
				{ID: "industrial_metals_etps", Name: "Industrial-metals ETPs", Order: 70},
				{ID: "crypto_etps", Name: "Crypto ETPs", Order: 80},
			},
		},
	}
	return manual
}

func stockPipelineManual() taxonomy.ManualData {
	manual := emptyManual()
	manual.Themes = []taxonomy.Theme{
		{ID: "financial_system", Name: "Financial system"},
		{ID: "connectivity_infrastructure", Name: "Connectivity infrastructure"},
		{ID: "healthcare", Name: "Healthcare"},
		{ID: "enterprise_software", Name: "Enterprise software"},
		{ID: "fintech", Name: "Fintech"},
	}
	manual.SupplyChains = []taxonomy.SupplyChain{
		{
			ThemeID: "financial_system",
			Name:    "Financial system supply chain",
			Layers: []taxonomy.SupplyChainLayer{
				{ID: "diversified_banks", Name: "Diversified banks", Order: 10},
				{ID: "insurance", Name: "Insurance", Order: 20},
			},
		},
		{
			ThemeID: "connectivity_infrastructure",
			Name:    "Connectivity infrastructure supply chain",
			Layers:  []taxonomy.SupplyChainLayer{{ID: "telecom_operators", Name: "Telecom operators", Order: 10}},
		},
		{
			ThemeID: "healthcare",
			Name:    "Healthcare supply chain",
			Layers:  []taxonomy.SupplyChainLayer{{ID: "large_pharma", Name: "Large pharma", Order: 10}},
		},
		{
			ThemeID: "enterprise_software",
			Name:    "Enterprise software supply chain",
			Layers:  []taxonomy.SupplyChainLayer{{ID: "erp_crm_workflow", Name: "ERP and workflow", Order: 10}},
		},
		{
			ThemeID: "fintech",
			Name:    "Fintech supply chain",
			Layers: []taxonomy.SupplyChainLayer{
				{ID: "crypto_rails", Name: "Crypto rails", Order: 10},
				{ID: "insurtech", Name: "Insurtech", Order: 20},
			},
		},
	}
	return manual
}

func findTicker(t *testing.T, cat *Catalogue, ticker string) Ticker {
	t.Helper()
	for _, row := range cat.Tickers {
		if row.Ticker == ticker {
			return row
		}
	}
	t.Fatalf("missing ticker %q in %#v", ticker, cat.Tickers)
	return Ticker{}
}

func assertTickerThemeLayer(t *testing.T, cat *Catalogue, tickerID string, themeID string, layerID string) {
	t.Helper()
	ticker := findTicker(t, cat, tickerID)
	if !containsString(ticker.ThemeIDs, themeID) || !containsString(ticker.LayerIDs, layerID) {
		t.Fatalf("%s theme/layers = %#v/%#v, want %s/%s", tickerID, ticker.ThemeIDs, ticker.LayerIDs, themeID, layerID)
	}
}

func hasExposure(cat *Catalogue, themeID string, layerID string, isin string) bool {
	for _, exposure := range cat.Exposures {
		if exposure.ThemeID == themeID && exposure.LayerID == layerID && exposure.ISIN == isin && exposure.Confidence == "rule_low" {
			return true
		}
	}
	return false
}

func hasTickerExposure(cat *Catalogue, themeID string, layerID string, tickerID string, confidence string) bool {
	for _, exposure := range cat.Exposures {
		if exposure.ThemeID == themeID && exposure.LayerID == layerID && exposure.Ticker == tickerID && exposure.Confidence == confidence {
			return true
		}
	}
	return false
}

func hasIdentityIssue(cat *Catalogue, code string) bool {
	for _, issue := range cat.IdentityIssues {
		if issue.IssueCode == code {
			return true
		}
	}
	return false
}
