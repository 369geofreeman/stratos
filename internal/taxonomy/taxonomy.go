package taxonomy

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type ManualData struct {
	Themes           []Theme
	SupplyChains     []SupplyChain
	Exposures        []Exposure
	CompanyOverrides map[string]CompanyOverride
	TickerOverrides  map[string]TickerOverride
	Notes            []Note
}

type Theme struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
}

type SupplyChain struct {
	ThemeID     string             `json:"themeId"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Layers      []SupplyChainLayer `json:"layers"`
}

type SupplyChainLayer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Order       int    `json:"order"`
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
}

type CompanyOverride struct {
	CompanyID    string `json:"companyId"`
	Name         string `json:"name,omitempty"`
	Sector       string `json:"sector,omitempty"`
	Industry     string `json:"industry,omitempty"`
	Country      string `json:"country,omitempty"`
	SourceURL    string `json:"sourceUrl,omitempty"`
	LastReviewed string `json:"lastReviewed,omitempty"`
}

type TickerOverride struct {
	Ticker       string `json:"ticker"`
	CompanyID    string `json:"companyId,omitempty"`
	Name         string `json:"name,omitempty"`
	Sector       string `json:"sector,omitempty"`
	Industry     string `json:"industry,omitempty"`
	Country      string `json:"country,omitempty"`
	YahooSymbol  string `json:"yahooSymbol,omitempty"`
	SourceURL    string `json:"sourceUrl,omitempty"`
	LastReviewed string `json:"lastReviewed,omitempty"`
}

type Note struct {
	TargetType string   `json:"targetType"`
	TargetID   string   `json:"targetId"`
	Title      string   `json:"title"`
	Tags       []string `json:"tags,omitempty"`
	Path       string   `json:"path"`
	Text       string   `json:"text"`
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
	if data.Exposures, err = LoadExposures(filepath.Join(dir, "exposures.csv")); err != nil {
		return ManualData{}, err
	}
	if data.Notes, err = LoadNotes(filepath.Join(dir, "notes")); err != nil {
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
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == "themes:" {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			if current != nil {
				out = append(out, *current)
			}
			current = &Theme{}
			setThemeField(current, strings.TrimPrefix(trimmed, "- "))
			continue
		}
		if current != nil {
			setThemeField(current, trimmed)
		}
	}
	if current != nil {
		out = append(out, *current)
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

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == "supply_chains:" {
			continue
		}
		indent := countIndent(line)
		switch {
		case indent == 2 && strings.HasPrefix(trimmed, "- "):
			flushChain()
			chain = &SupplyChain{}
			inLayers = false
			setSupplyChainField(chain, strings.TrimPrefix(trimmed, "- "))
		case indent == 4 && trimmed == "layers:":
			inLayers = true
		case indent == 4 && chain != nil && !inLayers:
			setSupplyChainField(chain, trimmed)
		case indent == 6 && strings.HasPrefix(trimmed, "- "):
			flushLayer()
			layer = &SupplyChainLayer{}
			setLayerField(layer, strings.TrimPrefix(trimmed, "- "))
		case indent >= 8 && layer != nil:
			setLayerField(layer, trimmed)
		}
	}
	flushChain()
	return out, nil
}

func LoadCompanyOverrides(path string) (map[string]CompanyOverride, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}
	out := map[string]CompanyOverride{}
	for _, row := range rows {
		id := row["company_id"]
		if id == "" {
			continue
		}
		out[id] = CompanyOverride{
			CompanyID:    id,
			Name:         row["name"],
			Sector:       row["sector"],
			Industry:     row["industry"],
			Country:      row["country"],
			SourceURL:    row["source_url"],
			LastReviewed: row["last_reviewed"],
		}
	}
	return out, nil
}

func LoadTickerOverrides(path string) (map[string]TickerOverride, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}
	out := map[string]TickerOverride{}
	for _, row := range rows {
		ticker := row["ticker"]
		if ticker == "" {
			continue
		}
		out[ticker] = TickerOverride{
			Ticker:       ticker,
			CompanyID:    row["company_id"],
			Name:         row["name"],
			Sector:       row["sector"],
			Industry:     row["industry"],
			Country:      row["country"],
			YahooSymbol:  row["yahoo_symbol"],
			SourceURL:    row["source_url"],
			LastReviewed: row["last_reviewed"],
		}
	}
	return out, nil
}

func LoadExposures(path string) ([]Exposure, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}
	var out []Exposure
	for _, row := range rows {
		score, _ := strconv.ParseFloat(row["exposure_score"], 64)
		if row["theme_id"] == "" || row["layer_id"] == "" {
			continue
		}
		out = append(out, Exposure{
			ThemeID:       row["theme_id"],
			LayerID:       row["layer_id"],
			Ticker:        row["ticker"],
			ISIN:          row["isin"],
			CompanyID:     row["company_id"],
			ExposureScore: score,
			Confidence:    row["confidence"],
			SourceURL:     row["source_url"],
			Rationale:     row["rationale"],
			LastReviewed:  row["last_reviewed"],
		})
	}
	return out, nil
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
		if idx := strings.Index(rest, "\n---\n"); idx >= 0 {
			meta := rest[:idx]
			text = rest[idx+5:]
			for _, line := range strings.Split(meta, "\n") {
				key, value, ok := splitField(strings.TrimSpace(line))
				if !ok {
					continue
				}
				switch key {
				case "target_type":
					note.TargetType = value
				case "target_id":
					note.TargetID = value
				case "title":
					note.Title = value
				case "tags":
					note.Tags = splitList(value)
				}
			}
		}
	}
	note.Text = strings.TrimSpace(text)
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

func readCSV(path string) ([]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	headers := records[0]
	var out []map[string]string
	for _, record := range records[1:] {
		row := map[string]string{}
		for i, header := range headers {
			if i < len(record) {
				row[header] = strings.TrimSpace(record[i])
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func setThemeField(theme *Theme, field string) {
	key, value, ok := splitField(field)
	if !ok {
		return
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
	}
}

func setSupplyChainField(chain *SupplyChain, field string) {
	key, value, ok := splitField(field)
	if !ok {
		return
	}
	switch key {
	case "theme_id":
		chain.ThemeID = value
	case "name":
		chain.Name = value
	case "description":
		chain.Description = value
	}
}

func setLayerField(layer *SupplyChainLayer, field string) {
	key, value, ok := splitField(field)
	if !ok {
		return
	}
	switch key {
	case "id":
		layer.ID = value
	case "name":
		layer.Name = value
	case "description":
		layer.Description = value
	case "order":
		order, _ := strconv.Atoi(value)
		layer.Order = order
	}
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

func countIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func Validate(data ManualData) error {
	themes := map[string]bool{}
	for _, theme := range data.Themes {
		if theme.ID == "" {
			return fmt.Errorf("theme has empty id")
		}
		themes[theme.ID] = true
	}
	layers := map[string]bool{}
	for _, chain := range data.SupplyChains {
		if !themes[chain.ThemeID] {
			return fmt.Errorf("supply chain references unknown theme %q", chain.ThemeID)
		}
		for _, layer := range chain.Layers {
			layers[chain.ThemeID+"|"+layer.ID] = true
		}
	}
	for _, exposure := range data.Exposures {
		if !themes[exposure.ThemeID] {
			return fmt.Errorf("exposure references unknown theme %q", exposure.ThemeID)
		}
		if !layers[exposure.ThemeID+"|"+exposure.LayerID] {
			return fmt.Errorf("exposure references unknown layer %q for theme %q", exposure.LayerID, exposure.ThemeID)
		}
	}
	return nil
}
