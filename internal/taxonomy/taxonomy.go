package taxonomy

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ExposureCSVHeader = []string{"theme_id", "layer_id", "ticker", "isin", "company_id", "exposure_score", "confidence", "source_url", "rationale", "last_reviewed"}
var CompanyOverridesCSVHeader = []string{"company_id", "name", "sector", "industry", "country", "source_url", "last_reviewed"}
var TickerOverridesCSVHeader = []string{"ticker", "company_id", "name", "sector", "industry", "country", "yahoo_symbol", "market_cap", "exchange", "currency", "source_url", "last_reviewed"}
var ClassificationOverridesCSVHeader = []string{"target_type", "ticker", "isin", "company_id", "sector", "industry", "country", "source_url", "last_reviewed"}
var IdentityOverridesCSVHeader = []string{"target_type", "ticker", "isin", "security_id", "company_id", "override_security_id", "override_company_id", "category", "flags", "confidence", "reason", "source_url", "last_reviewed"}
var RelationshipsCSVHeader = []string{"relationship_type", "source_ticker", "source_isin", "source_company_id", "target_ticker", "target_isin", "target_company_id", "theme_id", "layer_id", "confidence", "source_url", "rationale", "last_reviewed"}

type ManualData struct {
	Themes                  []Theme
	SupplyChains            []SupplyChain
	Exposures               []Exposure
	CompanyOverrides        map[string]CompanyOverride
	TickerOverrides         map[string]TickerOverride
	ClassificationOverrides []ClassificationOverride
	IdentityOverrides       []IdentityOverride
	Relationships           []Relationship
	Notes                   []Note
}

type Theme struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
	SourcePath  string `json:"-"`
	SourceLine  int    `json:"-"`
}

type SupplyChain struct {
	ThemeID     string             `json:"themeId"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Layers      []SupplyChainLayer `json:"layers"`
	SourcePath  string             `json:"-"`
	SourceLine  int                `json:"-"`
}

type SupplyChainLayer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Order       int    `json:"order"`
	SourcePath  string `json:"-"`
	SourceLine  int    `json:"-"`
	OrderSet    bool   `json:"-"`
}

type Exposure struct {
	ThemeID       string  `json:"themeId"`
	LayerID       string  `json:"layerId"`
	Ticker        string  `json:"ticker,omitempty"`
	ISIN          string  `json:"isin,omitempty"`
	CompanyID     string  `json:"companyId,omitempty"`
	ExposureScore float64 `json:"exposureScore"`
	Confidence    string  `json:"confidence"`
	SourceURL     string  `json:"sourceUrl,omitempty"`
	Rationale     string  `json:"rationale,omitempty"`
	LastReviewed  string  `json:"lastReviewed,omitempty"`
	SourcePath    string  `json:"-"`
	SourceRow     int     `json:"-"`
}

type CompanyOverride struct {
	CompanyID    string `json:"companyId"`
	Name         string `json:"name,omitempty"`
	Sector       string `json:"sector,omitempty"`
	Industry     string `json:"industry,omitempty"`
	Country      string `json:"country,omitempty"`
	SourceURL    string `json:"sourceUrl,omitempty"`
	LastReviewed string `json:"lastReviewed,omitempty"`
	SourcePath   string `json:"-"`
	SourceRow    int    `json:"-"`
}

type TickerOverride struct {
	Ticker       string `json:"ticker"`
	CompanyID    string `json:"companyId,omitempty"`
	Name         string `json:"name,omitempty"`
	Sector       string `json:"sector,omitempty"`
	Industry     string `json:"industry,omitempty"`
	Country      string `json:"country,omitempty"`
	YahooSymbol  string `json:"yahooSymbol,omitempty"`
	MarketCap    int64  `json:"marketCap,omitempty"`
	Exchange     string `json:"exchange,omitempty"`
	Currency     string `json:"currency,omitempty"`
	SourceURL    string `json:"sourceUrl,omitempty"`
	LastReviewed string `json:"lastReviewed,omitempty"`
	SourcePath   string `json:"-"`
	SourceRow    int    `json:"-"`
}

type ClassificationOverride struct {
	TargetType   string `json:"targetType"`
	Ticker       string `json:"ticker,omitempty"`
	ISIN         string `json:"isin,omitempty"`
	CompanyID    string `json:"companyId,omitempty"`
	Sector       string `json:"sector,omitempty"`
	Industry     string `json:"industry,omitempty"`
	Country      string `json:"country,omitempty"`
	SourceURL    string `json:"sourceUrl,omitempty"`
	LastReviewed string `json:"lastReviewed,omitempty"`
	SourcePath   string `json:"-"`
	SourceRow    int    `json:"-"`
}

type IdentityOverride struct {
	TargetType         string   `json:"targetType"`
	Ticker             string   `json:"ticker,omitempty"`
	ISIN               string   `json:"isin,omitempty"`
	SecurityID         string   `json:"securityId,omitempty"`
	CompanyID          string   `json:"companyId,omitempty"`
	OverrideSecurityID string   `json:"overrideSecurityId,omitempty"`
	OverrideCompanyID  string   `json:"overrideCompanyId,omitempty"`
	Category           string   `json:"category,omitempty"`
	Flags              []string `json:"flags,omitempty"`
	Confidence         string   `json:"confidence,omitempty"`
	Reason             string   `json:"reason,omitempty"`
	SourceURL          string   `json:"sourceUrl,omitempty"`
	LastReviewed       string   `json:"lastReviewed,omitempty"`
	SourcePath         string   `json:"-"`
	SourceRow          int      `json:"-"`
}

type Relationship struct {
	RelationshipType string `json:"relationshipType"`
	SourceTicker     string `json:"sourceTicker,omitempty"`
	SourceISIN       string `json:"sourceIsin,omitempty"`
	SourceCompanyID  string `json:"sourceCompanyId,omitempty"`
	TargetTicker     string `json:"targetTicker,omitempty"`
	TargetISIN       string `json:"targetIsin,omitempty"`
	TargetCompanyID  string `json:"targetCompanyId,omitempty"`
	ThemeID          string `json:"themeId,omitempty"`
	LayerID          string `json:"layerId,omitempty"`
	Confidence       string `json:"confidence"`
	SourceURL        string `json:"sourceUrl"`
	Rationale        string `json:"rationale,omitempty"`
	LastReviewed     string `json:"lastReviewed"`
	SourcePath       string `json:"-"`
	SourceRow        int    `json:"-"`
}

type Note struct {
	TargetType   string   `json:"targetType"`
	TargetID     string   `json:"targetId"`
	Title        string   `json:"title"`
	Tags         []string `json:"tags,omitempty"`
	LastReviewed string   `json:"lastReviewed,omitempty"`
	Path         string   `json:"path"`
	Text         string   `json:"text"`
}

func Load(dir string) (ManualData, error) {
	data := ManualData{
		CompanyOverrides: map[string]CompanyOverride{},
		TickerOverrides:  map[string]TickerOverride{},
	}
	var err error
	if data.Themes, err = LoadThemes(filepath.Join(dir, "themes.yml")); err != nil {
		return ManualData{}, err
	}
	if data.SupplyChains, err = LoadSupplyChains(filepath.Join(dir, "supply_chains.yml")); err != nil {
		return ManualData{}, err
	}
	if data.CompanyOverrides, err = LoadCompanyOverrides(filepath.Join(dir, "company_overrides.csv")); err != nil {
		return ManualData{}, err
	}
	if data.TickerOverrides, err = LoadTickerOverrides(filepath.Join(dir, "ticker_overrides.csv")); err != nil {
		return ManualData{}, err
	}
	if data.ClassificationOverrides, err = LoadClassificationOverrides(filepath.Join(dir, "classification_overrides.csv")); err != nil {
		return ManualData{}, err
	}
	if data.IdentityOverrides, err = LoadIdentityOverrides(filepath.Join(dir, "identity_overrides.csv")); err != nil {
		return ManualData{}, err
	}
	if data.Exposures, err = LoadExposures(filepath.Join(dir, "exposures.csv")); err != nil {
		return ManualData{}, err
	}
	if data.Relationships, err = LoadRelationships(filepath.Join(dir, "relationships.csv")); err != nil {
		return ManualData{}, err
	}
	if data.Notes, err = LoadNotes(filepath.Join(dir, "notes")); err != nil {
		return ManualData{}, err
	}
	if err := Validate(data); err != nil {
		return ManualData{}, err
	}
	return data, nil
}

func LoadThemes(path string) ([]Theme, error) {
	lines, err := readYAMLLines(path)
	if err != nil {
		return nil, err
	}
	var out []Theme
	var current *Theme
	seenRoot := false
	seenIDs := map[string]int{}
	for i, line := range lines {
		lineNo := i + 1
		if strings.Contains(line, "\t") {
			return nil, fmt.Errorf("%s:%d uses tabs for indentation; use spaces", path, lineNo)
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := countIndent(line)
		switch {
		case indent == 0 && trimmed == "themes:":
			if seenRoot {
				return nil, fmt.Errorf("%s:%d has duplicate themes root", path, lineNo)
			}
			seenRoot = true
		case indent == 0:
			return nil, fmt.Errorf("%s:%d expected themes root, got %q", path, lineNo, trimmed)
		case !seenRoot:
			return nil, fmt.Errorf("%s:%d has theme content before themes root", path, lineNo)
		case indent == 2 && strings.HasPrefix(trimmed, "- "):
			if current != nil {
				out = append(out, *current)
			}
			current = &Theme{SourcePath: path, SourceLine: lineNo}
			if err := setThemeField(current, strings.TrimPrefix(trimmed, "- "), path, lineNo); err != nil {
				return nil, err
			}
		case indent == 4 && current != nil:
			if err := setThemeField(current, trimmed, path, lineNo); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%s:%d malformed theme indentation or item", path, lineNo)
		}
	}
	if current != nil {
		out = append(out, *current)
	}
	if !seenRoot {
		return nil, fmt.Errorf("%s missing required themes root", path)
	}
	for _, theme := range out {
		if theme.ID == "" {
			return nil, fmt.Errorf("%s:%d theme has empty id", path, theme.SourceLine)
		}
		if !validSlug(theme.ID) {
			return nil, fmt.Errorf("%s:%d theme id %q is not a slug", path, theme.SourceLine, theme.ID)
		}
		if existingLine, ok := seenIDs[theme.ID]; ok {
			return nil, fmt.Errorf("%s:%d duplicate theme id %q first defined at line %d", path, theme.SourceLine, theme.ID, existingLine)
		}
		seenIDs[theme.ID] = theme.SourceLine
		if theme.Name == "" {
			return nil, fmt.Errorf("%s:%d theme %q has empty name", path, theme.SourceLine, theme.ID)
		}
		if theme.Color != "" && !validHexColor(theme.Color) {
			return nil, fmt.Errorf("%s:%d theme %q has invalid color %q; expected #RRGGBB", path, theme.SourceLine, theme.ID, theme.Color)
		}
	}
	return out, nil
}

func LoadSupplyChains(path string) ([]SupplyChain, error) {
	lines, err := readYAMLLines(path)
	if err != nil {
		return nil, err
	}
	var out []SupplyChain
	var chain *SupplyChain
	var layer *SupplyChainLayer
	inLayers := false
	seenRoot := false
	layerIDsByTheme := map[string]map[string]int{}

	flushLayer := func() {
		if chain != nil && layer != nil {
			chain.Layers = append(chain.Layers, *layer)
			layer = nil
		}
	}
	flushChain := func() {
		flushLayer()
		if chain != nil {
			sort.SliceStable(chain.Layers, func(i, j int) bool {
				return chain.Layers[i].Order < chain.Layers[j].Order
			})
			out = append(out, *chain)
			chain = nil
		}
	}

	for i, line := range lines {
		lineNo := i + 1
		if strings.Contains(line, "\t") {
			return nil, fmt.Errorf("%s:%d uses tabs for indentation; use spaces", path, lineNo)
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := countIndent(line)
		switch {
		case indent == 0 && trimmed == "supply_chains:":
			if seenRoot {
				return nil, fmt.Errorf("%s:%d has duplicate supply_chains root", path, lineNo)
			}
			seenRoot = true
		case indent == 0:
			return nil, fmt.Errorf("%s:%d expected supply_chains root, got %q", path, lineNo, trimmed)
		case !seenRoot:
			return nil, fmt.Errorf("%s:%d has supply-chain content before supply_chains root", path, lineNo)
		case indent == 2 && strings.HasPrefix(trimmed, "- "):
			flushChain()
			chain = &SupplyChain{SourcePath: path, SourceLine: lineNo}
			inLayers = false
			if err := setSupplyChainField(chain, strings.TrimPrefix(trimmed, "- "), path, lineNo); err != nil {
				return nil, err
			}
		case indent == 4 && trimmed == "layers:":
			if chain == nil {
				return nil, fmt.Errorf("%s:%d layers block appears before a supply chain item", path, lineNo)
			}
			inLayers = true
		case indent == 4 && chain != nil && !inLayers:
			if err := setSupplyChainField(chain, trimmed, path, lineNo); err != nil {
				return nil, err
			}
		case indent == 6 && strings.HasPrefix(trimmed, "- "):
			if chain == nil || !inLayers {
				return nil, fmt.Errorf("%s:%d layer item appears outside a layers block", path, lineNo)
			}
			flushLayer()
			layer = &SupplyChainLayer{SourcePath: path, SourceLine: lineNo}
			if err := setLayerField(layer, strings.TrimPrefix(trimmed, "- "), path, lineNo); err != nil {
				return nil, err
			}
		case indent == 8 && layer != nil:
			if err := setLayerField(layer, trimmed, path, lineNo); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%s:%d malformed supply-chain indentation or item", path, lineNo)
		}
	}
	flushChain()
	if !seenRoot {
		return nil, fmt.Errorf("%s missing required supply_chains root", path)
	}
	for _, chain := range out {
		if chain.ThemeID == "" {
			return nil, fmt.Errorf("%s:%d supply chain has empty theme_id", path, chain.SourceLine)
		}
		if chain.Name == "" {
			return nil, fmt.Errorf("%s:%d supply chain for theme %q has empty name", path, chain.SourceLine, chain.ThemeID)
		}
		if len(chain.Layers) == 0 {
			return nil, fmt.Errorf("%s:%d supply chain for theme %q has no layers", path, chain.SourceLine, chain.ThemeID)
		}
		if layerIDsByTheme[chain.ThemeID] == nil {
			layerIDsByTheme[chain.ThemeID] = map[string]int{}
		}
		orders := map[int]int{}
		for _, layer := range chain.Layers {
			if layer.ID == "" {
				return nil, fmt.Errorf("%s:%d layer has empty id", path, layer.SourceLine)
			}
			if !validSlug(layer.ID) {
				return nil, fmt.Errorf("%s:%d layer id %q is not a slug", path, layer.SourceLine, layer.ID)
			}
			if existingLine, ok := layerIDsByTheme[chain.ThemeID][layer.ID]; ok {
				return nil, fmt.Errorf("%s:%d duplicate layer id %q for theme %q first defined at line %d", path, layer.SourceLine, layer.ID, chain.ThemeID, existingLine)
			}
			layerIDsByTheme[chain.ThemeID][layer.ID] = layer.SourceLine
			if layer.Name == "" {
				return nil, fmt.Errorf("%s:%d layer %q has empty name", path, layer.SourceLine, layer.ID)
			}
			if !layer.OrderSet {
				return nil, fmt.Errorf("%s:%d layer %q missing order", path, layer.SourceLine, layer.ID)
			}
			if existingLine, ok := orders[layer.Order]; ok {
				return nil, fmt.Errorf("%s:%d duplicate layer order %d for chain %q first defined at line %d", path, layer.SourceLine, layer.Order, chain.Name, existingLine)
			}
			orders[layer.Order] = layer.SourceLine
		}
	}
	return out, nil
}

func LoadCompanyOverrides(path string) (map[string]CompanyOverride, error) {
	rows, err := readManualCSV(path, CompanyOverridesCSVHeader, false)
	if err != nil {
		return nil, err
	}
	out := map[string]CompanyOverride{}
	for _, row := range rows {
		if blankRow(row.Values) {
			continue
		}
		id := row.Values["company_id"]
		if id == "" {
			return nil, fmt.Errorf("%s row %d requires company_id", path, row.Number)
		}
		if err := validateOptionalURL(path, row.Number, "source_url", row.Values["source_url"]); err != nil {
			return nil, err
		}
		if err := validateOptionalReviewDate(path, row.Number, "last_reviewed", row.Values["last_reviewed"]); err != nil {
			return nil, err
		}
		out[id] = CompanyOverride{
			CompanyID:    id,
			Name:         row.Values["name"],
			Sector:       row.Values["sector"],
			Industry:     row.Values["industry"],
			Country:      row.Values["country"],
			SourceURL:    row.Values["source_url"],
			LastReviewed: row.Values["last_reviewed"],
			SourcePath:   path,
			SourceRow:    row.Number,
		}
	}
	return out, nil
}

func LoadTickerOverrides(path string) (map[string]TickerOverride, error) {
	rows, err := readManualCSV(path, TickerOverridesCSVHeader, false)
	if err != nil {
		return nil, err
	}
	out := map[string]TickerOverride{}
	for _, row := range rows {
		if blankRow(row.Values) {
			continue
		}
		ticker := row.Values["ticker"]
		if ticker == "" {
			return nil, fmt.Errorf("%s row %d requires ticker", path, row.Number)
		}
		marketCap, err := parseOptionalInt(row.Values["market_cap"], path, row.Number, "market_cap")
		if err != nil {
			return nil, err
		}
		if err := validateOptionalURL(path, row.Number, "source_url", row.Values["source_url"]); err != nil {
			return nil, err
		}
		if err := validateOptionalReviewDate(path, row.Number, "last_reviewed", row.Values["last_reviewed"]); err != nil {
			return nil, err
		}
		out[ticker] = TickerOverride{
			Ticker:       ticker,
			CompanyID:    row.Values["company_id"],
			Name:         row.Values["name"],
			Sector:       row.Values["sector"],
			Industry:     row.Values["industry"],
			Country:      row.Values["country"],
			YahooSymbol:  row.Values["yahoo_symbol"],
			MarketCap:    marketCap,
			Exchange:     row.Values["exchange"],
			Currency:     row.Values["currency"],
			SourceURL:    row.Values["source_url"],
			LastReviewed: row.Values["last_reviewed"],
			SourcePath:   path,
			SourceRow:    row.Number,
		}
	}
	return out, nil
}

func LoadClassificationOverrides(path string) ([]ClassificationOverride, error) {
	rows, err := readManualCSV(path, ClassificationOverridesCSVHeader, true)
	if err != nil {
		return nil, err
	}
	var out []ClassificationOverride
	seen := map[string]ClassificationOverride{}
	for _, row := range rows {
		if blankRow(row.Values) {
			continue
		}
		override := ClassificationOverride{
			TargetType:   row.Values["target_type"],
			Ticker:       row.Values["ticker"],
			ISIN:         row.Values["isin"],
			CompanyID:    row.Values["company_id"],
			Sector:       row.Values["sector"],
			Industry:     row.Values["industry"],
			Country:      row.Values["country"],
			SourceURL:    row.Values["source_url"],
			LastReviewed: row.Values["last_reviewed"],
			SourcePath:   path,
			SourceRow:    row.Number,
		}
		if err := validateClassificationOverride(override); err != nil {
			return nil, err
		}
		key := classificationOverrideKey(override)
		if existing, ok := seen[key]; ok && conflictingClassificationOverride(existing, override) {
			return nil, fmt.Errorf("%s row %d conflicts with another classification override for %s", path, row.Number, key)
		}
		seen[key] = override
		out = append(out, override)
	}
	return out, nil
}

func LoadIdentityOverrides(path string) ([]IdentityOverride, error) {
	rows, err := readManualCSV(path, IdentityOverridesCSVHeader, true)
	if err != nil {
		return nil, err
	}
	var out []IdentityOverride
	for _, row := range rows {
		if blankRow(row.Values) {
			continue
		}
		override := IdentityOverride{
			TargetType:         row.Values["target_type"],
			Ticker:             row.Values["ticker"],
			ISIN:               row.Values["isin"],
			SecurityID:         row.Values["security_id"],
			CompanyID:          row.Values["company_id"],
			OverrideSecurityID: row.Values["override_security_id"],
			OverrideCompanyID:  row.Values["override_company_id"],
			Category:           row.Values["category"],
			Flags:              splitSemicolonList(row.Values["flags"]),
			Confidence:         row.Values["confidence"],
			Reason:             row.Values["reason"],
			SourceURL:          row.Values["source_url"],
			LastReviewed:       row.Values["last_reviewed"],
			SourcePath:         path,
			SourceRow:          row.Number,
		}
		if err := validateOptionalURL(path, row.Number, "source_url", override.SourceURL); err != nil {
			return nil, err
		}
		if err := validateOptionalReviewDate(path, row.Number, "last_reviewed", override.LastReviewed); err != nil {
			return nil, err
		}
		out = append(out, override)
	}
	return out, nil
}

func LoadExposures(path string) ([]Exposure, error) {
	rows, err := readManualCSV(path, ExposureCSVHeader, false)
	if err != nil {
		return nil, err
	}
	var out []Exposure
	for _, row := range rows {
		if blankRow(row.Values) {
			continue
		}
		score, err := parseExposureScore(path, row.Number, row.Values["exposure_score"])
		if err != nil {
			return nil, err
		}
		exposure := Exposure{
			ThemeID:       row.Values["theme_id"],
			LayerID:       row.Values["layer_id"],
			Ticker:        row.Values["ticker"],
			ISIN:          row.Values["isin"],
			CompanyID:     row.Values["company_id"],
			ExposureScore: score,
			Confidence:    row.Values["confidence"],
			SourceURL:     row.Values["source_url"],
			Rationale:     row.Values["rationale"],
			LastReviewed:  row.Values["last_reviewed"],
			SourcePath:    path,
			SourceRow:     row.Number,
		}
		if err := validateExposureRow(exposure); err != nil {
			return nil, err
		}
		out = append(out, exposure)
	}
	return out, nil
}

func LoadRelationships(path string) ([]Relationship, error) {
	rows, err := readManualCSV(path, RelationshipsCSVHeader, true)
	if err != nil {
		return nil, err
	}
	var out []Relationship
	for _, row := range rows {
		if blankRow(row.Values) {
			continue
		}
		relationship := Relationship{
			RelationshipType: row.Values["relationship_type"],
			SourceTicker:     row.Values["source_ticker"],
			SourceISIN:       row.Values["source_isin"],
			SourceCompanyID:  row.Values["source_company_id"],
			TargetTicker:     row.Values["target_ticker"],
			TargetISIN:       row.Values["target_isin"],
			TargetCompanyID:  row.Values["target_company_id"],
			ThemeID:          row.Values["theme_id"],
			LayerID:          row.Values["layer_id"],
			Confidence:       row.Values["confidence"],
			SourceURL:        row.Values["source_url"],
			Rationale:        row.Values["rationale"],
			LastReviewed:     row.Values["last_reviewed"],
			SourcePath:       path,
			SourceRow:        row.Number,
		}
		if err := validateRelationshipRow(relationship); err != nil {
			return nil, err
		}
		out = append(out, relationship)
	}
	return out, nil
}

func blankRow(row map[string]string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func LoadNotes(dir string) ([]Note, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Note
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		note, err := loadNote(path)
		if err != nil {
			return nil, err
		}
		out = append(out, note)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func loadNote(path string) (Note, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Note{}, err
	}
	text := string(b)
	note := Note{Path: filepath.ToSlash(path)}
	if strings.HasPrefix(text, "---\n") {
		rest := strings.TrimPrefix(text, "---\n")
		idx := strings.Index(rest, "\n---\n")
		if idx < 0 {
			return Note{}, fmt.Errorf("%s: missing closing frontmatter delimiter", path)
		}
		meta := rest[:idx]
		text = rest[idx+5:]
		for lineIndex, line := range strings.Split(meta, "\n") {
			lineNo := lineIndex + 2
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			key, value, ok := splitField(trimmed)
			if !ok {
				return Note{}, fmt.Errorf("%s:%d malformed note frontmatter line", path, lineNo)
			}
			switch key {
			case "target_type":
				note.TargetType = value
			case "target_id":
				note.TargetID = value
			case "title":
				note.Title = value
			case "tags":
				note.Tags = sortedUnique(splitList(value))
			case "last_reviewed":
				if err := validateOptionalReviewDate(path, lineNo, "last_reviewed", value); err != nil {
					return Note{}, err
				}
				note.LastReviewed = value
			default:
				return Note{}, fmt.Errorf("%s:%d unknown note frontmatter key %q", path, lineNo, key)
			}
		}
		if note.TargetType == "" {
			return Note{}, fmt.Errorf("%s: note frontmatter requires target_type", path)
		}
		if !validNoteTargetType(note.TargetType) {
			return Note{}, fmt.Errorf("%s: note frontmatter has unknown target_type %q", path, note.TargetType)
		}
		if note.TargetID == "" {
			return Note{}, fmt.Errorf("%s: note frontmatter requires target_id", path)
		}
		if note.Title == "" {
			return Note{}, fmt.Errorf("%s: note frontmatter requires title", path)
		}
	}
	note.Text = strings.TrimSpace(text)
	if note.Text == "" {
		return Note{}, fmt.Errorf("%s: note body is empty", path)
	}
	if note.Title == "" {
		note.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return note, nil
}

func readYAMLLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

type csvRow struct {
	Number int
	Values map[string]string
}

func readCSV(path string) ([]map[string]string, error) {
	_, rows, err := readCSVRows(path)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Values)
	}
	return out, nil
}

func readManualCSV(path string, allowed []string, missingOK bool) ([]csvRow, error) {
	headers, rows, err := readCSVRows(path)
	if os.IsNotExist(err) && missingOK {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	allowedSet := map[string]bool{}
	for _, header := range allowed {
		allowedSet[header] = true
	}
	for _, header := range headers {
		if !allowedSet[header] {
			return nil, fmt.Errorf("%s has unknown column %q", path, header)
		}
	}
	return rows, nil
}

func readCSVRows(path string) ([]string, []csvRow, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, nil, nil
	}
	headers := make([]string, len(records[0]))
	seenHeaders := map[string]int{}
	for i, header := range records[0] {
		header = strings.TrimSpace(header)
		if header == "" {
			return nil, nil, fmt.Errorf("%s header column %d is empty", path, i+1)
		}
		if first, ok := seenHeaders[header]; ok {
			return nil, nil, fmt.Errorf("%s has duplicate header %q in columns %d and %d", path, header, first+1, i+1)
		}
		seenHeaders[header] = i
		headers[i] = header
	}
	var out []csvRow
	for i, record := range records[1:] {
		row := map[string]string{}
		for i, header := range headers {
			if i < len(record) {
				row[header] = strings.TrimSpace(record[i])
			}
		}
		out = append(out, csvRow{Number: i + 2, Values: row})
	}
	return headers, out, nil
}

func parseOptionalInt(value string, path string, row int, field string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s row %d has malformed %s %q: %w", path, row, field, value, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s row %d has negative %s %q", path, row, field, value)
	}
	return parsed, nil
}

func setThemeField(theme *Theme, field string, path string, line int) error {
	key, value, ok := splitField(field)
	if !ok {
		return fmt.Errorf("%s:%d malformed theme field %q", path, line, field)
	}
	switch key {
	case "id":
		theme.ID = value
	case "name":
		theme.Name = value
	case "description":
		theme.Description = value
	case "color":
		theme.Color = value
	default:
		return fmt.Errorf("%s:%d unknown theme field %q", path, line, key)
	}
	return nil
}

func setSupplyChainField(chain *SupplyChain, field string, path string, line int) error {
	key, value, ok := splitField(field)
	if !ok {
		return fmt.Errorf("%s:%d malformed supply-chain field %q", path, line, field)
	}
	switch key {
	case "theme_id":
		chain.ThemeID = value
	case "name":
		chain.Name = value
	case "description":
		chain.Description = value
	default:
		return fmt.Errorf("%s:%d unknown supply-chain field %q", path, line, key)
	}
	return nil
}

func setLayerField(layer *SupplyChainLayer, field string, path string, line int) error {
	key, value, ok := splitField(field)
	if !ok {
		return fmt.Errorf("%s:%d malformed layer field %q", path, line, field)
	}
	switch key {
	case "id":
		layer.ID = value
	case "name":
		layer.Name = value
	case "description":
		layer.Description = value
	case "order":
		order, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s:%d layer order %q is not an integer", path, line, value)
		}
		layer.Order = order
		layer.OrderSet = true
	default:
		return fmt.Errorf("%s:%d unknown layer field %q", path, line, key)
	}
	return nil
}

func splitField(field string) (string, string, bool) {
	idx := strings.Index(field, ":")
	if idx < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(field[:idx])
	value := strings.TrimSpace(field[idx+1:])
	value = strings.Trim(value, `"'`)
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func splitSemicolonList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ';' || r == ','
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func countIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

var (
	slugPattern     = regexp.MustCompile(`^[a-z0-9]+([_-][a-z0-9]+)*$`)
	hexColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

func validSlug(value string) bool {
	return slugPattern.MatchString(value)
}

func validHexColor(value string) bool {
	return hexColorPattern.MatchString(value)
}

func parseExposureScore(path string, row int, value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("%s row %d missing exposure_score", path, row)
	}
	score, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s row %d has malformed exposure_score %q: %w", path, row, value, err)
	}
	if score < 0 || score > 5 {
		return 0, fmt.Errorf("%s row %d exposure_score %q is outside 0..5", path, row, value)
	}
	return score, nil
}

func validateExposureRow(exposure Exposure) error {
	location := csvLocation(exposure.SourcePath, exposure.SourceRow)
	if exposure.ThemeID == "" {
		return fmt.Errorf("%s exposure requires theme_id", location)
	}
	if exposure.LayerID == "" {
		return fmt.Errorf("%s exposure requires layer_id", location)
	}
	if countNonEmpty(exposure.Ticker, exposure.ISIN, exposure.CompanyID) == 0 {
		return fmt.Errorf("%s exposure requires at least one of ticker, isin, or company_id", location)
	}
	if exposure.Confidence == "" {
		return fmt.Errorf("%s exposure requires confidence", location)
	}
	if !ValidManualConfidence(exposure.Confidence) {
		return fmt.Errorf("%s exposure has unknown confidence %q", location, exposure.Confidence)
	}
	if err := validateRequiredURL(exposure.SourcePath, exposure.SourceRow, "source_url", exposure.SourceURL); err != nil {
		return err
	}
	if err := validateRequiredReviewDate(exposure.SourcePath, exposure.SourceRow, "last_reviewed", exposure.LastReviewed); err != nil {
		return err
	}
	return nil
}

func validateClassificationOverride(override ClassificationOverride) error {
	location := csvLocation(override.SourcePath, override.SourceRow)
	if override.TargetType == "" {
		return fmt.Errorf("%s classification override requires target_type", location)
	}
	switch override.TargetType {
	case "ticker":
		if err := requireExactTarget(location, "target_type ticker", "ticker", override.Ticker, map[string]string{"isin": override.ISIN, "company_id": override.CompanyID}); err != nil {
			return err
		}
	case "isin":
		if err := requireExactTarget(location, "target_type isin", "isin", override.ISIN, map[string]string{"ticker": override.Ticker, "company_id": override.CompanyID}); err != nil {
			return err
		}
	case "company":
		if err := requireExactTarget(location, "target_type company", "company_id", override.CompanyID, map[string]string{"ticker": override.Ticker, "isin": override.ISIN}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s classification override has unknown target_type %q", location, override.TargetType)
	}
	if override.Sector == "" && override.Industry == "" && override.Country == "" {
		return fmt.Errorf("%s classification override has no classification fields", location)
	}
	if err := validateOptionalURL(override.SourcePath, override.SourceRow, "source_url", override.SourceURL); err != nil {
		return err
	}
	if err := validateOptionalReviewDate(override.SourcePath, override.SourceRow, "last_reviewed", override.LastReviewed); err != nil {
		return err
	}
	return nil
}

func classificationOverrideKey(override ClassificationOverride) string {
	switch override.TargetType {
	case "ticker":
		return "ticker:" + override.Ticker
	case "isin":
		return "isin:" + override.ISIN
	case "company":
		return "company:" + override.CompanyID
	default:
		return override.TargetType + ":"
	}
}

func conflictingClassificationOverride(a, b ClassificationOverride) bool {
	return conflicts(a.Sector, b.Sector) || conflicts(a.Industry, b.Industry) || conflicts(a.Country, b.Country)
}

func validateRelationshipRow(relationship Relationship) error {
	location := csvLocation(relationship.SourcePath, relationship.SourceRow)
	if !validRelationshipType(relationship.RelationshipType) {
		return fmt.Errorf("%s relationship has unknown relationship_type %q", location, relationship.RelationshipType)
	}
	if countNonEmpty(relationship.SourceTicker, relationship.SourceISIN, relationship.SourceCompanyID) != 1 {
		return fmt.Errorf("%s relationship requires exactly one source target field", location)
	}
	if countNonEmpty(relationship.TargetTicker, relationship.TargetISIN, relationship.TargetCompanyID) != 1 {
		return fmt.Errorf("%s relationship requires exactly one target target field", location)
	}
	if relationship.Confidence == "" {
		return fmt.Errorf("%s relationship requires confidence", location)
	}
	if !ValidManualConfidence(relationship.Confidence) {
		return fmt.Errorf("%s relationship has unknown confidence %q", location, relationship.Confidence)
	}
	if err := validateRequiredURL(relationship.SourcePath, relationship.SourceRow, "source_url", relationship.SourceURL); err != nil {
		return err
	}
	if err := validateRequiredReviewDate(relationship.SourcePath, relationship.SourceRow, "last_reviewed", relationship.LastReviewed); err != nil {
		return err
	}
	return nil
}

func validRelationshipType(value string) bool {
	switch value {
	case "peer", "substitute", "upstream_supplier", "downstream_customer", "related_play":
		return true
	default:
		return false
	}
}

func requireExactTarget(location, label, requiredField, requiredValue string, otherFields map[string]string) error {
	if requiredValue == "" {
		return fmt.Errorf("%s %s requires %s", location, label, requiredField)
	}
	for field, value := range otherFields {
		if value != "" {
			return fmt.Errorf("%s %s must not set %s", location, label, field)
		}
	}
	return nil
}

func validateOptionalURL(path string, row int, field string, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return validateURL(path, row, field, value)
}

func validateRequiredURL(path string, row int, field string, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s missing required %s", csvLocation(path, row), field)
	}
	return validateURL(path, row, field, value)
}

func validateURL(path string, row int, field string, value string) error {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || strings.ContainsAny(value, " \t\r\n") || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s has invalid %s %q; expected absolute http or https URL", csvLocation(path, row), field, value)
	}
	return nil
}

func validateOptionalReviewDate(path string, row int, field string, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return validateReviewDate(path, row, field, value)
}

func validateRequiredReviewDate(path string, row int, field string, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s missing required %s", csvLocation(path, row), field)
	}
	return validateReviewDate(path, row, field, value)
}

func validateReviewDate(path string, row int, field string, value string) error {
	if _, err := time.Parse("2006-01-02", strings.TrimSpace(value)); err != nil {
		return fmt.Errorf("%s has invalid %s %q; expected YYYY-MM-DD", csvLocation(path, row), field, value)
	}
	return nil
}

func ValidManualConfidence(value string) bool {
	switch value {
	case "manual_high", "manual_medium", "manual_low", "rule_high", "rule_medium", "rule_low":
		return true
	default:
		return false
	}
}

func validNoteTargetType(value string) bool {
	switch value {
	case "ticker", "company", "security", "sector", "industry", "theme", "layer":
		return true
	default:
		return false
	}
}

func csvLocation(path string, row int) string {
	if path == "" {
		path = "manual CSV"
	}
	if row <= 0 {
		return path
	}
	return fmt.Sprintf("%s row %d", path, row)
}

func yamlLocation(path string, line int) string {
	if path == "" {
		path = "manual YAML"
	}
	if line <= 0 {
		return path
	}
	return fmt.Sprintf("%s:%d", path, line)
}

func countNonEmpty(values ...string) int {
	count := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func sortedUnique(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func Validate(data ManualData) error {
	themes := map[string]Theme{}
	for _, theme := range data.Themes {
		if theme.ID == "" {
			return fmt.Errorf("%s theme has empty id", yamlLocation(theme.SourcePath, theme.SourceLine))
		}
		if !validSlug(theme.ID) {
			return fmt.Errorf("%s theme id %q is not a slug", yamlLocation(theme.SourcePath, theme.SourceLine), theme.ID)
		}
		if _, ok := themes[theme.ID]; ok {
			return fmt.Errorf("%s duplicate theme id %q", yamlLocation(theme.SourcePath, theme.SourceLine), theme.ID)
		}
		if theme.Name == "" {
			return fmt.Errorf("%s theme %q has empty name", yamlLocation(theme.SourcePath, theme.SourceLine), theme.ID)
		}
		if theme.Color != "" && !validHexColor(theme.Color) {
			return fmt.Errorf("%s theme %q has invalid color %q; expected #RRGGBB", yamlLocation(theme.SourcePath, theme.SourceLine), theme.ID, theme.Color)
		}
		themes[theme.ID] = theme
	}
	layers := map[string]bool{}
	for _, chain := range data.SupplyChains {
		if _, ok := themes[chain.ThemeID]; !ok {
			return fmt.Errorf("%s supply chain references unknown theme %q", yamlLocation(chain.SourcePath, chain.SourceLine), chain.ThemeID)
		}
		if chain.Name == "" {
			return fmt.Errorf("%s supply chain for theme %q has empty name", yamlLocation(chain.SourcePath, chain.SourceLine), chain.ThemeID)
		}
		if len(chain.Layers) == 0 {
			return fmt.Errorf("%s supply chain for theme %q has no layers", yamlLocation(chain.SourcePath, chain.SourceLine), chain.ThemeID)
		}
		for _, layer := range chain.Layers {
			if layer.ID == "" {
				return fmt.Errorf("%s layer has empty id", yamlLocation(layer.SourcePath, layer.SourceLine))
			}
			if layer.Name == "" {
				return fmt.Errorf("%s layer %q has empty name", yamlLocation(layer.SourcePath, layer.SourceLine), layer.ID)
			}
			layers[chain.ThemeID+"|"+layer.ID] = true
		}
	}
	for _, exposure := range data.Exposures {
		if _, ok := themes[exposure.ThemeID]; !ok {
			return fmt.Errorf("%s exposure references unknown theme %q", csvLocation(exposure.SourcePath, exposure.SourceRow), exposure.ThemeID)
		}
		if !layers[exposure.ThemeID+"|"+exposure.LayerID] {
			return fmt.Errorf("%s exposure references unknown layer %q for theme %q", csvLocation(exposure.SourcePath, exposure.SourceRow), exposure.LayerID, exposure.ThemeID)
		}
	}
	for _, relationship := range data.Relationships {
		if relationship.ThemeID != "" {
			if _, ok := themes[relationship.ThemeID]; !ok {
				return fmt.Errorf("%s relationship references unknown theme %q", csvLocation(relationship.SourcePath, relationship.SourceRow), relationship.ThemeID)
			}
		}
		if relationship.LayerID != "" {
			if relationship.ThemeID == "" {
				return fmt.Errorf("%s relationship layer_id %q requires theme_id", csvLocation(relationship.SourcePath, relationship.SourceRow), relationship.LayerID)
			}
			if !layers[relationship.ThemeID+"|"+relationship.LayerID] {
				return fmt.Errorf("%s relationship references unknown layer %q for theme %q", csvLocation(relationship.SourcePath, relationship.SourceRow), relationship.LayerID, relationship.ThemeID)
			}
		}
	}
	if err := validateIdentityOverrides(data.IdentityOverrides); err != nil {
		return err
	}
	return nil
}

func validateIdentityOverrides(overrides []IdentityOverride) error {
	seen := map[string]IdentityOverride{}
	for i, override := range overrides {
		row := override.SourceRow
		if row == 0 {
			row = i + 2
		}
		location := csvLocation(firstNonEmpty(override.SourcePath, "identity_overrides.csv"), row)
		switch override.TargetType {
		case "ticker":
			if override.Ticker == "" {
				return fmt.Errorf("%s target_type ticker requires ticker", location)
			}
		case "isin":
			if override.ISIN == "" {
				return fmt.Errorf("%s target_type isin requires isin", location)
			}
		case "security":
			if override.SecurityID == "" {
				return fmt.Errorf("%s target_type security requires security_id", location)
			}
		case "company":
			if override.CompanyID == "" {
				return fmt.Errorf("%s target_type company requires company_id", location)
			}
		default:
			return fmt.Errorf("%s has unknown target_type %q", location, override.TargetType)
		}
		if override.OverrideSecurityID == "" && override.OverrideCompanyID == "" && override.Category == "" && len(override.Flags) == 0 && override.Confidence == "" {
			return fmt.Errorf("%s has no override fields", location)
		}
		if override.Category != "" && !validIdentityCategory(override.Category) {
			return fmt.Errorf("%s has unknown category %q", location, override.Category)
		}
		if override.Confidence != "" && !validIdentityConfidence(override.Confidence) {
			return fmt.Errorf("%s has unknown confidence %q", location, override.Confidence)
		}
		for _, flag := range override.Flags {
			if !validIdentityFlag(flag) {
				return fmt.Errorf("%s has unknown flag %q", location, flag)
			}
		}
		key := identityOverrideKey(override)
		if existing, ok := seen[key]; ok && conflictingIdentityOverride(existing, override) {
			return fmt.Errorf("%s conflicts with another override for %s", location, key)
		}
		seen[key] = override
	}
	return nil
}

func identityOverrideKey(override IdentityOverride) string {
	switch override.TargetType {
	case "ticker":
		return "ticker:" + override.Ticker
	case "isin":
		return "isin:" + override.ISIN
	case "security":
		return "security:" + override.SecurityID
	case "company":
		return "company:" + override.CompanyID
	default:
		return override.TargetType + ":"
	}
}

func conflictingIdentityOverride(a, b IdentityOverride) bool {
	return conflicts(a.OverrideSecurityID, b.OverrideSecurityID) ||
		conflicts(a.OverrideCompanyID, b.OverrideCompanyID) ||
		conflicts(a.Category, b.Category) ||
		conflicts(a.Confidence, b.Confidence)
}

func conflicts(a, b string) bool {
	return a != "" && b != "" && a != b
}

func validIdentityCategory(value string) bool {
	switch value {
	case "stock", "etf", "fund", "investment_trust", "warrant", "crypto", "forex", "bond", "commodity", "other":
		return true
	default:
		return false
	}
}

func validIdentityFlag(value string) bool {
	switch value {
	case "inverse", "short", "leveraged", "synthetic", "hedged", "accumulating", "distributing", "adr", "gdr", "fund_like":
		return true
	default:
		return false
	}
}

func validIdentityConfidence(value string) bool {
	return ValidManualConfidence(value)
}
