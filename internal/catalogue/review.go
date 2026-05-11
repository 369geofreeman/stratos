package catalogue

import (
	"bytes"
	"encoding/csv"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"statos/internal/enrichment"
	"statos/internal/taxonomy"
)

const (
	ReviewQueueTaxonomy   = "taxonomy"
	ReviewQueueEnrichment = "enrichment"
	ReviewQueueIdentity   = "identity"
	ReviewQueueStale      = "stale_review"

	ReviewSeverityHigh   = "high"
	ReviewSeverityMedium = "medium"
	ReviewSeverityLow    = "low"

	ReasonMissingSector           = "missing_sector"
	ReasonMissingIndustry         = "missing_industry"
	ReasonMissingThemeExposure    = "missing_theme_exposure"
	ReasonMissingCompanyID        = "missing_company_id"
	ReasonMissingISIN             = "missing_isin"
	ReasonMissingTicker           = "missing_ticker"
	ReasonIdentityLowConfidence   = "identity_low_confidence"
	ReasonIdentityCollision       = "identity_collision"
	ReasonIdentityDuplicate       = "identity_duplicate_ticker"
	ReasonIdentityOverrideMiss    = "identity_override_unmatched"
	ReasonIdentityUnknownCategory = "identity_unknown_category"
	ReasonEnrichmentFailed        = "enrichment_failed"
	ReasonEnrichmentAmbiguous     = "enrichment_ambiguous"
	ReasonEnrichmentCacheMiss     = "enrichment_cache_miss"
	ReasonEnrichmentStale         = "enrichment_stale"
	ReasonManualReviewStale       = "manual_review_stale"
)

const (
	manualFileClassificationOverrides = "data/manual/classification_overrides.csv"
	manualFileExposures               = "data/manual/exposures.csv"
	manualFileTickerOverrides         = "data/manual/ticker_overrides.csv"
	manualFileIdentityOverrides       = "data/manual/identity_overrides.csv"
	manualFileCompanyOverrides        = "data/manual/company_overrides.csv"
	manualFileRelationships           = "data/manual/relationships.csv"
)

var defaultStaleReviewThresholdDays = map[string]int{
	"manual_high":   180,
	"manual_medium": 120,
	"rule_low":      60,
}

type ReviewInput struct {
	Tickers            []Ticker
	Unclassified       []UnclassifiedRow
	IdentityIssues     []IdentityIssue
	EnrichmentFailures []EnrichmentFailure
	Manual             taxonomy.ManualData
	BuiltAt            time.Time
}

func BuildReviewQueues(input ReviewInput) []ReviewQueueRow {
	if input.BuiltAt.IsZero() {
		input.BuiltAt = time.Now().UTC()
	}
	tickerByID := map[string]Ticker{}
	for _, ticker := range input.Tickers {
		tickerByID[ticker.Ticker] = ticker
	}

	var rows []ReviewQueueRow
	rows = append(rows, taxonomyReviewRows(input.Unclassified, tickerByID)...)
	rows = append(rows, enrichmentReviewRows(input.EnrichmentFailures, tickerByID)...)
	rows = append(rows, identityReviewRows(input.IdentityIssues, tickerByID)...)
	rows = append(rows, staleReviewRows(input.Manual, input.BuiltAt)...)
	sortReviewRows(rows)
	return rows
}

func BuildReviewSummary(rows []ReviewQueueRow, builtAt time.Time) ReviewSummary {
	summary := ReviewSummary{
		TotalCount:                len(rows),
		ByQueue:                   map[string]int{},
		ByReasonCode:              map[string]int{},
		BySeverity:                map[string]int{},
		TaxonomyGaps:              map[string]int{},
		EnrichmentStatuses:        map[string]int{},
		IdentityIssueTypes:        map[string]int{},
		StaleReviewBuckets:        map[string]int{},
		StaleReviewThresholdDays:  copyStringIntMap(defaultStaleReviewThresholdDays),
		SuggestedManualFileCounts: map[string]int{},
	}
	if !builtAt.IsZero() {
		summary.GeneratedAt = builtAt.UTC().Format(time.RFC3339)
	}
	seenSuggestions := map[string]bool{}
	for _, row := range rows {
		summary.ByQueue[row.Queue]++
		summary.ByReasonCode[row.ReasonCode]++
		summary.BySeverity[row.Severity]++
		if row.SuggestedManualFile != "" && row.SuggestedCSVRow != "" {
			key := row.SuggestedManualFile + "\x00" + row.SuggestedCSVRow
			if !seenSuggestions[key] {
				seenSuggestions[key] = true
				summary.SuggestedManualFileCounts[row.SuggestedManualFile]++
			}
		}
		switch row.Queue {
		case ReviewQueueTaxonomy:
			switch row.ReasonCode {
			case ReasonMissingSector:
				summary.TaxonomyGaps["sector"]++
			case ReasonMissingIndustry:
				summary.TaxonomyGaps["industry"]++
			case ReasonMissingThemeExposure:
				summary.TaxonomyGaps["theme"]++
			}
		case ReviewQueueEnrichment:
			if row.IssueType != "" {
				summary.EnrichmentStatuses[row.IssueType]++
			}
		case ReviewQueueIdentity:
			if row.IssueType != "" {
				summary.IdentityIssueTypes[row.IssueType]++
			}
		case ReviewQueueStale:
			if row.StaleBucket != "" {
				summary.StaleReviewBuckets[row.StaleBucket]++
			}
		}
	}
	return summary
}

func ReasonCodesForUnclassifiedReason(reason string) []string {
	var out []string
	for _, part := range strings.Split(reason, ";") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "missing sector":
			out = appendUnique(out, ReasonMissingSector)
		case "missing industry":
			out = appendUnique(out, ReasonMissingIndustry)
		case "missing theme exposure":
			out = appendUnique(out, ReasonMissingThemeExposure)
		case "missing company id", "missing company_id":
			out = appendUnique(out, ReasonMissingCompanyID)
		case "missing isin":
			out = appendUnique(out, ReasonMissingISIN)
		}
	}
	return out
}

func taxonomyReviewRows(unclassified []UnclassifiedRow, tickerByID map[string]Ticker) []ReviewQueueRow {
	rows := []ReviewQueueRow{}
	for index, row := range unclassified {
		reasonCodes := row.ReasonCodes
		if len(reasonCodes) == 0 {
			reasonCodes = ReasonCodesForUnclassifiedReason(row.Reason)
		}
		ticker := tickerByID[row.Ticker]
		if ticker.Ticker == "" {
			ticker = Ticker{
				Ticker:    row.Ticker,
				CompanyID: row.CompanyID,
				Name:      row.Name,
				ISIN:      row.ISIN,
			}
		}
		for _, code := range reasonCodes {
			review := reviewRowFromTicker(ReviewQueueTaxonomy, code, ticker)
			review.SourceFile = "site/data/unclassified.csv"
			review.SourceRow = index + 2
			review.IssueType = code
			switch code {
			case ReasonMissingSector:
				review.Severity = ReviewSeverityMedium
				review.SuggestedAction = "add reviewed sector and industry fields in classification_overrides.csv"
				addClassificationSuggestion(&review)
			case ReasonMissingIndustry:
				review.Severity = ReviewSeverityMedium
				review.SuggestedAction = "add reviewed industry field in classification_overrides.csv"
				addClassificationSuggestion(&review)
			case ReasonMissingThemeExposure:
				review.Severity = ReviewSeverityLow
				review.SuggestedAction = "add a reviewed theme and layer exposure row"
				addExposureSuggestion(&review)
			case ReasonMissingCompanyID:
				review.Severity = ReviewSeverityHigh
				review.SuggestedAction = "add a manual identity override with a stable company ID"
				addIdentitySuggestion(&review)
			case ReasonMissingISIN:
				review.Severity = ReviewSeverityMedium
				review.SuggestedAction = "add a manual identity override if the ticker should merge with another security"
				addIdentitySuggestion(&review)
			default:
				continue
			}
			rows = append(rows, review)
		}
	}
	return rows
}

func enrichmentReviewRows(failures []EnrichmentFailure, tickerByID map[string]Ticker) []ReviewQueueRow {
	rows := make([]ReviewQueueRow, 0, len(failures))
	for index, failure := range failures {
		ticker := tickerByID[failure.Ticker]
		if ticker.Ticker == "" {
			ticker = Ticker{Ticker: failure.Ticker, ISIN: failure.ISIN, Name: failure.Name}
		}
		code := enrichmentReasonCode(failure)
		review := reviewRowFromTicker(ReviewQueueEnrichment, code, ticker)
		review.SourceFile = "site/data/enrichment_failures.csv"
		review.SourceRow = index + 2
		review.Severity = enrichmentSeverity(code)
		review.IssueType = firstNonEmpty(failure.Status, "failure")
		review.SuggestedAction = firstNonEmpty(failure.NextAction, "retry provider later or add manual ticker override")
		addTickerOverrideSuggestion(&review)
		rows = append(rows, review)
	}
	return rows
}

func enrichmentReasonCode(failure EnrichmentFailure) string {
	switch failure.Status {
	case enrichment.StatusAmbiguous:
		return ReasonEnrichmentAmbiguous
	case enrichment.StatusCacheMiss:
		return ReasonEnrichmentCacheMiss
	}
	message := strings.ToLower(failure.Error + " " + failure.NextAction)
	if strings.Contains(message, "stale") || strings.Contains(message, "unknown schema") {
		return ReasonEnrichmentStale
	}
	return ReasonEnrichmentFailed
}

func enrichmentSeverity(code string) string {
	switch code {
	case ReasonEnrichmentAmbiguous:
		return ReviewSeverityHigh
	case ReasonEnrichmentCacheMiss:
		return ReviewSeverityMedium
	case ReasonEnrichmentStale:
		return ReviewSeverityLow
	default:
		return ReviewSeverityMedium
	}
}

func identityReviewRows(issues []IdentityIssue, tickerByID map[string]Ticker) []ReviewQueueRow {
	rows := make([]ReviewQueueRow, 0, len(issues))
	for index, issue := range issues {
		ticker := tickerByID[issue.Ticker]
		if ticker.Ticker == "" {
			ticker = Ticker{
				Ticker:     issue.Ticker,
				ISIN:       issue.ISIN,
				SecurityID: issue.SecurityID,
				CompanyID:  issue.CompanyID,
				Name:       issue.Name,
			}
		}
		review := reviewRowFromTicker(ReviewQueueIdentity, identityReasonCode(issue.IssueCode), ticker)
		if review.ISIN == "" {
			review.ISIN = issue.ISIN
		}
		if review.SecurityID == "" {
			review.SecurityID = issue.SecurityID
		}
		if review.CompanyID == "" {
			review.CompanyID = issue.CompanyID
		}
		if review.Name == "" {
			review.Name = issue.Name
		}
		review.SourceFile = "site/data/identity_issues.csv"
		review.SourceRow = index + 2
		review.Severity = identitySeverity(review.ReasonCode)
		review.IssueType = issue.IssueCode
		review.SuggestedAction = issue.SuggestedAction
		addIdentitySuggestion(&review)
		rows = append(rows, review)
	}
	return rows
}

func identityReasonCode(issueCode string) string {
	switch issueCode {
	case "missing_ticker":
		return ReasonMissingTicker
	case "missing_isin":
		return ReasonMissingISIN
	case "duplicate_ticker":
		return ReasonIdentityDuplicate
	case "low_confidence_company_identity", "broker_ticker_parse_uncertain":
		return ReasonIdentityLowConfidence
	case "unknown_instrument_category":
		return ReasonIdentityUnknownCategory
	case "shared_isin_multiple_companies", "security_id_multiple_isins", "security_id_multiple_companies", "security_category_collision", "manual_override_conflict":
		return ReasonIdentityCollision
	default:
		if strings.HasPrefix(issueCode, "manual_override_unknown_") {
			return ReasonIdentityOverrideMiss
		}
		return ReasonIdentityLowConfidence
	}
}

func identitySeverity(reasonCode string) string {
	switch reasonCode {
	case ReasonIdentityCollision, ReasonIdentityDuplicate, ReasonMissingTicker:
		return ReviewSeverityHigh
	case ReasonIdentityOverrideMiss, ReasonMissingISIN, ReasonIdentityUnknownCategory:
		return ReviewSeverityMedium
	default:
		return ReviewSeverityMedium
	}
}

func staleReviewRows(manual taxonomy.ManualData, builtAt time.Time) []ReviewQueueRow {
	var rows []ReviewQueueRow
	for _, row := range sortedCompanyOverrides(manual.CompanyOverrides) {
		if bucket, stale := staleBucket(row.LastReviewed, "manual_medium", builtAt); stale {
			rows = append(rows, staleReviewRow(ReviewQueueRow{
				CompanyID:           row.CompanyID,
				Name:                row.Name,
				Sector:              row.Sector,
				Industry:            row.Industry,
				SourceFile:          defaultSourceFile(row.SourcePath, manualFileCompanyOverrides),
				SourceRow:           row.SourceRow,
				SuggestedManualFile: manualFileCompanyOverrides,
				LastReviewed:        row.LastReviewed,
			}, bucket, "review company override and update last_reviewed"))
		}
	}
	for _, row := range sortedTickerOverrides(manual.TickerOverrides) {
		if bucket, stale := staleBucket(row.LastReviewed, "manual_medium", builtAt); stale {
			rows = append(rows, staleReviewRow(ReviewQueueRow{
				Ticker:              row.Ticker,
				CompanyID:           row.CompanyID,
				Name:                row.Name,
				Sector:              row.Sector,
				Industry:            row.Industry,
				SourceFile:          defaultSourceFile(row.SourcePath, manualFileTickerOverrides),
				SourceRow:           row.SourceRow,
				SuggestedManualFile: manualFileTickerOverrides,
				LastReviewed:        row.LastReviewed,
			}, bucket, "review ticker override and update last_reviewed"))
		}
	}
	for _, row := range manual.ClassificationOverrides {
		if bucket, stale := staleBucket(row.LastReviewed, "manual_medium", builtAt); stale {
			rows = append(rows, staleReviewRow(ReviewQueueRow{
				Ticker:              row.Ticker,
				ISIN:                row.ISIN,
				CompanyID:           row.CompanyID,
				Sector:              row.Sector,
				Industry:            row.Industry,
				SourceFile:          defaultSourceFile(row.SourcePath, manualFileClassificationOverrides),
				SourceRow:           row.SourceRow,
				SuggestedManualFile: manualFileClassificationOverrides,
				LastReviewed:        row.LastReviewed,
			}, bucket, "review classification override and update last_reviewed"))
		}
	}
	for _, row := range manual.Exposures {
		if bucket, stale := staleBucket(row.LastReviewed, row.Confidence, builtAt); stale {
			rows = append(rows, staleReviewRow(ReviewQueueRow{
				Ticker:              row.Ticker,
				ISIN:                row.ISIN,
				CompanyID:           row.CompanyID,
				ThemeIDs:            optionalStringSlice(row.ThemeID),
				LayerIDs:            optionalStringSlice(row.LayerID),
				SourceFile:          defaultSourceFile(row.SourcePath, manualFileExposures),
				SourceRow:           row.SourceRow,
				SuggestedManualFile: manualFileExposures,
				LastReviewed:        row.LastReviewed,
			}, bucket, "review exposure confidence/source and update last_reviewed"))
		}
	}
	for _, row := range manual.IdentityOverrides {
		if bucket, stale := staleBucket(row.LastReviewed, row.Confidence, builtAt); stale {
			rows = append(rows, staleReviewRow(ReviewQueueRow{
				Ticker:              row.Ticker,
				ISIN:                row.ISIN,
				CompanyID:           row.CompanyID,
				SecurityID:          row.SecurityID,
				SourceFile:          defaultSourceFile(row.SourcePath, manualFileIdentityOverrides),
				SourceRow:           row.SourceRow,
				SuggestedManualFile: manualFileIdentityOverrides,
				LastReviewed:        row.LastReviewed,
			}, bucket, "review identity override and update last_reviewed"))
		}
	}
	for _, row := range manual.Relationships {
		if bucket, stale := staleBucket(row.LastReviewed, row.Confidence, builtAt); stale {
			rows = append(rows, staleReviewRow(ReviewQueueRow{
				Ticker:              firstNonEmpty(row.SourceTicker, row.TargetTicker),
				ISIN:                firstNonEmpty(row.SourceISIN, row.TargetISIN),
				CompanyID:           firstNonEmpty(row.SourceCompanyID, row.TargetCompanyID),
				ThemeIDs:            optionalStringSlice(row.ThemeID),
				LayerIDs:            optionalStringSlice(row.LayerID),
				SourceFile:          defaultSourceFile(row.SourcePath, manualFileRelationships),
				SourceRow:           row.SourceRow,
				SuggestedManualFile: manualFileRelationships,
				LastReviewed:        row.LastReviewed,
				IssueType:           row.RelationshipType,
			}, bucket, "review relationship row and update last_reviewed"))
		}
	}
	for _, note := range manual.Notes {
		if bucket, stale := staleBucket(note.LastReviewed, "manual_medium", builtAt); stale {
			rows = append(rows, staleReviewRow(ReviewQueueRow{
				CompanyID:           note.TargetID,
				Name:                note.Title,
				SourceFile:          filepath.ToSlash(note.Path),
				SourceRow:           1,
				SuggestedManualFile: filepath.ToSlash(note.Path),
				LastReviewed:        note.LastReviewed,
				IssueType:           "note_" + note.TargetType,
			}, bucket, "review note frontmatter and update last_reviewed"))
		}
	}
	return rows
}

func staleReviewRow(row ReviewQueueRow, bucket string, action string) ReviewQueueRow {
	row.Queue = ReviewQueueStale
	row.ReasonCode = ReasonManualReviewStale
	row.Severity = staleSeverity(bucket)
	row.StaleBucket = bucket
	row.IssueType = firstNonEmpty(row.IssueType, bucket)
	row.SuggestedAction = action
	return row
}

func staleBucket(lastReviewed string, confidence string, builtAt time.Time) (string, bool) {
	lastReviewed = strings.TrimSpace(lastReviewed)
	if lastReviewed == "" {
		return "missing_last_reviewed", true
	}
	parsed, err := time.Parse("2006-01-02", lastReviewed)
	if err != nil {
		return "invalid_last_reviewed", true
	}
	bucket := staleThresholdBucket(confidence)
	threshold := staleThresholdDays(confidence)
	ageDays := int(builtAt.UTC().Truncate(24*time.Hour).Sub(parsed.UTC().Truncate(24*time.Hour)).Hours() / 24)
	if ageDays >= threshold {
		return bucket, true
	}
	return "", false
}

func staleThresholdBucket(confidence string) string {
	switch confidence {
	case "manual_high", "rule_high":
		return "manual_high_over_180d"
	case "manual_low", "rule_low":
		return "rule_low_over_60d"
	default:
		return "manual_medium_over_120d"
	}
}

func staleThresholdDays(confidence string) int {
	switch confidence {
	case "manual_high", "rule_high":
		return defaultStaleReviewThresholdDays["manual_high"]
	case "manual_low", "rule_low":
		return defaultStaleReviewThresholdDays["rule_low"]
	default:
		return defaultStaleReviewThresholdDays["manual_medium"]
	}
}

func staleSeverity(bucket string) string {
	switch bucket {
	case "missing_last_reviewed", "invalid_last_reviewed":
		return ReviewSeverityMedium
	default:
		return ReviewSeverityLow
	}
}

func reviewRowFromTicker(queue string, reasonCode string, ticker Ticker) ReviewQueueRow {
	return ReviewQueueRow{
		Queue:         queue,
		ReasonCode:    reasonCode,
		Ticker:        ticker.Ticker,
		ISIN:          ticker.ISIN,
		CompanyID:     ticker.CompanyID,
		SecurityID:    ticker.SecurityID,
		Name:          ticker.Name,
		Sector:        ticker.Sector,
		Industry:      ticker.Industry,
		ThemeIDs:      append([]string(nil), ticker.ThemeIDs...),
		LayerIDs:      append([]string(nil), ticker.LayerIDs...),
		LastReviewed:  ticker.LastReviewed,
		LastRefreshed: ticker.LastRefreshed,
	}
}

func addClassificationSuggestion(row *ReviewQueueRow) {
	targetType, fields := manualTarget(row.Ticker, row.ISIN, row.SecurityID, row.CompanyID)
	if targetType == "" {
		return
	}
	record := []string{targetType, fields["ticker"], fields["isin"], fields["company_id"], "", "", "", "", ""}
	setSuggestion(row, manualFileClassificationOverrides, record)
}

func addExposureSuggestion(row *ReviewQueueRow) {
	if countReviewNonEmpty(row.Ticker, row.ISIN, row.CompanyID) == 0 {
		return
	}
	record := []string{"", "", row.Ticker, row.ISIN, row.CompanyID, "", "", "", "", ""}
	setSuggestion(row, manualFileExposures, record)
}

func addTickerOverrideSuggestion(row *ReviewQueueRow) {
	if row.Ticker == "" {
		return
	}
	record := []string{row.Ticker, row.CompanyID, "", "", "", "", "", "", "", "", "", ""}
	setSuggestion(row, manualFileTickerOverrides, record)
}

func addIdentitySuggestion(row *ReviewQueueRow) {
	targetType, fields := manualTarget(row.Ticker, row.ISIN, row.SecurityID, row.CompanyID)
	if targetType == "" {
		return
	}
	record := []string{targetType, fields["ticker"], fields["isin"], fields["security_id"], fields["company_id"], "", "", "", "", "", "", "", ""}
	setSuggestion(row, manualFileIdentityOverrides, record)
}

func manualTarget(ticker string, isin string, securityID string, companyID string) (string, map[string]string) {
	fields := map[string]string{
		"ticker":      "",
		"isin":        "",
		"security_id": "",
		"company_id":  "",
	}
	switch {
	case ticker != "":
		fields["ticker"] = ticker
		return "ticker", fields
	case isin != "":
		fields["isin"] = isin
		return "isin", fields
	case securityID != "":
		fields["security_id"] = securityID
		return "security", fields
	case companyID != "":
		fields["company_id"] = companyID
		return "company", fields
	default:
		return "", fields
	}
}

func setSuggestion(row *ReviewQueueRow, manualFile string, record []string) {
	row.SuggestedManualFile = manualFile
	row.suggestedCSVFields = append([]string(nil), record...)
	row.SuggestedCSVRow = csvRecordString(record)
}

func csvRecordString(record []string) string {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.Write(record)
	writer.Flush()
	return strings.TrimRight(buf.String(), "\r\n")
}

func sortReviewRows(rows []ReviewQueueRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		a := reviewSortKey(rows[i])
		b := reviewSortKey(rows[j])
		return a < b
	})
}

func reviewSortKey(row ReviewQueueRow) string {
	return strings.Join([]string{
		reviewQueueSort(row.Queue),
		severitySort(row.Severity),
		row.ReasonCode,
		row.Ticker,
		row.ISIN,
		row.SecurityID,
		row.CompanyID,
		row.SourceFile,
		intSort(row.SourceRow),
		row.Name,
	}, "\x00")
}

func reviewQueueSort(queue string) string {
	switch queue {
	case ReviewQueueTaxonomy:
		return "1"
	case ReviewQueueEnrichment:
		return "2"
	case ReviewQueueIdentity:
		return "3"
	case ReviewQueueStale:
		return "4"
	default:
		return "9" + queue
	}
}

func severitySort(severity string) string {
	switch severity {
	case ReviewSeverityHigh:
		return "1"
	case ReviewSeverityMedium:
		return "2"
	case ReviewSeverityLow:
		return "3"
	default:
		return "9" + severity
	}
}

func intSort(value int) string {
	if value < 0 {
		value = 0
	}
	return strings.Repeat("0", 10-len(strconv.Itoa(value))) + strconv.Itoa(value)
}

func addReviewManifestDeltas(manifest *BuildManifest, previous *BuildManifest) {
	if manifest.ReviewQueueCounts == nil {
		manifest.ReviewQueueCounts = map[string]int{}
	}
	if manifest.ReviewReasonCounts == nil {
		manifest.ReviewReasonCounts = map[string]int{}
	}
	if previous == nil {
		return
	}
	if previous.BuiltAt == "" {
		return
	}
	if previous.BuiltAt == manifest.BuiltAt {
		manifest.PreviousBuildAt = previous.PreviousBuildAt
		if previous.ReviewQueueDeltas != nil {
			manifest.ReviewQueueDeltas = copyStringIntMap(previous.ReviewQueueDeltas)
		}
		if previous.ReviewReasonDeltas != nil {
			manifest.ReviewReasonDeltas = copyStringIntMap(previous.ReviewReasonDeltas)
		}
		return
	}
	manifest.PreviousBuildAt = previous.BuiltAt
	manifest.ReviewQueueDeltas = countDeltas(manifest.ReviewQueueCounts, previous.ReviewQueueCounts)
	manifest.ReviewReasonDeltas = countDeltas(manifest.ReviewReasonCounts, previous.ReviewReasonCounts)
}

func countDeltas(current map[string]int, previous map[string]int) map[string]int {
	out := map[string]int{}
	keys := map[string]bool{}
	for key := range current {
		keys[key] = true
	}
	for key := range previous {
		keys[key] = true
	}
	for key := range keys {
		out[key] = current[key] - previous[key]
	}
	return out
}

func copyStringIntMap(in map[string]int) map[string]int {
	out := map[string]int{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func sortedCompanyOverrides(rows map[string]taxonomy.CompanyOverride) []taxonomy.CompanyOverride {
	out := make([]taxonomy.CompanyOverride, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CompanyID < out[j].CompanyID })
	return out
}

func sortedTickerOverrides(rows map[string]taxonomy.TickerOverride) []taxonomy.TickerOverride {
	out := make([]taxonomy.TickerOverride, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Ticker < out[j].Ticker })
	return out
}

func defaultSourceFile(path string, fallback string) string {
	if path == "" {
		return fallback
	}
	return filepath.ToSlash(path)
}

func optionalStringSlice(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

func countReviewNonEmpty(values ...string) int {
	count := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}
