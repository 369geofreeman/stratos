package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"statos/internal/catalogue"
)

type SearchDocument struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle,omitempty"`
	Text     string   `json:"text"`
	Tickers  []string `json:"tickers,omitempty"`
}

func WriteSiteData(dir string, cat *catalogue.Catalogue) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string]any{
		"catalogue.json":      cat,
		"companies.json":      cat.Companies,
		"sectors.json":        cat.Sectors,
		"industries.json":     cat.Industries,
		"themes.json":         cat.Themes,
		"supply_chains.json":  cat.SupplyChains,
		"search_index.json":   BuildSearchIndex(cat),
		"build_manifest.json": cat.Manifest,
	}
	for name, value := range files {
		if err := writeJSON(filepath.Join(dir, name), value); err != nil {
			return err
		}
	}
	if err := writeTickersCSV(filepath.Join(dir, "tickers.csv"), cat.Tickers); err != nil {
		return err
	}
	if err := writeUnclassifiedCSV(filepath.Join(dir, "unclassified.csv"), cat.Unclassified); err != nil {
		return err
	}
	return nil
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

func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func writeTickersCSV(path string, tickers []catalogue.Ticker) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	w := csv.NewWriter(file)
	defer w.Flush()
	headers := []string{"ticker", "name", "isin", "company_id", "security_id", "type", "currency", "exchange", "yahoo_symbol", "sector", "industry", "country", "market_cap", "directionality", "themes", "layers", "unclassified"}
	if err := w.Write(headers); err != nil {
		return err
	}
	for _, ticker := range tickers {
		row := []string{
			ticker.Ticker,
			ticker.Name,
			ticker.ISIN,
			ticker.CompanyID,
			ticker.SecurityID,
			ticker.Type,
			ticker.CurrencyCode,
			ticker.ExchangeName,
			ticker.YahooSymbol,
			ticker.Sector,
			ticker.Industry,
			ticker.Country,
			strconv.FormatInt(ticker.MarketCap, 10),
			ticker.Directionality,
			joinCSVList(ticker.ThemeIDs),
			joinCSVList(ticker.LayerIDs),
			strconv.FormatBool(ticker.Unclassified),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return w.Error()
}

func writeUnclassifiedCSV(path string, rows []catalogue.UnclassifiedRow) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	w := csv.NewWriter(file)
	defer w.Flush()
	if err := w.Write([]string{"ticker", "company_id", "name", "isin", "reason"}); err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.Write([]string{row.Ticker, row.CompanyID, row.Name, row.ISIN, row.Reason}); err != nil {
			return err
		}
	}
	return w.Error()
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
