package enrichment

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ErrCacheMiss = errors.New("enrichment cache miss")

type Request struct {
	Ticker       string
	ISIN         string
	Name         string
	CurrencyCode string
	ExchangeCode string
}

type Profile struct {
	Symbol      string `json:"symbol,omitempty"`
	Name        string `json:"name,omitempty"`
	Sector      string `json:"sector,omitempty"`
	Industry    string `json:"industry,omitempty"`
	Exchange    string `json:"exchange,omitempty"`
	Currency    string `json:"currency,omitempty"`
	Country     string `json:"country,omitempty"`
	MarketCap   int64  `json:"marketCap,omitempty"`
	Source      string `json:"source,omitempty"`
	RetrievedAt string `json:"retrievedAt,omitempty"`
	Error       string `json:"error,omitempty"`
}

type Provider interface {
	Lookup(context.Context, Request) (Profile, error)
}

type CacheProvider struct {
	Dir   string
	Inner Provider
	Now   func() time.Time
}

func (p CacheProvider) Lookup(ctx context.Context, req Request) (Profile, error) {
	if p.Dir == "" {
		if p.Inner == nil {
			return Profile{}, ErrCacheMiss
		}
		return p.Inner.Lookup(ctx, req)
	}
	path := filepath.Join(p.Dir, cacheKey(req)+".json")
	if b, err := os.ReadFile(path); err == nil {
		var profile Profile
		if err := json.Unmarshal(b, &profile); err != nil {
			return Profile{}, fmt.Errorf("decode enrichment cache %s: %w", path, err)
		}
		if profile.Error != "" {
			return profile, errors.New(profile.Error)
		}
		return profile, nil
	}
	if p.Inner == nil {
		return Profile{}, ErrCacheMiss
	}
	profile, err := p.Inner.Lookup(ctx, req)
	if profile.RetrievedAt == "" {
		now := time.Now().UTC()
		if p.Now != nil {
			now = p.Now().UTC()
		}
		profile.RetrievedAt = now.Format(time.RFC3339)
	}
	if err != nil {
		profile.Error = err.Error()
	}
	if mkErr := os.MkdirAll(p.Dir, 0o755); mkErr != nil {
		return profile, mkErr
	}
	b, encErr := json.MarshalIndent(profile, "", "  ")
	if encErr != nil {
		return profile, encErr
	}
	if writeErr := os.WriteFile(path, append(b, '\n'), 0o644); writeErr != nil {
		return profile, writeErr
	}
	return profile, err
}

type YahooProvider struct {
	HTTPClient *http.Client
}

func (p YahooProvider) Lookup(ctx context.Context, req Request) (Profile, error) {
	symbols, err := p.resolveSymbols(ctx, req)
	if err != nil {
		return Profile{}, err
	}
	for _, symbol := range symbols {
		profile, err := p.quoteSummary(ctx, symbol)
		if err == nil {
			return profile, nil
		}
	}
	return Profile{}, fmt.Errorf("no yahoo profile matched %s", req.Ticker)
}

func (p YahooProvider) resolveSymbols(ctx context.Context, req Request) ([]string, error) {
	queries := []string{}
	if req.ISIN != "" {
		queries = append(queries, req.ISIN)
	}
	queries = append(queries, CandidateSymbols(req.Ticker)...)
	queries = append(queries, req.Name)

	seen := map[string]bool{}
	var out []string
	for _, q := range queries {
		q = strings.TrimSpace(q)
		if q == "" || seen[strings.ToUpper(q)] {
			continue
		}
		seen[strings.ToUpper(q)] = true
		found, err := p.search(ctx, q)
		if err == nil {
			for _, sym := range found {
				key := strings.ToUpper(sym)
				if sym != "" && !seen[key] {
					seen[key] = true
					out = append(out, sym)
				}
			}
		}
		if len(out) == 0 {
			out = append(out, q)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no yahoo symbol candidates for %s", req.Ticker)
	}
	return out, nil
}

func (p YahooProvider) search(ctx context.Context, query string) ([]string, error) {
	u := "https://query1.finance.yahoo.com/v1/finance/search?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	var payload struct {
		Quotes []struct {
			Symbol    string `json:"symbol"`
			QuoteType string `json:"quoteType"`
		} `json:"quotes"`
	}
	if err := p.doJSON(req, &payload); err != nil {
		return nil, err
	}
	var out []string
	for _, quote := range payload.Quotes {
		typ := strings.ToUpper(quote.QuoteType)
		if quote.Symbol != "" && (typ == "EQUITY" || typ == "ETF" || typ == "") {
			out = append(out, quote.Symbol)
		}
	}
	return out, nil
}

func (p YahooProvider) quoteSummary(ctx context.Context, symbol string) (Profile, error) {
	u := "https://query1.finance.yahoo.com/v10/finance/quoteSummary/" + url.PathEscape(symbol) + "?modules=price,summaryProfile,summaryDetail"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Profile{}, err
	}
	req.Header.Set("Accept", "application/json")
	var payload struct {
		QuoteSummary struct {
			Result []struct {
				Price struct {
					Symbol       string `json:"symbol"`
					ShortName    string `json:"shortName"`
					LongName     string `json:"longName"`
					ExchangeName string `json:"exchangeName"`
					Currency     string `json:"currency"`
					MarketCap    rawInt `json:"marketCap"`
				} `json:"price"`
				SummaryProfile struct {
					Sector   string `json:"sector"`
					Industry string `json:"industry"`
					Country  string `json:"country"`
				} `json:"summaryProfile"`
				SummaryDetail struct {
					MarketCap rawInt `json:"marketCap"`
				} `json:"summaryDetail"`
			} `json:"result"`
			Error any `json:"error"`
		} `json:"quoteSummary"`
	}
	if err := p.doJSON(req, &payload); err != nil {
		return Profile{}, err
	}
	if len(payload.QuoteSummary.Result) == 0 {
		return Profile{}, fmt.Errorf("empty yahoo quoteSummary for %s", symbol)
	}
	result := payload.QuoteSummary.Result[0]
	name := result.Price.LongName
	if name == "" {
		name = result.Price.ShortName
	}
	marketCap := result.Price.MarketCap.Raw
	if marketCap == 0 {
		marketCap = result.SummaryDetail.MarketCap.Raw
	}
	return Profile{
		Symbol:    firstNonEmpty(result.Price.Symbol, symbol),
		Name:      name,
		Sector:    result.SummaryProfile.Sector,
		Industry:  result.SummaryProfile.Industry,
		Exchange:  result.Price.ExchangeName,
		Currency:  result.Price.Currency,
		Country:   result.SummaryProfile.Country,
		MarketCap: marketCap,
		Source:    "yahoo",
	}, nil
}

func (p YahooProvider) doJSON(req *http.Request, out any) error {
	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("yahoo returned %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type rawInt struct {
	Raw int64 `json:"raw"`
}

func CandidateSymbols(t212Ticker string) []string {
	t := strings.TrimSpace(t212Ticker)
	if t == "" {
		return nil
	}
	parts := strings.Split(t, "_")
	candidates := []string{t}
	if len(parts) >= 3 {
		base := strings.Join(parts[:len(parts)-2], "_")
		exchange := parts[len(parts)-2]
		if base != "" {
			candidates = append(candidates, base)
			if suffix := yahooSuffix(exchange); suffix != "" {
				candidates = append(candidates, base+suffix)
			}
		}
	}
	seen := map[string]bool{}
	out := candidates[:0]
	for _, candidate := range candidates {
		key := strings.ToUpper(candidate)
		if candidate != "" && !seen[key] {
			seen[key] = true
			out = append(out, candidate)
		}
	}
	sort.Strings(out)
	return out
}

func yahooSuffix(exchange string) string {
	switch strings.ToUpper(exchange) {
	case "L", "LN", "LSE":
		return ".L"
	case "DE", "XETRA":
		return ".DE"
	case "PA":
		return ".PA"
	case "AS":
		return ".AS"
	case "MI":
		return ".MI"
	case "MC":
		return ".MC"
	case "SW":
		return ".SW"
	case "US", "NASDAQ", "NYSE":
		return ""
	default:
		return ""
	}
}

func cacheKey(req Request) string {
	h := sha1.Sum([]byte(strings.Join([]string{
		strings.ToUpper(req.Ticker),
		strings.ToUpper(req.ISIN),
		strings.ToUpper(req.Name),
	}, "|")))
	return hex.EncodeToString(h[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
