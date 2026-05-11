package export

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"statos/internal/catalogue"
	"statos/internal/taxonomy"
)

var updateGolden = flag.Bool("update", false, "update export golden files")

func TestWriteSiteDataWritesExpectedFiles(t *testing.T) {
	dir := t.TempDir()
	cat := testCatalogue()

	if err := WriteSiteData(dir, cat); err != nil {
		t.Fatal(err)
	}
	for _, name := range GeneratedSiteDataFiles {
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

	file, err = os.Open(filepath.Join(dir, "suggested_classification_overrides.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	records, err = csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || !reflect.DeepEqual(records[0], taxonomy.ClassificationOverridesCSVHeader) || records[1][0] != "ticker" || records[1][1] != "ABC_US_EQ" {
		t.Fatalf("suggested_classification_overrides.csv records = %#v", records)
	}
}

func TestStandaloneJSONExportsMatchCatalogue(t *testing.T) {
	dir := t.TempDir()
	cat := testCatalogue()
	if err := WriteSiteData(dir, cat); err != nil {
		t.Fatal(err)
	}

	var bootstrap AppBootstrap
	readJSON(t, filepath.Join(dir, "app_bootstrap.json"), &bootstrap)
	if bootstrap.Manifest.GeneratedFiles != nil {
		t.Fatalf("app_bootstrap embedded manifest should not carry generatedFiles: %#v", bootstrap.Manifest.GeneratedFiles)
	}
	if len(bootstrap.GeneratedFiles) != len(GeneratedSiteDataFiles) {
		t.Fatalf("app_bootstrap generatedFiles = %d, want %d", len(bootstrap.GeneratedFiles), len(GeneratedSiteDataFiles))
	}
	if bootstrap.Counts.TickerCount != len(cat.Tickers) || bootstrap.Counts.CompanyCount != len(cat.Companies) {
		t.Fatalf("app_bootstrap counts = %#v", bootstrap.Counts)
	}

	var tickerIndex TickerIndex
	readJSON(t, filepath.Join(dir, "tickers_index.json"), &tickerIndex)
	if len(tickerIndex.Tickers) != len(cat.Tickers) || tickerIndex.Tickers[0].Ticker != cat.Tickers[0].Ticker {
		t.Fatalf("tickers_index.json = %#v", tickerIndex.Tickers)
	}
	if tickerIndex.Tickers[0].CompanyID != cat.Tickers[0].CompanyID || tickerIndex.Tickers[0].SecurityID != cat.Tickers[0].SecurityID {
		t.Fatalf("tickers_index identity fields = %#v", tickerIndex.Tickers[0])
	}

	var securities []catalogue.Security
	readJSON(t, filepath.Join(dir, "securities.json"), &securities)
	if !reflect.DeepEqual(securities, cat.Securities) {
		t.Fatalf("securities.json = %#v, want %#v", securities, cat.Securities)
	}

	var listings []catalogue.Listing
	readJSON(t, filepath.Join(dir, "listings.json"), &listings)
	if !reflect.DeepEqual(listings, cat.Listings) {
		t.Fatalf("listings.json = %#v, want %#v", listings, cat.Listings)
	}

	var relationships []taxonomy.Relationship
	readJSON(t, filepath.Join(dir, "relationships.json"), &relationships)
	wantRelationships := sortedRelationships(cat.Relationships)
	if !reflect.DeepEqual(relationships, wantRelationships) {
		t.Fatalf("relationships.json = %#v, want %#v", relationships, wantRelationships)
	}
	if relationships[0].RelationshipType != "peer" || relationships[1].RelationshipType != "substitute" {
		t.Fatalf("relationships were not sorted deterministically: %#v", relationships)
	}

	var unclassified []catalogue.UnclassifiedRow
	readJSON(t, filepath.Join(dir, "unclassified.json"), &unclassified)
	if !reflect.DeepEqual(unclassified, cat.Unclassified) {
		t.Fatalf("unclassified.json = %#v, want %#v", unclassified, cat.Unclassified)
	}

	var reviewQueues []catalogue.ReviewQueueRow
	readJSON(t, filepath.Join(dir, "review_queues.json"), &reviewQueues)
	if !reflect.DeepEqual(reviewQueues, cat.ReviewQueues) {
		t.Fatalf("review_queues.json = %#v, want %#v", reviewQueues, cat.ReviewQueues)
	}
	var reviewSummary catalogue.ReviewSummary
	readJSON(t, filepath.Join(dir, "review_summary.json"), &reviewSummary)
	if reviewSummary.TotalCount != len(cat.ReviewQueues) || reviewSummary.ByReasonCode[catalogue.ReasonMissingSector] != 1 {
		t.Fatalf("review_summary.json = %#v", reviewSummary)
	}
}

func TestManifestContractMetadataAndChecksumCoverage(t *testing.T) {
	dir := t.TempDir()
	cat := testCatalogue()
	if err := WriteSiteData(dir, cat); err != nil {
		t.Fatal(err)
	}

	var manifest catalogue.BuildManifest
	readJSON(t, filepath.Join(dir, "build_manifest.json"), &manifest)
	if manifest.DataContractVersion != catalogue.DataContractVersion || manifest.SchemaVersion != catalogue.DataContractSchemaVersion {
		t.Fatalf("manifest contract versions = %d/%d", manifest.DataContractVersion, manifest.SchemaVersion)
	}
	if manifest.RelationshipCount != len(cat.Relationships) {
		t.Fatalf("RelationshipCount = %d, want %d", manifest.RelationshipCount, len(cat.Relationships))
	}

	gotPaths := make([]string, 0, len(manifest.GeneratedFiles))
	for _, file := range manifest.GeneratedFiles {
		gotPaths = append(gotPaths, strings.TrimPrefix(file.Path, "site/data/"))
		if file.SchemaVersion != catalogue.DataContractSchemaVersion {
			t.Fatalf("%s schemaVersion = %d", file.Path, file.SchemaVersion)
		}
		if file.Format != "json" && file.Format != "csv" {
			t.Fatalf("%s format = %q", file.Path, file.Format)
		}
	}
	if !reflect.DeepEqual(gotPaths, GeneratedSiteDataFiles) {
		t.Fatalf("generatedFiles paths = %#v, want %#v", gotPaths, GeneratedSiteDataFiles)
	}

	for _, file := range manifest.GeneratedFiles {
		name := strings.TrimPrefix(file.Path, "site/data/")
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if file.Bytes != int64(len(b)) {
			t.Fatalf("%s bytes = %d, want %d", file.Path, file.Bytes, len(b))
		}
		if name == "app_bootstrap.json" {
			if file.ChecksumMode != appBootstrapChecksumMode {
				t.Fatalf("app_bootstrap checksumMode = %q", file.ChecksumMode)
			}
			var bootstrap AppBootstrap
			if err := json.Unmarshal(b, &bootstrap); err != nil {
				t.Fatal(err)
			}
			projection := bootstrap
			projection.GeneratedFiles = nil
			projectionBytes, err := marshalCompactJSON(projection)
			if err != nil {
				t.Fatal(err)
			}
			if file.SHA256 != shaHex(projectionBytes) {
				t.Fatalf("app_bootstrap projection sha256 = %s, want %s", file.SHA256, shaHex(projectionBytes))
			}
			continue
		}
		if name == "build_manifest.json" {
			if file.ChecksumMode != buildManifestChecksumMode {
				t.Fatalf("build_manifest checksumMode = %q", file.ChecksumMode)
			}
			projection := manifest
			projection.GeneratedFiles = nil
			projectionBytes, err := marshalJSON(projection)
			if err != nil {
				t.Fatal(err)
			}
			if file.SHA256 != shaHex(projectionBytes) {
				t.Fatalf("build_manifest projection sha256 = %s, want %s", file.SHA256, shaHex(projectionBytes))
			}
			continue
		}
		if file.ChecksumMode != "" {
			t.Fatalf("%s unexpected checksumMode %q", file.Path, file.ChecksumMode)
		}
		if file.SHA256 != shaHex(b) {
			t.Fatalf("%s sha256 = %s, want %s", file.Path, file.SHA256, shaHex(b))
		}
	}
}

func TestCSVHeaderStability(t *testing.T) {
	want := map[string][]string{
		"tickers.csv":             {"ticker", "name", "isin", "company_id", "security_id", "type", "instrument_category", "structure_flags", "currency", "exchange", "yahoo_symbol", "sector", "industry", "country", "market_cap", "directionality", "identity_confidence", "identity_reasons", "themes", "layers", "unclassified"},
		"securities.csv":          {"security_id", "isin", "name", "type", "instrument_category", "structure_flags", "company_id", "listing_ids", "ticker_ids", "currency_set", "identity_confidence", "identity_reasons"},
		"listings.csv":            {"listing_id", "ticker", "security_id", "company_id", "exchange_code", "exchange_name", "currency_code"},
		"unclassified.csv":        {"ticker", "company_id", "name", "isin", "reason", "reason_codes"},
		"taxonomy_issues.csv":     {"queue", "reason_code", "severity", "ticker", "isin", "company_id", "security_id", "name", "sector", "industry", "theme_ids", "layer_ids", "source_file", "source_row", "suggested_action", "suggested_manual_file", "suggested_csv_row", "last_reviewed", "last_refreshed"},
		"enrichment_issues.csv":   {"queue", "reason_code", "severity", "ticker", "isin", "company_id", "security_id", "name", "sector", "industry", "theme_ids", "layer_ids", "source_file", "source_row", "suggested_action", "suggested_manual_file", "suggested_csv_row", "last_reviewed", "last_refreshed"},
		"identity_issues.csv":     {"issue_code", "ticker", "isin", "security_id", "company_id", "name", "reason", "suggested_action"},
		"enrichment_failures.csv": {"ticker", "isin", "name", "provider", "attempted_symbols", "status", "error", "next_action"},
		"stale_reviews.csv":       {"queue", "reason_code", "severity", "ticker", "isin", "company_id", "security_id", "name", "sector", "industry", "theme_ids", "layer_ids", "source_file", "source_row", "suggested_action", "suggested_manual_file", "suggested_csv_row", "last_reviewed", "last_refreshed"},
	}
	want["suggested_classification_overrides.csv"] = taxonomy.ClassificationOverridesCSVHeader
	want["suggested_exposures.csv"] = taxonomy.ExposureCSVHeader
	want["suggested_ticker_overrides.csv"] = taxonomy.TickerOverridesCSVHeader
	want["suggested_identity_overrides.csv"] = taxonomy.IdentityOverridesCSVHeader
	if !reflect.DeepEqual(CSVHeaders(), want) {
		t.Fatalf("CSVHeaders() = %#v, want %#v", CSVHeaders(), want)
	}

	dir := t.TempDir()
	if err := WriteSiteData(dir, testCatalogue()); err != nil {
		t.Fatal(err)
	}
	for name, header := range want {
		got := readCSVHeader(t, filepath.Join(dir, name))
		if !reflect.DeepEqual(got, header) {
			t.Fatalf("%s header = %#v, want %#v", name, got, header)
		}
	}
}

func TestDataContractDocsListCSVHeaders(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "data-contract.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	for name, header := range CSVHeaders() {
		want := "`" + strings.Join(header, ",") + "`"
		if !strings.Contains(doc, "### "+name) || !strings.Contains(doc, want) {
			t.Fatalf("docs/data-contract.md does not document %s header %s", name, want)
		}
	}
}

func TestCatalogueJSONShapeRemainsFrontendCompatible(t *testing.T) {
	dir := t.TempDir()
	if err := WriteSiteData(dir, testCatalogue()); err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	readJSON(t, filepath.Join(dir, "catalogue.json"), &raw)
	for _, key := range []string{"dataContractVersion", "schemaVersion", "manifest", "tickers", "securities", "listings", "relationships"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("catalogue.json missing key %q", key)
		}
	}
	var tickers []catalogue.Ticker
	if err := json.Unmarshal(raw["tickers"], &tickers); err != nil {
		t.Fatalf("tickers is no longer a frontend-compatible array: %v", err)
	}
	var manifest catalogue.BuildManifest
	if err := json.Unmarshal(raw["manifest"], &manifest); err != nil {
		t.Fatalf("manifest is no longer an object: %v", err)
	}
	if len(manifest.GeneratedFiles) != 0 {
		t.Fatalf("catalogue embedded manifest should not carry generatedFiles checksum projection: %#v", manifest.GeneratedFiles)
	}
}

func TestWriteSiteDataGoldenSnapshot(t *testing.T) {
	dir := t.TempDir()
	if err := WriteSiteData(dir, testCatalogue()); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{
		"app_bootstrap.json",
		"tickers_index.json",
		"build_manifest.json",
		"catalogue.json",
		"unclassified.json",
		"review_queues.json",
		"review_summary.json",
		"tickers.csv",
		"securities.csv",
		"listings.csv",
		"securities.json",
		"listings.json",
		"relationships.json",
		"taxonomy_issues.csv",
		"enrichment_issues.csv",
		"stale_reviews.csv",
		"suggested_classification_overrides.csv",
	} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		assertGolden(t, name, got)
	}
}

func testCatalogue() *catalogue.Catalogue {
	builtAt := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	cat := &catalogue.Catalogue{
		DataContractVersion: catalogue.DataContractVersion,
		SchemaVersion:       catalogue.DataContractSchemaVersion,
		GeneratedAt:         builtAt,
		Manifest: catalogue.BuildManifest{
			DataContractVersion:    catalogue.DataContractVersion,
			SchemaVersion:          catalogue.DataContractSchemaVersion,
			BuiltAt:                builtAt,
			SourceMode:             "sample",
			Trading212Environment:  "sample",
			InstrumentCount:        1,
			ExchangeCount:          1,
			SecurityCount:          1,
			CompanyCount:           1,
			ListingCount:           1,
			ThemeCount:             1,
			ExposureCount:          1,
			RelationshipCount:      2,
			EnrichmentAttempted:    1,
			EnrichmentSucceeded:    0,
			EnrichmentFailed:       1,
			UnclassifiedCount:      0,
			IdentityIssueCount:     1,
			EnrichmentFailureCount: 1,
			EnrichmentFailureCSV:   "site/data/enrichment_failures.csv",
			DataFreshness:          "0h",
		},
		Tickers: []catalogue.Ticker{{
			Ticker:             "ABC_US_EQ",
			Name:               "ABC Corp",
			Type:               "STOCK",
			InstrumentCategory: catalogue.CategoryStock,
			CurrencyCode:       "USD",
			ISIN:               "US0000000001",
			ExchangeCode:       "US",
			ExchangeName:       "NASDAQ",
			SecurityID:         "isin:US0000000001",
			CompanyID:          "abc",
			ListingID:          "ABC_US_EQ",
			Directionality:     "long_or_unlevered",
			IdentityConfidence: "rule_high",
			ThemeIDs:           []string{"ai"},
			LayerIDs:           []string{"chips"},
			LastRefreshed:      builtAt,
		}},
		Securities: []catalogue.Security{{
			ID:                 "isin:US0000000001",
			ISIN:               "US0000000001",
			Name:               "ABC Corp",
			Type:               "STOCK",
			InstrumentCategory: catalogue.CategoryStock,
			CompanyID:          "abc",
			ListingIDs:         []string{"ABC_US_EQ"},
			TickerIDs:          []string{"ABC_US_EQ"},
			CurrencySet:        []string{"USD"},
			IdentityConfidence: "rule_high",
		}},
		Listings: []catalogue.Listing{{
			ID:           "ABC_US_EQ",
			Ticker:       "ABC_US_EQ",
			CompanyID:    "abc",
			SecurityID:   "isin:US0000000001",
			ExchangeCode: "US",
			ExchangeName: "NASDAQ",
			CurrencyCode: "USD",
		}},
		Companies: []catalogue.Company{{
			ID:                 "abc",
			Name:               "ABC Corp",
			PrimaryTicker:      "ABC_US_EQ",
			Sector:             "Technology",
			Industry:           "Semiconductors",
			Country:            "US",
			SecurityIDs:        []string{"isin:US0000000001"},
			ListingIDs:         []string{"ABC_US_EQ"},
			TickerIDs:          []string{"ABC_US_EQ"},
			ThemeIDs:           []string{"ai"},
			LayerIDs:           []string{"chips"},
			IdentityConfidence: "rule_high",
			LastRefreshed:      builtAt,
		}},
		Sectors:    []catalogue.GroupCount{{ID: "technology", Name: "Technology", Count: 1, Tickers: []string{"ABC_US_EQ"}}},
		Industries: []catalogue.GroupCount{{ID: "semiconductors", Name: "Semiconductors", Count: 1, Tickers: []string{"ABC_US_EQ"}}},
		Themes:     []taxonomy.Theme{{ID: "ai", Name: "AI", Description: "Artificial intelligence"}},
		SupplyChains: []taxonomy.SupplyChain{{
			ThemeID: "ai",
			Name:    "AI chain",
			Layers:  []taxonomy.SupplyChainLayer{{ID: "chips", Name: "Chips", Order: 10}},
		}},
		Exposures: []taxonomy.Exposure{{
			ThemeID:       "ai",
			LayerID:       "chips",
			Ticker:        "ABC_US_EQ",
			CompanyID:     "abc",
			ExposureScore: 4,
			Confidence:    "manual_high",
			SourceURL:     "https://example.com/exposure",
			Rationale:     "Sample exposure",
			LastReviewed:  "2026-05-09",
		}},
		Relationships: []taxonomy.Relationship{
			{
				RelationshipType: "substitute",
				SourceTicker:     "ABC_US_EQ",
				TargetTicker:     "DEF_US_EQ",
				ThemeID:          "ai",
				LayerID:          "chips",
				Confidence:       "manual_low",
				SourceURL:        "https://example.com/substitute",
				Rationale:        "Alternative supplier",
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
				Rationale:        "Comparable company",
				LastReviewed:     "2026-05-09",
			},
		},
		Notes:          []taxonomy.Note{{TargetType: "ticker", TargetID: "ABC_US_EQ", Title: "ABC note", Path: "data/manual/notes/abc.md", Text: "Reviewed manually."}},
		IdentityIssues: []catalogue.IdentityIssue{{IssueCode: "missing_isin", Ticker: "ABC_US_EQ", Reason: "test", SuggestedAction: "review"}},
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
	}
	cat.ReviewQueues = []catalogue.ReviewQueueRow{{
		Queue:               catalogue.ReviewQueueTaxonomy,
		ReasonCode:          catalogue.ReasonMissingSector,
		Severity:            catalogue.ReviewSeverityMedium,
		Ticker:              "ABC_US_EQ",
		ISIN:                "US0000000001",
		CompanyID:           "abc",
		SecurityID:          "isin:US0000000001",
		Name:                "ABC Corp",
		Sector:              "Technology",
		Industry:            "Semiconductors",
		ThemeIDs:            []string{"ai"},
		LayerIDs:            []string{"chips"},
		SourceFile:          "site/data/unclassified.csv",
		SourceRow:           2,
		SuggestedAction:     "add reviewed sector and industry fields in classification_overrides.csv",
		SuggestedManualFile: "data/manual/classification_overrides.csv",
		SuggestedCSVRow:     "ticker,ABC_US_EQ,,,,,,,",
		LastRefreshed:       builtAt,
	}}
	cat.ReviewSummary = catalogue.BuildReviewSummary(cat.ReviewQueues, time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC))
	cat.Manifest.ReviewQueueCounts = cat.ReviewSummary.ByQueue
	cat.Manifest.ReviewReasonCounts = cat.ReviewSummary.ByReasonCode
	return cat
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func readCSVHeader(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 {
		t.Fatalf("%s is empty", path)
	}
	return records[0]
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s changed; run `GOCACHE=\"$PWD/.gocache\" go test ./internal/export -run TestWriteSiteDataGoldenSnapshot -update` after an intentional contract update", name)
	}
}

func shaHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
