package catalogue

import (
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

func emptyManual() taxonomy.ManualData {
	return taxonomy.ManualData{
		CompanyOverrides: map[string]taxonomy.CompanyOverride{},
		TickerOverrides:  map[string]taxonomy.TickerOverride{},
	}
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

func hasIdentityIssue(cat *Catalogue, code string) bool {
	for _, issue := range cat.IdentityIssues {
		if issue.IssueCode == code {
			return true
		}
	}
	return false
}
