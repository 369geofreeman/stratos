package catalogue

import (
	"encoding/csv"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"statos/internal/enrichment"
	"statos/internal/taxonomy"
	"statos/internal/trading212"
)

func TestReasonCodesForUnclassifiedReason(t *testing.T) {
	got := ReasonCodesForUnclassifiedReason("missing sector; missing industry; missing theme exposure")
	want := []string{ReasonMissingSector, ReasonMissingIndustry, ReasonMissingThemeExposure}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReasonCodesForUnclassifiedReason = %#v, want %#v", got, want)
	}
}

func TestBuildReviewQueuesSummarySuggestionsStaleAndDeltas(t *testing.T) {
	input := reviewFixtureInput()
	cat, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}

	for _, code := range []string{ReasonMissingSector, ReasonMissingIndustry, ReasonMissingThemeExposure} {
		row := findReviewRow(t, cat.ReviewQueues, ReviewQueueTaxonomy, code, "ABC_US_EQ")
		if row.SuggestedCSVRow == "" || row.SuggestedManualFile == "" {
			t.Fatalf("taxonomy row %s missing suggestion: %#v", code, row)
		}
	}
	if got := findUnclassified(t, cat, "ABC_US_EQ").ReasonCodes; !containsString(got, ReasonMissingSector) || !containsString(got, ReasonMissingIndustry) || !containsString(got, ReasonMissingThemeExposure) {
		t.Fatalf("unclassified reason codes = %#v", got)
	}

	cacheMiss := findReviewRow(t, cat.ReviewQueues, ReviewQueueEnrichment, ReasonEnrichmentCacheMiss, "ABC_US_EQ")
	if cacheMiss.IssueType != enrichment.StatusCacheMiss || cacheMiss.SuggestedManualFile != "data/manual/ticker_overrides.csv" {
		t.Fatalf("cache miss review row = %#v", cacheMiss)
	}
	ambiguous := findReviewRow(t, cat.ReviewQueues, ReviewQueueEnrichment, ReasonEnrichmentAmbiguous, "NOISIN_US_EQ")
	if ambiguous.Severity != ReviewSeverityHigh {
		t.Fatalf("ambiguous severity = %#v", ambiguous)
	}
	raw, err := json.Marshal(cacheMiss)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "attemptedSymbols") || strings.Contains(string(raw), "provider payload") {
		t.Fatalf("enrichment review row leaked raw/provider payload fields: %s", raw)
	}

	for _, code := range []string{ReasonMissingISIN, ReasonIdentityLowConfidence, ReasonIdentityDuplicate, ReasonIdentityOverrideMiss} {
		if !hasReviewReason(cat.ReviewQueues, ReviewQueueIdentity, code) {
			t.Fatalf("missing identity review reason %s in %#v", code, cat.ReviewQueues)
		}
	}

	staleMissing := findReviewRow(t, cat.ReviewQueues, ReviewQueueStale, ReasonManualReviewStale, "STALE_US_EQ")
	if staleMissing.StaleBucket != "missing_last_reviewed" {
		t.Fatalf("missing last_reviewed stale bucket = %#v", staleMissing)
	}
	if !hasStaleBucket(cat.ReviewQueues, "manual_high_over_180d") || !hasStaleBucket(cat.ReviewQueues, "manual_medium_over_120d") || !hasStaleBucket(cat.ReviewQueues, "rule_low_over_60d") {
		t.Fatalf("stale buckets missing from %#v", cat.ReviewQueues)
	}

	assertSuggestedRecordShape(t, findReviewRow(t, cat.ReviewQueues, ReviewQueueTaxonomy, ReasonMissingSector, "ABC_US_EQ"), taxonomy.ClassificationOverridesCSVHeader, []string{"ticker", "ABC_US_EQ"})
	assertSuggestedRecordShape(t, findReviewRow(t, cat.ReviewQueues, ReviewQueueTaxonomy, ReasonMissingThemeExposure, "ABC_US_EQ"), taxonomy.ExposureCSVHeader, []string{"", "", "ABC_US_EQ", "US0000000001", "abc_corp"})
	assertSuggestedRecordShape(t, cacheMiss, taxonomy.TickerOverridesCSVHeader, []string{"ABC_US_EQ", "abc_corp"})
	assertSuggestedRecordShape(t, findReviewRow(t, cat.ReviewQueues, ReviewQueueIdentity, ReasonMissingISIN, "NOISIN_US_EQ"), taxonomy.IdentityOverridesCSVHeader, []string{"ticker", "NOISIN_US_EQ"})

	if cat.ReviewSummary.TotalCount != len(cat.ReviewQueues) {
		t.Fatalf("summary total = %d, rows = %d", cat.ReviewSummary.TotalCount, len(cat.ReviewQueues))
	}
	if cat.ReviewSummary.TaxonomyGaps["sector"] == 0 || cat.ReviewSummary.TaxonomyGaps["industry"] == 0 || cat.ReviewSummary.TaxonomyGaps["theme"] == 0 {
		t.Fatalf("taxonomy gap counts = %#v", cat.ReviewSummary.TaxonomyGaps)
	}
	if cat.ReviewSummary.EnrichmentStatuses[enrichment.StatusCacheMiss] != 1 || cat.ReviewSummary.IdentityIssueTypes["duplicate_ticker"] == 0 {
		t.Fatalf("issue type summary = enrichment %#v identity %#v", cat.ReviewSummary.EnrichmentStatuses, cat.ReviewSummary.IdentityIssueTypes)
	}
	if cat.Manifest.ReviewQueueCounts[ReviewQueueTaxonomy] != cat.ReviewSummary.ByQueue[ReviewQueueTaxonomy] || cat.Manifest.ReviewReasonCounts[ReasonMissingSector] != cat.ReviewSummary.ByReasonCode[ReasonMissingSector] {
		t.Fatalf("manifest review counts = %#v %#v summary=%#v", cat.Manifest.ReviewQueueCounts, cat.Manifest.ReviewReasonCounts, cat.ReviewSummary)
	}
	if cat.Manifest.PreviousBuildAt != "2026-05-01T12:00:00Z" {
		t.Fatalf("previous build = %q", cat.Manifest.PreviousBuildAt)
	}
	if cat.Manifest.ReviewQueueDeltas[ReviewQueueTaxonomy] != cat.Manifest.ReviewQueueCounts[ReviewQueueTaxonomy]-1 {
		t.Fatalf("queue deltas = %#v counts=%#v", cat.Manifest.ReviewQueueDeltas, cat.Manifest.ReviewQueueCounts)
	}

	cat2, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cat.ReviewQueues, cat2.ReviewQueues) {
		t.Fatalf("review queue ordering is not deterministic\nfirst=%#v\nsecond=%#v", cat.ReviewQueues, cat2.ReviewQueues)
	}
}

func TestBuildReviewQueuesWithoutPreviousManifest(t *testing.T) {
	input := reviewFixtureInput()
	input.PreviousManifest = nil
	cat, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Manifest.PreviousBuildAt != "" || cat.Manifest.ReviewQueueDeltas != nil || cat.Manifest.ReviewReasonDeltas != nil {
		t.Fatalf("unexpected previous manifest trend fields: %#v", cat.Manifest)
	}
	if len(cat.Manifest.ReviewQueueCounts) == 0 || len(cat.Manifest.ReviewReasonCounts) == 0 {
		t.Fatalf("review counts missing without previous manifest: %#v", cat.Manifest)
	}
}

func TestReviewManifestDeltasPreserveSameBuildTrendFields(t *testing.T) {
	manifest := BuildManifest{
		BuiltAt:            "2026-05-09T12:00:00Z",
		ReviewQueueCounts:  map[string]int{ReviewQueueTaxonomy: 3},
		ReviewReasonCounts: map[string]int{ReasonMissingSector: 2},
	}
	previous := &BuildManifest{
		BuiltAt:            "2026-05-09T12:00:00Z",
		PreviousBuildAt:    "2026-05-01T12:00:00Z",
		ReviewQueueCounts:  map[string]int{ReviewQueueTaxonomy: 3},
		ReviewReasonCounts: map[string]int{ReasonMissingSector: 2},
		ReviewQueueDeltas:  map[string]int{ReviewQueueTaxonomy: 1},
		ReviewReasonDeltas: map[string]int{ReasonMissingSector: 1},
	}
	addReviewManifestDeltas(&manifest, previous)

	if manifest.PreviousBuildAt != previous.PreviousBuildAt ||
		manifest.ReviewQueueDeltas[ReviewQueueTaxonomy] != 1 ||
		manifest.ReviewReasonDeltas[ReasonMissingSector] != 1 {
		t.Fatalf("same-builtAt trend fields were not preserved: %#v", manifest)
	}
	previous.ReviewQueueDeltas[ReviewQueueTaxonomy] = 99
	if manifest.ReviewQueueDeltas[ReviewQueueTaxonomy] != 1 {
		t.Fatalf("trend deltas were not copied defensively: %#v", manifest.ReviewQueueDeltas)
	}
}

func reviewFixtureInput() BuildInput {
	builtAt := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	manual := taxonomy.ManualData{
		Themes: []taxonomy.Theme{{ID: "ai", Name: "AI"}},
		SupplyChains: []taxonomy.SupplyChain{{
			ThemeID: "ai",
			Name:    "AI chain",
			Layers:  []taxonomy.SupplyChainLayer{{ID: "chips", Name: "Chips", Order: 10}},
		}},
		CompanyOverrides: map[string]taxonomy.CompanyOverride{
			"oldco": {
				CompanyID:    "oldco",
				Name:         "Old Co",
				Sector:       "Industrials",
				Industry:     "Machinery",
				LastReviewed: "2026-01-01",
				SourcePath:   "data/manual/company_overrides.csv",
				SourceRow:    2,
			},
		},
		TickerOverrides: map[string]taxonomy.TickerOverride{
			"STALE_US_EQ": {
				Ticker:     "STALE_US_EQ",
				CompanyID:  "stale",
				SourcePath: "data/manual/ticker_overrides.csv",
				SourceRow:  2,
			},
		},
		ClassificationOverrides: []taxonomy.ClassificationOverride{{
			TargetType:   "company",
			CompanyID:    "oldco",
			Sector:       "Industrials",
			LastReviewed: "2026-01-01",
			SourcePath:   "data/manual/classification_overrides.csv",
			SourceRow:    2,
		}},
		IdentityOverrides: []taxonomy.IdentityOverride{{
			TargetType: "ticker",
			Ticker:     "MISSING_US_EQ",
			Category:   CategoryStock,
			SourcePath: "data/manual/identity_overrides.csv",
			SourceRow:  2,
		}},
		Exposures: []taxonomy.Exposure{{
			ThemeID:      "ai",
			LayerID:      "chips",
			Ticker:       "EXPO_US_EQ",
			CompanyID:    "expo",
			Confidence:   "manual_high",
			LastReviewed: "2025-01-01",
			SourcePath:   "data/manual/exposures.csv",
			SourceRow:    2,
		}},
		Relationships: []taxonomy.Relationship{{
			RelationshipType: "peer",
			SourceTicker:     "ABC_US_EQ",
			TargetTicker:     "XYZ_US_EQ",
			ThemeID:          "ai",
			LayerID:          "chips",
			Confidence:       "rule_low",
			LastReviewed:     "2026-02-01",
			SourcePath:       "data/manual/relationships.csv",
			SourceRow:        2,
		}},
		Notes: []taxonomy.Note{{
			TargetType: "theme",
			TargetID:   "ai",
			Title:      "AI note",
			Path:       "data/manual/notes/ai.md",
			Text:       "Review note.",
		}},
	}
	return BuildInput{
		Instruments: []trading212.Instrument{
			{Ticker: "ABC_US_EQ", Name: "ABC Corp", ISIN: "US0000000001", Type: "STOCK", CurrencyCode: "USD"},
			{Ticker: "NOISIN_US_EQ", Name: "No ISIN Corp", Type: "STOCK", CurrencyCode: "USD"},
			{Ticker: "DUP_US_EQ", Name: "Duplicate One", ISIN: "US0000000002", Type: "STOCK", CurrencyCode: "USD"},
			{Ticker: "DUP_US_EQ", Name: "Duplicate Two", ISIN: "US0000000003", Type: "STOCK", CurrencyCode: "USD"},
		},
		Manual:  manual,
		BuiltAt: builtAt,
		EnrichmentFailures: []EnrichmentFailure{
			{Ticker: "ABC_US_EQ", ISIN: "US0000000001", Name: "ABC Corp", Provider: "cache", AttemptedSymbols: "ABC;ABC_US_EQ", Status: enrichment.StatusCacheMiss, Error: "enrichment cache miss", NextAction: "populate cache"},
			{Ticker: "NOISIN_US_EQ", Name: "No ISIN Corp", Provider: "cache", AttemptedSymbols: "NOISIN", Status: enrichment.StatusAmbiguous, Error: "ambiguous enrichment match", NextAction: "add manual ticker override"},
		},
		PreviousManifest: &BuildManifest{
			BuiltAt:            "2026-05-01T12:00:00Z",
			ReviewQueueCounts:  map[string]int{ReviewQueueTaxonomy: 1, ReviewQueueEnrichment: 1},
			ReviewReasonCounts: map[string]int{ReasonMissingSector: 1, ReasonEnrichmentFailed: 2},
		},
	}
}

func findReviewRow(t *testing.T, rows []ReviewQueueRow, queue string, reasonCode string, ticker string) ReviewQueueRow {
	t.Helper()
	for _, row := range rows {
		if row.Queue == queue && row.ReasonCode == reasonCode && row.Ticker == ticker {
			return row
		}
	}
	t.Fatalf("missing review row queue=%s reason=%s ticker=%s in %#v", queue, reasonCode, ticker, rows)
	return ReviewQueueRow{}
}

func findUnclassified(t *testing.T, cat *Catalogue, ticker string) UnclassifiedRow {
	t.Helper()
	for _, row := range cat.Unclassified {
		if row.Ticker == ticker {
			return row
		}
	}
	t.Fatalf("missing unclassified row %s in %#v", ticker, cat.Unclassified)
	return UnclassifiedRow{}
}

func hasReviewReason(rows []ReviewQueueRow, queue string, reason string) bool {
	for _, row := range rows {
		if row.Queue == queue && row.ReasonCode == reason {
			return true
		}
	}
	return false
}

func hasStaleBucket(rows []ReviewQueueRow, bucket string) bool {
	for _, row := range rows {
		if row.Queue == ReviewQueueStale && row.StaleBucket == bucket {
			return true
		}
	}
	return false
}

func assertSuggestedRecordShape(t *testing.T, row ReviewQueueRow, header []string, prefix []string) {
	t.Helper()
	record, err := csv.NewReader(strings.NewReader(row.SuggestedCSVRow + "\n")).Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(record) != len(header) {
		t.Fatalf("%s suggested record has %d fields, want %d: %#v", row.SuggestedManualFile, len(record), len(header), record)
	}
	for i, value := range prefix {
		if record[i] != value {
			t.Fatalf("%s suggested record field %d = %q, want %q: %#v", row.SuggestedManualFile, i, record[i], value, record)
		}
	}
}
