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

	"statos/internal/catalogue"
	"statos/internal/taxonomy"
)

var TickersCSVHeader = []string{"ticker", "name", "isin", "company_id", "security_id", "type", "instrument_category", "structure_flags", "currency", "exchange", "yahoo_symbol", "sector", "industry", "country", "market_cap", "directionality", "identity_confidence", "identity_reasons", "themes", "layers", "unclassified"}
var SecuritiesCSVHeader = []string{"security_id", "isin", "name", "type", "instrument_category", "structure_flags", "company_id", "listing_ids", "ticker_ids", "currency_set", "identity_confidence", "identity_reasons"}
var ListingsCSVHeader = []string{"listing_id", "ticker", "security_id", "company_id", "exchange_code", "exchange_name", "currency_code"}
var UnclassifiedCSVHeader = []string{"ticker", "company_id", "name", "isin", "reason"}
var IdentityIssuesCSVHeader = []string{"issue_code", "ticker", "isin", "security_id", "company_id", "name", "reason", "suggested_action"}
var EnrichmentFailuresCSVHeader = []string{"ticker", "isin", "name", "provider", "attempted_symbols", "status", "error", "next_action"}

var GeneratedSiteDataFiles = []string{
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
	"tickers.csv",
	"securities.csv",
	"listings.csv",
	"unclassified.csv",
	"identity_issues.csv",
	"enrichment_failures.csv",
	"build_manifest.json",
}

const buildManifestChecksumMode = "projection_excludes_generatedFiles"

type generatedOutput struct {
	name          string
	format        string
	schemaVersion int
	bytes         []byte
}

type SearchDocument struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle,omitempty"`
	Text     string   `json:"text"`
	Tickers  []string `json:"tickers,omitempty"`
}

func CSVHeaders() map[string][]string {
	headers := map[string][]string{
		"tickers.csv":             TickersCSVHeader,
		"securities.csv":          SecuritiesCSVHeader,
		"listings.csv":            ListingsCSVHeader,
		"unclassified.csv":        UnclassifiedCSVHeader,
		"identity_issues.csv":     IdentityIssuesCSVHeader,
		"enrichment_failures.csv": EnrichmentFailuresCSVHeader,
	}
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
		{"catalogue.json", contractCat},
		{"companies.json", contractCat.Companies},
		{"sectors.json", contractCat.Sectors},
		{"industries.json", contractCat.Industries},
		{"themes.json", contractCat.Themes},
		{"supply_chains.json", contractCat.SupplyChains},
		{"search_index.json", BuildSearchIndex(&contractCat)},
		{"securities.json", contractCat.Securities},
		{"listings.json", contractCat.Listings},
		{"relationships.json", contractCat.Relationships},
	}
	outputs := make([]generatedOutput, 0, len(GeneratedSiteDataFiles))
	for _, def := range defs {
		b, err := marshalJSON(def.value)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, generatedOutput{name: def.name, format: "json", schemaVersion: catalogue.DataContractSchemaVersion, bytes: b})
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
		{name: "identity_issues.csv"},
		{name: "enrichment_failures.csv"},
	}
	csvDefs[0].data, csvDefs[0].err = marshalTickersCSV(contractCat.Tickers)
	csvDefs[1].data, csvDefs[1].err = marshalSecuritiesCSV(contractCat.Securities)
	csvDefs[2].data, csvDefs[2].err = marshalListingsCSV(contractCat.Listings)
	csvDefs[3].data, csvDefs[3].err = marshalUnclassifiedCSV(contractCat.Unclassified)
	csvDefs[4].data, csvDefs[4].err = marshalIdentityIssuesCSV(contractCat.IdentityIssues)
	csvDefs[5].data, csvDefs[5].err = marshalEnrichmentFailuresCSV(contractCat.EnrichmentFailures)
	for _, def := range csvDefs {
		if def.err != nil {
			return nil, def.err
		}
		outputs = append(outputs, generatedOutput{name: def.name, format: "csv", schemaVersion: catalogue.DataContractSchemaVersion, bytes: def.data})
	}

	manifestProjection := contractCat.Manifest
	manifestProjection.GeneratedFiles = nil
	projectionBytes, err := marshalJSON(manifestProjection)
	if err != nil {
		return nil, err
	}
	generatedFiles := generatedFileMetadata(outputs)
	generatedFiles = append(generatedFiles, catalogue.GeneratedFile{
		Path:          canonicalSiteDataPath("build_manifest.json"),
		Format:        "json",
		SchemaVersion: catalogue.DataContractSchemaVersion,
		SHA256:        sha256Hex(projectionBytes),
		Bytes:         int64(len(projectionBytes)),
		ChecksumMode:  buildManifestChecksumMode,
	})
	manifestBytes, _, err := marshalManifestWithFileMetadata(manifestProjection, generatedFiles)
	if err != nil {
		return nil, err
	}
	outputs = append(outputs, generatedOutput{name: "build_manifest.json", format: "json", schemaVersion: catalogue.DataContractSchemaVersion, bytes: manifestBytes})

	return outputs, nil
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
	return manifest
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
		records = append(records, []string{row.Ticker, row.CompanyID, row.Name, row.ISIN, row.Reason})
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
