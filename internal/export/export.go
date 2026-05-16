package export

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"statos/internal/catalogue"
	"statos/internal/taxonomy"
)

var TickersCSVHeader = []string{"ticker", "name", "isin", "company_id", "security_id", "type", "instrument_category", "structure_flags", "currency", "exchange", "yahoo_symbol", "sector", "industry", "country", "market_cap", "directionality", "identity_confidence", "identity_reasons", "themes", "layers", "unclassified"}
var SecuritiesCSVHeader = []string{"security_id", "isin", "name", "type", "instrument_category", "structure_flags", "company_id", "listing_ids", "ticker_ids", "currency_set", "identity_confidence", "identity_reasons"}
var ListingsCSVHeader = []string{"listing_id", "ticker", "security_id", "company_id", "exchange_code", "exchange_name", "currency_code"}
var UnclassifiedCSVHeader = []string{"ticker", "company_id", "name", "isin", "reason", "reason_codes"}
var IdentityIssuesCSVHeader = []string{"issue_code", "ticker", "isin", "security_id", "company_id", "name", "reason", "suggested_action"}
var EnrichmentFailuresCSVHeader = []string{"ticker", "isin", "name", "provider", "attempted_symbols", "status", "error", "next_action"}
var ReviewIssuesCSVHeader = []string{"queue", "reason_code", "severity", "ticker", "isin", "company_id", "security_id", "name", "sector", "industry", "theme_ids", "layer_ids", "source_file", "source_row", "suggested_action", "suggested_manual_file", "suggested_csv_row", "last_reviewed", "last_refreshed"}

var GeneratedSiteDataFiles = []string{
	"app_bootstrap.json",
	"tickers_index.json",
	"explorer_index.json",
	"catalogue.json",
	"companies.json",
	"sectors.json",
	"industries.json",
	"themes.json",
	"supply_chains.json",
	"search_index.json",
	"securities.json",
	"listings.json",
	"relationships.json",
	"unclassified.json",
	"review_queues.json",
	"review_summary.json",
	"tickers.csv",
	"securities.csv",
	"listings.csv",
	"unclassified.csv",
	"taxonomy_issues.csv",
	"enrichment_issues.csv",
	"identity_issues.csv",
	"enrichment_failures.csv",
	"stale_reviews.csv",
	"suggested_classification_overrides.csv",
	"suggested_exposures.csv",
	"suggested_ticker_overrides.csv",
	"suggested_identity_overrides.csv",
	"build_manifest.json",
}

const buildManifestChecksumMode = "projection_excludes_generatedFiles"
const appBootstrapChecksumMode = "projection_excludes_generatedFiles"
const groupSummaryTickerLimit = 12
const manualFileClassificationOverrides = "data/manual/classification_overrides.csv"
const manualFileExposures = "data/manual/exposures.csv"
const manualFileTickerOverrides = "data/manual/ticker_overrides.csv"
const manualFileIdentityOverrides = "data/manual/identity_overrides.csv"

type generatedOutput struct {
	name          string
	format        string
	schemaVersion int
	bytes         []byte
}

type AppBootstrap struct {
	DataContractVersion int                       `json:"dataContractVersion"`
	SchemaVersion       int                       `json:"schemaVersion"`
	GeneratedAt         string                    `json:"generatedAt,omitempty"`
	Manifest            catalogue.BuildManifest   `json:"manifest"`
	Themes              []taxonomy.Theme          `json:"themes"`
	SupplyChains        []taxonomy.SupplyChain    `json:"supplyChains"`
	Exposures           []taxonomy.Exposure       `json:"exposures,omitempty"`
	ThemeCounts         map[string]int            `json:"themeCounts,omitempty"`
	Sectors             []GroupSummary            `json:"sectors"`
	Industries          []GroupSummary            `json:"industries"`
	Counts              AppBootstrapCounts        `json:"counts"`
	GeneratedFiles      []catalogue.GeneratedFile `json:"generatedFiles,omitempty"`
}

type AppBootstrapCounts struct {
	TickerCount       int `json:"tickerCount"`
	CompanyCount      int `json:"companyCount"`
	SecurityCount     int `json:"securityCount"`
	ListingCount      int `json:"listingCount"`
	ThemeCount        int `json:"themeCount"`
	SupplyChainCount  int `json:"supplyChainCount"`
	ExposureCount     int `json:"exposureCount"`
	RelationshipCount int `json:"relationshipCount"`
	UnclassifiedCount int `json:"unclassifiedCount"`
}

type GroupSummary struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Count   int      `json:"count"`
	Tickers []string `json:"tickers,omitempty"`
}

type TickerIndex struct {
	DataContractVersion int              `json:"dataContractVersion"`
	SchemaVersion       int              `json:"schemaVersion"`
	GeneratedAt         string           `json:"generatedAt,omitempty"`
	Tickers             []TickerIndexRow `json:"tickers"`
}

type TickerIndexRow struct {
	Ticker             string   `json:"ticker"`
	Name               string   `json:"name"`
	ISIN               string   `json:"isin,omitempty"`
	CompanyID          string   `json:"companyId"`
	SecurityID         string   `json:"securityId"`
	ListingID          string   `json:"listingId"`
	Type               string   `json:"type,omitempty"`
	InstrumentCategory string   `json:"instrumentCategory,omitempty"`
	StructureFlags     []string `json:"structureFlags,omitempty"`
	CurrencyCode       string   `json:"currencyCode,omitempty"`
	ExchangeCode       string   `json:"exchangeCode,omitempty"`
	ExchangeName       string   `json:"exchangeName,omitempty"`
	YahooSymbol        string   `json:"yahooSymbol,omitempty"`
	Sector             string   `json:"sector,omitempty"`
	Industry           string   `json:"industry,omitempty"`
	Country            string   `json:"country,omitempty"`
	MarketCap          int64    `json:"marketCap,omitempty"`
	Directionality     string   `json:"directionality,omitempty"`
	IdentityConfidence string   `json:"identityConfidence,omitempty"`
	ThemeIDs           []string `json:"themeIds,omitempty"`
	LayerIDs           []string `json:"layerIds,omitempty"`
	RelatedTickers     []string `json:"relatedTickers,omitempty"`
	LastReviewed       string   `json:"lastReviewed,omitempty"`
	LastRefreshed      string   `json:"lastRefreshed,omitempty"`
	Unclassified       bool     `json:"unclassified"`
}

type SearchDocument struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle,omitempty"`
	Text     string   `json:"text"`
	Tickers  []string `json:"tickers,omitempty"`
}

type ExplorerIndex struct {
	DataContractVersion int             `json:"dataContractVersion"`
	SchemaVersion       int             `json:"schemaVersion"`
	GeneratedAt         string          `json:"generatedAt,omitempty"`
	Groups              []ExplorerGroup `json:"groups"`
}

type ExplorerGroup struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Value       string   `json:"value,omitempty"`
	Label       string   `json:"label"`
	ParentID    string   `json:"parentId,omitempty"`
	ParentLabel string   `json:"parentLabel,omitempty"`
	Description string   `json:"description,omitempty"`
	Count       int      `json:"count"`
	EdgeCount   int      `json:"edgeCount,omitempty"`
	Tickers     []string `json:"tickers"`
}

type CatalogueIndex struct {
	DataContractVersion int                     `json:"dataContractVersion"`
	SchemaVersion       int                     `json:"schemaVersion"`
	GeneratedAt         string                  `json:"generatedAt,omitempty"`
	Manifest            catalogue.BuildManifest `json:"manifest"`
	Counts              AppBootstrapCounts      `json:"counts"`
	Slices              []CatalogueSlice        `json:"slices"`
}

type CatalogueSlice struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Format string `json:"format"`
}

func CSVHeaders() map[string][]string {
	headers := map[string][]string{
		"tickers.csv":             TickersCSVHeader,
		"securities.csv":          SecuritiesCSVHeader,
		"listings.csv":            ListingsCSVHeader,
		"unclassified.csv":        UnclassifiedCSVHeader,
		"taxonomy_issues.csv":     ReviewIssuesCSVHeader,
		"enrichment_issues.csv":   ReviewIssuesCSVHeader,
		"identity_issues.csv":     IdentityIssuesCSVHeader,
		"enrichment_failures.csv": EnrichmentFailuresCSVHeader,
		"stale_reviews.csv":       ReviewIssuesCSVHeader,
	}
	headers["suggested_classification_overrides.csv"] = taxonomy.ClassificationOverridesCSVHeader
	headers["suggested_exposures.csv"] = taxonomy.ExposureCSVHeader
	headers["suggested_ticker_overrides.csv"] = taxonomy.TickerOverridesCSVHeader
	headers["suggested_identity_overrides.csv"] = taxonomy.IdentityOverridesCSVHeader
	out := map[string][]string{}
	for name, header := range headers {
		out[name] = append([]string(nil), header...)
	}
	return out
}

func WriteSiteData(dir string, cat *catalogue.Catalogue) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	outputs, err := buildSiteDataOutputs(cat)
	if err != nil {
		return err
	}
	for _, output := range outputs {
		if err := os.WriteFile(filepath.Join(dir, output.name), output.bytes, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func buildSiteDataOutputs(cat *catalogue.Catalogue) ([]generatedOutput, error) {
	if cat == nil {
		return nil, fmt.Errorf("catalogue is nil")
	}
	contractCat := catalogueWithContractMetadata(cat)

	defs := []struct {
		name  string
		value any
	}{
		{"tickers_index.json", BuildTickerIndex(&contractCat)},
		{"explorer_index.json", BuildExplorerIndex(&contractCat)},
		{"catalogue.json", BuildCatalogueIndex(&contractCat)},
		{"companies.json", contractCat.Companies},
		{"sectors.json", contractCat.Sectors},
		{"industries.json", contractCat.Industries},
		{"themes.json", contractCat.Themes},
		{"supply_chains.json", contractCat.SupplyChains},
		{"search_index.json", BuildSearchIndex(&contractCat)},
		{"securities.json", contractCat.Securities},
		{"listings.json", contractCat.Listings},
		{"relationships.json", contractCat.Relationships},
		{"unclassified.json", contractCat.Unclassified},
		{"review_queues.json", contractCat.ReviewQueues},
		{"review_summary.json", contractCat.ReviewSummary},
	}
	outputByName := map[string]generatedOutput{}
	for _, def := range defs {
		marshal := marshalJSON
		if def.name == "tickers_index.json" || def.name == "explorer_index.json" || def.name == "unclassified.json" || def.name == "review_queues.json" {
			marshal = marshalCompactJSON
		}
		b, err := marshal(def.value)
		if err != nil {
			return nil, err
		}
		outputByName[def.name] = generatedOutput{name: def.name, format: "json", schemaVersion: catalogue.DataContractSchemaVersion, bytes: b}
	}

	csvDefs := []struct {
		name string
		data []byte
		err  error
	}{
		{name: "tickers.csv"},
		{name: "securities.csv"},
		{name: "listings.csv"},
		{name: "unclassified.csv"},
		{name: "taxonomy_issues.csv"},
		{name: "enrichment_issues.csv"},
		{name: "identity_issues.csv"},
		{name: "enrichment_failures.csv"},
		{name: "stale_reviews.csv"},
		{name: "suggested_classification_overrides.csv"},
		{name: "suggested_exposures.csv"},
		{name: "suggested_ticker_overrides.csv"},
		{name: "suggested_identity_overrides.csv"},
	}
	csvDefs[0].data, csvDefs[0].err = marshalTickersCSV(contractCat.Tickers)
	csvDefs[1].data, csvDefs[1].err = marshalSecuritiesCSV(contractCat.Securities)
	csvDefs[2].data, csvDefs[2].err = marshalListingsCSV(contractCat.Listings)
	csvDefs[3].data, csvDefs[3].err = marshalUnclassifiedCSV(contractCat.Unclassified)
	csvDefs[4].data, csvDefs[4].err = marshalReviewIssuesCSV(contractCat.ReviewQueues, catalogue.ReviewQueueTaxonomy)
	csvDefs[5].data, csvDefs[5].err = marshalReviewIssuesCSV(contractCat.ReviewQueues, catalogue.ReviewQueueEnrichment)
	csvDefs[6].data, csvDefs[6].err = marshalIdentityIssuesCSV(contractCat.IdentityIssues)
	csvDefs[7].data, csvDefs[7].err = marshalEnrichmentFailuresCSV(contractCat.EnrichmentFailures)
	csvDefs[8].data, csvDefs[8].err = marshalReviewIssuesCSV(contractCat.ReviewQueues, catalogue.ReviewQueueStale)
	csvDefs[9].data, csvDefs[9].err = marshalSuggestedManualCSV(contractCat.ReviewQueues, manualFileClassificationOverrides, taxonomy.ClassificationOverridesCSVHeader)
	csvDefs[10].data, csvDefs[10].err = marshalSuggestedManualCSV(contractCat.ReviewQueues, manualFileExposures, taxonomy.ExposureCSVHeader)
	csvDefs[11].data, csvDefs[11].err = marshalSuggestedManualCSV(contractCat.ReviewQueues, manualFileTickerOverrides, taxonomy.TickerOverridesCSVHeader)
	csvDefs[12].data, csvDefs[12].err = marshalSuggestedManualCSV(contractCat.ReviewQueues, manualFileIdentityOverrides, taxonomy.IdentityOverridesCSVHeader)
	for _, def := range csvDefs {
		if def.err != nil {
			return nil, def.err
		}
		outputByName[def.name] = generatedOutput{name: def.name, format: "csv", schemaVersion: catalogue.DataContractSchemaVersion, bytes: def.data}
	}

	files, err := generatedFileMetadataForContract(contractCat, outputByName)
	if err != nil {
		return nil, err
	}
	appBootstrapBytes, manifestBytes, files, err := marshalCircularMetadataOutputs(contractCat, files)
	if err != nil {
		return nil, err
	}
	outputByName["app_bootstrap.json"] = generatedOutput{name: "app_bootstrap.json", format: "json", schemaVersion: catalogue.DataContractSchemaVersion, bytes: appBootstrapBytes}
	outputByName["build_manifest.json"] = generatedOutput{name: "build_manifest.json", format: "json", schemaVersion: catalogue.DataContractSchemaVersion, bytes: manifestBytes}

	outputs := make([]generatedOutput, 0, len(GeneratedSiteDataFiles))
	for _, name := range GeneratedSiteDataFiles {
		output, ok := outputByName[name]
		if !ok {
			return nil, fmt.Errorf("missing generated output %s", name)
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

func BuildCatalogueIndex(cat *catalogue.Catalogue) CatalogueIndex {
	slices := make([]CatalogueSlice, 0, len(GeneratedSiteDataFiles)-1)
	for _, name := range GeneratedSiteDataFiles {
		if name == "catalogue.json" {
			continue
		}
		slices = append(slices, CatalogueSlice{
			Name:   name,
			Path:   canonicalSiteDataPath(name),
			Format: generatedFileFormat(name),
		})
	}
	return CatalogueIndex{
		DataContractVersion: catalogue.DataContractVersion,
		SchemaVersion:       catalogue.DataContractSchemaVersion,
		GeneratedAt:         cat.GeneratedAt,
		Manifest:            cat.Manifest,
		Counts:              appBootstrapCounts(cat),
		Slices:              slices,
	}
}

func BuildTickerIndex(cat *catalogue.Catalogue) TickerIndex {
	rows := make([]TickerIndexRow, 0, len(cat.Tickers))
	for _, ticker := range cat.Tickers {
		rows = append(rows, TickerIndexRow{
			Ticker:             ticker.Ticker,
			Name:               ticker.Name,
			ISIN:               ticker.ISIN,
			CompanyID:          ticker.CompanyID,
			SecurityID:         ticker.SecurityID,
			ListingID:          ticker.ListingID,
			Type:               ticker.Type,
			InstrumentCategory: ticker.InstrumentCategory,
			StructureFlags:     ticker.StructureFlags,
			CurrencyCode:       ticker.CurrencyCode,
			ExchangeCode:       ticker.ExchangeCode,
			ExchangeName:       ticker.ExchangeName,
			YahooSymbol:        ticker.YahooSymbol,
			Sector:             ticker.Sector,
			Industry:           ticker.Industry,
			Country:            ticker.Country,
			MarketCap:          ticker.MarketCap,
			Directionality:     ticker.Directionality,
			IdentityConfidence: ticker.IdentityConfidence,
			ThemeIDs:           ticker.ThemeIDs,
			LayerIDs:           ticker.LayerIDs,
			RelatedTickers:     ticker.RelatedTickers,
			LastReviewed:       ticker.LastReviewed,
			LastRefreshed:      ticker.LastRefreshed,
			Unclassified:       ticker.Unclassified,
		})
	}
	return TickerIndex{
		DataContractVersion: catalogue.DataContractVersion,
		SchemaVersion:       catalogue.DataContractSchemaVersion,
		GeneratedAt:         cat.GeneratedAt,
		Tickers:             rows,
	}
}

func BuildExplorerIndex(cat *catalogue.Catalogue) ExplorerIndex {
	if cat == nil {
		return ExplorerIndex{
			DataContractVersion: catalogue.DataContractVersion,
			SchemaVersion:       catalogue.DataContractSchemaVersion,
		}
	}
	builder := newExplorerIndexBuilder(cat)
	builder.addSectorGroups()
	builder.addIndustryGroups()
	builder.addCategoryGroups()
	builder.addStructureFlagGroups()
	builder.addThemeGroups()
	builder.addLayerGroups()
	builder.addRelationshipGroups()
	return ExplorerIndex{
		DataContractVersion: catalogue.DataContractVersion,
		SchemaVersion:       catalogue.DataContractSchemaVersion,
		GeneratedAt:         cat.GeneratedAt,
		Groups:              builder.groups(),
	}
}

type explorerIndexBuilder struct {
	cat              *catalogue.Catalogue
	groupsByID       map[string]*ExplorerGroup
	themeByID        map[string]taxonomy.Theme
	tickerByID       map[string]bool
	tickersByCompany map[string][]string
	tickersByISIN    map[string][]string
}

func newExplorerIndexBuilder(cat *catalogue.Catalogue) *explorerIndexBuilder {
	builder := &explorerIndexBuilder{
		cat:              cat,
		groupsByID:       map[string]*ExplorerGroup{},
		themeByID:        map[string]taxonomy.Theme{},
		tickerByID:       map[string]bool{},
		tickersByCompany: map[string][]string{},
		tickersByISIN:    map[string][]string{},
	}
	for _, theme := range cat.Themes {
		builder.themeByID[theme.ID] = theme
	}
	for _, company := range cat.Companies {
		builder.tickersByCompany[company.ID] = appendUniqueStrings(builder.tickersByCompany[company.ID], company.TickerIDs...)
	}
	for _, ticker := range cat.Tickers {
		builder.tickerByID[ticker.Ticker] = true
		if ticker.CompanyID != "" {
			builder.tickersByCompany[ticker.CompanyID] = appendUniqueStrings(builder.tickersByCompany[ticker.CompanyID], ticker.Ticker)
		}
		if ticker.ISIN != "" {
			builder.tickersByISIN[ticker.ISIN] = appendUniqueStrings(builder.tickersByISIN[ticker.ISIN], ticker.Ticker)
		}
	}
	return builder
}

func (builder *explorerIndexBuilder) addSectorGroups() {
	for _, group := range builder.cat.Sectors {
		builder.addGroup(ExplorerGroup{
			ID:      "sector:" + group.ID,
			Type:    "sector",
			Value:   group.Name,
			Label:   group.Name,
			Tickers: group.Tickers,
		})
	}
}

func (builder *explorerIndexBuilder) addIndustryGroups() {
	for _, group := range builder.cat.Industries {
		builder.addGroup(ExplorerGroup{
			ID:      "industry:" + group.ID,
			Type:    "industry",
			Value:   group.Name,
			Label:   group.Name,
			Tickers: group.Tickers,
		})
	}
}

func (builder *explorerIndexBuilder) addCategoryGroups() {
	byCategory := map[string][]string{}
	for _, ticker := range builder.cat.Tickers {
		if ticker.InstrumentCategory == "" {
			continue
		}
		byCategory[ticker.InstrumentCategory] = appendUniqueStrings(byCategory[ticker.InstrumentCategory], ticker.Ticker)
	}
	for category, tickers := range byCategory {
		builder.addGroup(ExplorerGroup{
			ID:      "category:" + category,
			Type:    "category",
			Value:   category,
			Label:   humanizeIdentifier(category),
			Tickers: tickers,
		})
	}
}

func (builder *explorerIndexBuilder) addStructureFlagGroups() {
	byFlag := map[string][]string{}
	for _, ticker := range builder.cat.Tickers {
		for _, flag := range ticker.StructureFlags {
			byFlag[flag] = appendUniqueStrings(byFlag[flag], ticker.Ticker)
		}
	}
	for flag, tickers := range byFlag {
		builder.addGroup(ExplorerGroup{
			ID:      "flag:" + flag,
			Type:    "flag",
			Value:   flag,
			Label:   humanizeIdentifier(flag),
			Tickers: tickers,
		})
	}
}

func (builder *explorerIndexBuilder) addThemeGroups() {
	for _, theme := range builder.cat.Themes {
		var tickers []string
		for _, ticker := range builder.cat.Tickers {
			if containsString(ticker.ThemeIDs, theme.ID) {
				tickers = appendUniqueStrings(tickers, ticker.Ticker)
			}
		}
		builder.addGroup(ExplorerGroup{
			ID:          "theme:" + theme.ID,
			Type:        "theme",
			Value:       theme.ID,
			Label:       theme.Name,
			Description: theme.Description,
			Tickers:     tickers,
		})
	}
}

func (builder *explorerIndexBuilder) addLayerGroups() {
	for _, chain := range builder.cat.SupplyChains {
		theme := builder.themeByID[chain.ThemeID]
		for _, layer := range chain.Layers {
			var tickers []string
			for _, exposure := range builder.cat.Exposures {
				if exposure.ThemeID == chain.ThemeID && exposure.LayerID == layer.ID {
					tickers = appendUniqueStrings(tickers, builder.tickersForExposure(exposure)...)
				}
			}
			builder.addGroup(ExplorerGroup{
				ID:          "layer:" + chain.ThemeID + ":" + layer.ID,
				Type:        "layer",
				Value:       layer.ID,
				Label:       layer.Name,
				ParentID:    chain.ThemeID,
				ParentLabel: firstNonEmpty(theme.Name, chain.ThemeID),
				Description: layer.Description,
				Tickers:     tickers,
			})
		}
	}
}

func (builder *explorerIndexBuilder) addRelationshipGroups() {
	type relationshipGroup struct {
		tickers []string
		edges   int
	}
	byType := map[string]relationshipGroup{}
	for _, relationship := range builder.cat.Relationships {
		if relationship.RelationshipType == "" {
			continue
		}
		group := byType[relationship.RelationshipType]
		group.edges++
		group.tickers = appendUniqueStrings(group.tickers, builder.relationshipEndpointTickers(relationship, "source")...)
		group.tickers = appendUniqueStrings(group.tickers, builder.relationshipEndpointTickers(relationship, "target")...)
		byType[relationship.RelationshipType] = group
	}
	for relationshipType, group := range byType {
		builder.addGroup(ExplorerGroup{
			ID:        "relationship:" + relationshipType,
			Type:      "relationship",
			Value:     relationshipType,
			Label:     humanizeIdentifier(relationshipType),
			EdgeCount: group.edges,
			Tickers:   group.tickers,
		})
	}
}

func (builder *explorerIndexBuilder) tickersForExposure(exposure taxonomy.Exposure) []string {
	switch {
	case exposure.Ticker != "" && builder.tickerByID[exposure.Ticker]:
		return []string{exposure.Ticker}
	case exposure.CompanyID != "":
		return builder.tickersByCompany[exposure.CompanyID]
	case exposure.ISIN != "":
		return builder.tickersByISIN[exposure.ISIN]
	default:
		return nil
	}
}

func (builder *explorerIndexBuilder) relationshipEndpointTickers(row taxonomy.Relationship, side string) []string {
	switch side {
	case "source":
		return builder.tickersForEndpoint(row.SourceTicker, row.SourceISIN, row.SourceCompanyID)
	case "target":
		return builder.tickersForEndpoint(row.TargetTicker, row.TargetISIN, row.TargetCompanyID)
	default:
		return nil
	}
}

func (builder *explorerIndexBuilder) tickersForEndpoint(ticker string, isin string, companyID string) []string {
	switch {
	case ticker != "" && builder.tickerByID[ticker]:
		return []string{ticker}
	case companyID != "":
		return builder.tickersByCompany[companyID]
	case isin != "":
		return builder.tickersByISIN[isin]
	default:
		return nil
	}
}

func (builder *explorerIndexBuilder) addGroup(group ExplorerGroup) {
	group.Tickers = sortedUniqueStrings(group.Tickers)
	group.Count = len(group.Tickers)
	if group.Count == 0 {
		return
	}
	builder.groupsByID[group.ID] = &group
}

func (builder *explorerIndexBuilder) groups() []ExplorerGroup {
	out := make([]ExplorerGroup, 0, len(builder.groupsByID))
	for _, group := range builder.groupsByID {
		out = append(out, *group)
	}
	sort.SliceStable(out, func(i, j int) bool {
		leftOrder := explorerTypeOrder(out[i].Type)
		rightOrder := explorerTypeOrder(out[j].Type)
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		if out[i].ParentLabel != out[j].ParentLabel {
			return out[i].ParentLabel < out[j].ParentLabel
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func explorerTypeOrder(value string) int {
	switch value {
	case "theme":
		return 10
	case "layer":
		return 20
	case "sector":
		return 30
	case "industry":
		return 40
	case "category":
		return 50
	case "flag":
		return 60
	case "relationship":
		return 70
	default:
		return 100
	}
}

func humanizeIdentifier(value string) string {
	parts := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(value, "_", " "), "-", " "))
	for index, part := range parts {
		if strings.EqualFold(part, "etf") || strings.EqualFold(part, "etp") || strings.EqualFold(part, "adr") || strings.EqualFold(part, "gdr") {
			parts[index] = strings.ToUpper(part)
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}

func appendUniqueStrings(existing []string, additions ...string) []string {
	seen := map[string]bool{}
	for _, value := range existing {
		seen[value] = true
	}
	for _, value := range additions {
		if value == "" || seen[value] {
			continue
		}
		existing = append(existing, value)
		seen[value] = true
	}
	return existing
}

func sortedUniqueStrings(values []string) []string {
	out := appendUniqueStrings(nil, values...)
	sort.Strings(out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func containsString(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}

func generatedFileFormat(name string) string {
	if strings.HasSuffix(name, ".csv") {
		return "csv"
	}
	return "json"
}

func BuildSearchIndex(cat *catalogue.Catalogue) []SearchDocument {
	var out []SearchDocument
	for _, ticker := range cat.Tickers {
		out = append(out, SearchDocument{
			ID:       ticker.Ticker,
			Type:     "ticker",
			Title:    ticker.Ticker,
			Subtitle: ticker.Name,
			Text:     joinText(ticker.Ticker, ticker.Name, ticker.ISIN, ticker.Sector, ticker.Industry, ticker.YahooSymbol),
			Tickers:  []string{ticker.Ticker},
		})
	}
	for _, company := range cat.Companies {
		out = append(out, SearchDocument{
			ID:       company.ID,
			Type:     "company",
			Title:    company.Name,
			Subtitle: company.PrimaryTicker,
			Text:     joinText(company.ID, company.Name, company.Sector, company.Industry, company.Country, company.YahooSymbol),
			Tickers:  company.TickerIDs,
		})
	}
	for _, theme := range cat.Themes {
		out = append(out, SearchDocument{
			ID:    theme.ID,
			Type:  "theme",
			Title: theme.Name,
			Text:  joinText(theme.ID, theme.Name, theme.Description),
		})
	}
	for _, note := range cat.Notes {
		out = append(out, SearchDocument{
			ID:       note.Path,
			Type:     "note",
			Title:    note.Title,
			Subtitle: note.TargetType + ":" + note.TargetID,
			Text:     joinText(note.Title, note.TargetType, note.TargetID, note.Text),
		})
	}
	return out
}

func catalogueWithContractMetadata(cat *catalogue.Catalogue) catalogue.Catalogue {
	out := *cat
	if out.DataContractVersion == 0 {
		out.DataContractVersion = catalogue.DataContractVersion
	}
	if out.SchemaVersion == 0 {
		out.SchemaVersion = catalogue.DataContractSchemaVersion
	}
	if out.GeneratedAt == "" {
		out.GeneratedAt = out.Manifest.BuiltAt
	}
	out.Relationships = sortedRelationships(out.Relationships)
	if out.ReviewSummary.ByQueue == nil {
		out.ReviewSummary = catalogue.BuildReviewSummary(out.ReviewQueues, parseManifestTime(out.Manifest.BuiltAt))
	}
	out.Manifest = manifestWithContractMetadata(out.Manifest)
	out.Manifest.GeneratedFiles = nil
	return out
}

func manifestWithContractMetadata(manifest catalogue.BuildManifest) catalogue.BuildManifest {
	if manifest.DataContractVersion == 0 {
		manifest.DataContractVersion = catalogue.DataContractVersion
	}
	if manifest.SchemaVersion == 0 {
		manifest.SchemaVersion = catalogue.DataContractSchemaVersion
	}
	if manifest.ReviewQueueCounts == nil {
		manifest.ReviewQueueCounts = map[string]int{}
	}
	if manifest.ReviewReasonCounts == nil {
		manifest.ReviewReasonCounts = map[string]int{}
	}
	return manifest
}

func parseManifestTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func sortedRelationships(rows []taxonomy.Relationship) []taxonomy.Relationship {
	out := make([]taxonomy.Relationship, 0, len(rows))
	out = append(out, rows...)
	sort.SliceStable(out, func(i, j int) bool {
		return relationshipSortKey(out[i]) < relationshipSortKey(out[j])
	})
	return out
}

func relationshipSortKey(row taxonomy.Relationship) string {
	return strings.Join([]string{
		row.RelationshipType,
		row.SourceTicker,
		row.SourceISIN,
		row.SourceCompanyID,
		row.TargetTicker,
		row.TargetISIN,
		row.TargetCompanyID,
		row.ThemeID,
		row.LayerID,
		row.Confidence,
		row.SourceURL,
		row.Rationale,
		row.LastReviewed,
	}, "\x00")
}

func generatedFileMetadataForContract(contractCat catalogue.Catalogue, outputByName map[string]generatedOutput) ([]catalogue.GeneratedFile, error) {
	files := make([]catalogue.GeneratedFile, 0, len(GeneratedSiteDataFiles))

	appBootstrapProjection := buildAppBootstrap(contractCat, nil)
	appBootstrapProjectionBytes, err := marshalCompactJSON(appBootstrapProjection)
	if err != nil {
		return nil, err
	}

	manifestProjection := contractCat.Manifest
	manifestProjection.GeneratedFiles = nil
	manifestProjectionBytes, err := marshalJSON(manifestProjection)
	if err != nil {
		return nil, err
	}

	for _, name := range GeneratedSiteDataFiles {
		switch name {
		case "app_bootstrap.json":
			files = append(files, catalogue.GeneratedFile{
				Path:          canonicalSiteDataPath(name),
				Format:        "json",
				SchemaVersion: catalogue.DataContractSchemaVersion,
				SHA256:        sha256Hex(appBootstrapProjectionBytes),
				Bytes:         int64(len(appBootstrapProjectionBytes)),
				ChecksumMode:  appBootstrapChecksumMode,
			})
		case "build_manifest.json":
			files = append(files, catalogue.GeneratedFile{
				Path:          canonicalSiteDataPath(name),
				Format:        "json",
				SchemaVersion: catalogue.DataContractSchemaVersion,
				SHA256:        sha256Hex(manifestProjectionBytes),
				Bytes:         int64(len(manifestProjectionBytes)),
				ChecksumMode:  buildManifestChecksumMode,
			})
		default:
			output, ok := outputByName[name]
			if !ok {
				return nil, fmt.Errorf("missing generated output metadata for %s", name)
			}
			files = append(files, catalogue.GeneratedFile{
				Path:          canonicalSiteDataPath(output.name),
				Format:        output.format,
				SchemaVersion: output.schemaVersion,
				SHA256:        sha256Hex(output.bytes),
				Bytes:         int64(len(output.bytes)),
			})
		}
	}
	return files, nil
}

func marshalCircularMetadataOutputs(contractCat catalogue.Catalogue, files []catalogue.GeneratedFile) ([]byte, []byte, []catalogue.GeneratedFile, error) {
	appBootstrapIndex := generatedFileIndex(files, "app_bootstrap.json")
	buildManifestIndex := generatedFileIndex(files, "build_manifest.json")
	if appBootstrapIndex < 0 || buildManifestIndex < 0 {
		return nil, nil, nil, fmt.Errorf("app bootstrap and build manifest metadata entries are required")
	}
	files = append([]catalogue.GeneratedFile(nil), files...)

	var appBootstrapBytes []byte
	var manifestBytes []byte
	for i := 0; i < 12; i++ {
		var err error
		appBootstrap := buildAppBootstrap(contractCat, files)
		appBootstrapBytes, err = marshalCompactJSON(appBootstrap)
		if err != nil {
			return nil, nil, nil, err
		}

		manifest := contractCat.Manifest
		manifest.GeneratedFiles = files
		manifestBytes, err = marshalJSON(manifest)
		if err != nil {
			return nil, nil, nil, err
		}

		changed := false
		if files[appBootstrapIndex].Bytes != int64(len(appBootstrapBytes)) {
			files[appBootstrapIndex].Bytes = int64(len(appBootstrapBytes))
			changed = true
		}
		if files[buildManifestIndex].Bytes != int64(len(manifestBytes)) {
			files[buildManifestIndex].Bytes = int64(len(manifestBytes))
			changed = true
		}
		if !changed {
			return appBootstrapBytes, manifestBytes, files, nil
		}
	}

	appBootstrap := buildAppBootstrap(contractCat, files)
	var err error
	appBootstrapBytes, err = marshalCompactJSON(appBootstrap)
	if err != nil {
		return nil, nil, nil, err
	}
	manifest := contractCat.Manifest
	manifest.GeneratedFiles = files
	manifestBytes, err = marshalJSON(manifest)
	if err != nil {
		return nil, nil, nil, err
	}
	return appBootstrapBytes, manifestBytes, files, nil
}

func generatedFileIndex(files []catalogue.GeneratedFile, name string) int {
	path := canonicalSiteDataPath(name)
	for i, file := range files {
		if file.Path == path {
			return i
		}
	}
	return -1
}

func buildAppBootstrap(cat catalogue.Catalogue, generatedFiles []catalogue.GeneratedFile) AppBootstrap {
	manifest := cat.Manifest
	manifest.GeneratedFiles = nil
	files := append([]catalogue.GeneratedFile(nil), generatedFiles...)
	return AppBootstrap{
		DataContractVersion: catalogue.DataContractVersion,
		SchemaVersion:       catalogue.DataContractSchemaVersion,
		GeneratedAt:         cat.GeneratedAt,
		Manifest:            manifest,
		Themes:              cat.Themes,
		SupplyChains:        cat.SupplyChains,
		Exposures:           cat.Exposures,
		ThemeCounts:         countTickersByTheme(cat.Tickers),
		Sectors:             summarizeGroups(cat.Sectors, groupSummaryTickerLimit),
		Industries:          summarizeGroups(cat.Industries, groupSummaryTickerLimit),
		Counts:              appBootstrapCounts(&cat),
		GeneratedFiles:      files,
	}
}

func appBootstrapCounts(cat *catalogue.Catalogue) AppBootstrapCounts {
	if cat == nil {
		return AppBootstrapCounts{}
	}
	return AppBootstrapCounts{
		TickerCount:       len(cat.Tickers),
		CompanyCount:      len(cat.Companies),
		SecurityCount:     len(cat.Securities),
		ListingCount:      len(cat.Listings),
		ThemeCount:        len(cat.Themes),
		SupplyChainCount:  len(cat.SupplyChains),
		ExposureCount:     len(cat.Exposures),
		RelationshipCount: len(cat.Relationships),
		UnclassifiedCount: len(cat.Unclassified),
	}
}

func summarizeGroups(groups []catalogue.GroupCount, tickerLimit int) []GroupSummary {
	out := make([]GroupSummary, 0, len(groups))
	for _, group := range groups {
		tickers := group.Tickers
		if tickerLimit >= 0 && len(tickers) > tickerLimit {
			tickers = tickers[:tickerLimit]
		}
		out = append(out, GroupSummary{
			ID:      group.ID,
			Name:    group.Name,
			Count:   group.Count,
			Tickers: append([]string(nil), tickers...),
		})
	}
	return out
}

func countTickersByTheme(tickers []catalogue.Ticker) map[string]int {
	out := map[string]int{}
	for _, ticker := range tickers {
		for _, themeID := range ticker.ThemeIDs {
			out[themeID]++
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func generatedFileMetadata(outputs []generatedOutput) []catalogue.GeneratedFile {
	files := make([]catalogue.GeneratedFile, 0, len(outputs))
	for _, output := range outputs {
		files = append(files, catalogue.GeneratedFile{
			Path:          canonicalSiteDataPath(output.name),
			Format:        output.format,
			SchemaVersion: output.schemaVersion,
			SHA256:        sha256Hex(output.bytes),
			Bytes:         int64(len(output.bytes)),
		})
	}
	return files
}

func canonicalSiteDataPath(name string) string {
	return filepath.ToSlash(filepath.Join("site", "data", name))
}

func marshalManifestWithFileMetadata(manifest catalogue.BuildManifest, generatedFiles []catalogue.GeneratedFile) ([]byte, []catalogue.GeneratedFile, error) {
	files := append([]catalogue.GeneratedFile(nil), generatedFiles...)
	for i := 0; i < 8; i++ {
		manifest.GeneratedFiles = files
		b, err := marshalJSON(manifest)
		if err != nil {
			return nil, nil, err
		}
		last := len(files) - 1
		if files[last].Path != canonicalSiteDataPath("build_manifest.json") {
			return nil, nil, fmt.Errorf("build manifest metadata entry must be last")
		}
		if files[last].Bytes == int64(len(b)) {
			return b, files, nil
		}
		files[last].Bytes = int64(len(b))
	}
	manifest.GeneratedFiles = files
	b, err := marshalJSON(manifest)
	if err != nil {
		return nil, nil, err
	}
	return b, files, nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func marshalJSON(value any) ([]byte, error) {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal JSON: %w", err)
	}
	b = append(b, '\n')
	return b, nil
}

func marshalCompactJSON(value any) ([]byte, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal JSON: %w", err)
	}
	b = append(b, '\n')
	return b, nil
}

func marshalTickersCSV(tickers []catalogue.Ticker) ([]byte, error) {
	var records [][]string
	for _, ticker := range tickers {
		records = append(records, []string{
			ticker.Ticker,
			ticker.Name,
			ticker.ISIN,
			ticker.CompanyID,
			ticker.SecurityID,
			ticker.Type,
			ticker.InstrumentCategory,
			joinCSVList(ticker.StructureFlags),
			ticker.CurrencyCode,
			ticker.ExchangeName,
			ticker.YahooSymbol,
			ticker.Sector,
			ticker.Industry,
			ticker.Country,
			strconv.FormatInt(ticker.MarketCap, 10),
			ticker.Directionality,
			ticker.IdentityConfidence,
			joinCSVList(ticker.IdentityReasons),
			joinCSVList(ticker.ThemeIDs),
			joinCSVList(ticker.LayerIDs),
			strconv.FormatBool(ticker.Unclassified),
		})
	}
	return marshalCSV(TickersCSVHeader, records)
}

func marshalSecuritiesCSV(rows []catalogue.Security) ([]byte, error) {
	records := make([][]string, 0, len(rows))
	for _, row := range rows {
		records = append(records, []string{
			row.ID,
			row.ISIN,
			row.Name,
			row.Type,
			row.InstrumentCategory,
			joinCSVList(row.StructureFlags),
			row.CompanyID,
			joinCSVList(row.ListingIDs),
			joinCSVList(row.TickerIDs),
			joinCSVList(row.CurrencySet),
			row.IdentityConfidence,
			joinCSVList(row.IdentityReasons),
		})
	}
	return marshalCSV(SecuritiesCSVHeader, records)
}

func marshalListingsCSV(rows []catalogue.Listing) ([]byte, error) {
	records := make([][]string, 0, len(rows))
	for _, row := range rows {
		records = append(records, []string{row.ID, row.Ticker, row.SecurityID, row.CompanyID, row.ExchangeCode, row.ExchangeName, row.CurrencyCode})
	}
	return marshalCSV(ListingsCSVHeader, records)
}

func marshalUnclassifiedCSV(rows []catalogue.UnclassifiedRow) ([]byte, error) {
	records := make([][]string, 0, len(rows))
	for _, row := range rows {
		records = append(records, []string{row.Ticker, row.CompanyID, row.Name, row.ISIN, row.Reason, joinCSVList(row.ReasonCodes)})
	}
	return marshalCSV(UnclassifiedCSVHeader, records)
}

func marshalIdentityIssuesCSV(rows []catalogue.IdentityIssue) ([]byte, error) {
	records := make([][]string, 0, len(rows))
	for _, row := range rows {
		records = append(records, []string{row.IssueCode, row.Ticker, row.ISIN, row.SecurityID, row.CompanyID, row.Name, row.Reason, row.SuggestedAction})
	}
	return marshalCSV(IdentityIssuesCSVHeader, records)
}

func marshalEnrichmentFailuresCSV(rows []catalogue.EnrichmentFailure) ([]byte, error) {
	records := make([][]string, 0, len(rows))
	for _, row := range rows {
		records = append(records, []string{row.Ticker, row.ISIN, row.Name, row.Provider, row.AttemptedSymbols, row.Status, row.Error, row.NextAction})
	}
	return marshalCSV(EnrichmentFailuresCSVHeader, records)
}

func marshalReviewIssuesCSV(rows []catalogue.ReviewQueueRow, queue string) ([]byte, error) {
	records := make([][]string, 0, len(rows))
	for _, row := range rows {
		if row.Queue != queue {
			continue
		}
		records = append(records, []string{
			row.Queue,
			row.ReasonCode,
			row.Severity,
			row.Ticker,
			row.ISIN,
			row.CompanyID,
			row.SecurityID,
			row.Name,
			row.Sector,
			row.Industry,
			joinCSVList(row.ThemeIDs),
			joinCSVList(row.LayerIDs),
			row.SourceFile,
			strconv.Itoa(row.SourceRow),
			row.SuggestedAction,
			row.SuggestedManualFile,
			row.SuggestedCSVRow,
			row.LastReviewed,
			row.LastRefreshed,
		})
	}
	return marshalCSV(ReviewIssuesCSVHeader, records)
}

func marshalSuggestedManualCSV(rows []catalogue.ReviewQueueRow, manualFile string, header []string) ([]byte, error) {
	records := [][]string{}
	seen := map[string]bool{}
	for _, row := range rows {
		if row.SuggestedManualFile != manualFile || row.SuggestedCSVRow == "" {
			continue
		}
		record, err := parseCSVRecord(row.SuggestedCSVRow)
		if err != nil {
			return nil, err
		}
		if len(record) != len(header) {
			return nil, fmt.Errorf("suggested row for %s has %d fields, want %d", manualFile, len(record), len(header))
		}
		key := strings.Join(record, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		records = append(records, record)
	}
	return marshalCSV(header, records)
}

func parseCSVRecord(value string) ([]string, error) {
	reader := csv.NewReader(strings.NewReader(value + "\n"))
	record, err := reader.Read()
	if err != nil {
		return nil, err
	}
	return record, nil
}

func marshalCSV(header []string, records [][]string) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, record := range records {
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func joinCSVList(values []string) string {
	out := ""
	for i, value := range values {
		if i > 0 {
			out += ";"
		}
		out += value
	}
	return out
}

func joinText(values ...string) string {
	out := ""
	for _, value := range values {
		if value == "" {
			continue
		}
		if out != "" {
			out += " "
		}
		out += value
	}
	return out
}
