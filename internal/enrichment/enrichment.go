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

const (
	CacheSchemaVersion  = 1
	DefaultCacheTTL     = 30 * 24 * time.Hour
	DefaultYahooBaseURL = "https://query1.finance.yahoo.com"

	StatusHit           = "hit"
	StatusFailure       = "failure"
	StatusAmbiguous     = "ambiguous"
	StatusCacheMiss     = "cache_miss"
	StatusUnknownSchema = "unknown_schema"

	CacheStatusHit           = "hit"
	CacheStatusMiss          = "miss"
	CacheStatusUnknownSchema = "unknown_schema"
)

var (
	ErrCacheMiss          = errors.New("enrichment cache miss")
	ErrAmbiguousMatch     = errors.New("ambiguous enrichment match")
	ErrUnknownCacheSchema = errors.New("unknown enrichment cache schema")
)

type Request struct {
	Ticker       string
	ISIN         string
	Name         string
	CurrencyCode string
	ExchangeCode string
}

type RequestSnapshot struct {
	Ticker           string   `json:"ticker,omitempty"`
	ISIN             string   `json:"isin,omitempty"`
	Name             string   `json:"name,omitempty"`
	CandidateSymbols []string `json:"candidateSymbols,omitempty"`
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

type Candidate struct {
	Symbol    string `json:"symbol,omitempty"`
	Name      string `json:"name,omitempty"`
	Exchange  string `json:"exchange,omitempty"`
	Currency  string `json:"currency,omitempty"`
	QuoteType string `json:"quoteType,omitempty"`
	Source    string `json:"source,omitempty"`
}

type Result struct {
	Provider         string          `json:"provider,omitempty"`
	Request          RequestSnapshot `json:"request,omitempty"`
	Profile          Profile         `json:"profile,omitempty"`
	Candidates       []Candidate     `json:"candidates,omitempty"`
	Status           string          `json:"status,omitempty"`
	Error            string          `json:"error,omitempty"`
	RetrievedAt      string          `json:"retrievedAt,omitempty"`
	AttemptedSymbols []string        `json:"-"`
	CacheStatus      string          `json:"-"`
	CachePath        string          `json:"-"`
	Stale            bool            `json:"-"`
}

type Failure struct {
	Ticker           string `json:"ticker"`
	ISIN             string `json:"isin,omitempty"`
	Name             string `json:"name,omitempty"`
	Provider         string `json:"provider,omitempty"`
	AttemptedSymbols string `json:"attemptedSymbols,omitempty"`
	Status           string `json:"status,omitempty"`
	Error            string `json:"error,omitempty"`
	NextAction       string `json:"nextAction,omitempty"`
}

type cacheEntry struct {
	SchemaVersion int             `json:"schemaVersion"`
	Provider      string          `json:"provider,omitempty"`
	Request       RequestSnapshot `json:"request,omitempty"`
	Profile       Profile         `json:"profile,omitempty"`
	Candidates    []Candidate     `json:"candidates,omitempty"`
	Status        string          `json:"status,omitempty"`
	Error         string          `json:"error,omitempty"`
	RetrievedAt   string          `json:"retrievedAt,omitempty"`
}

// Provider is the replaceable enrichment boundary.
//
// Implementations must treat Trading 212 metadata in Request as the source
// universe and return only supplemental data. They should never write generated
// site files directly. Return StatusAmbiguous with Candidates when a lookup has
// multiple plausible matches and provider fields should not be applied.
type Provider interface {
	Lookup(context.Context, Request) (Result, error)
}

type NamedProvider interface {
	Name() string
}

type CacheProvider struct {
	Dir   string
	Inner Provider
	Now   func() time.Time
	TTL   time.Duration
}

func (p CacheProvider) Lookup(ctx context.Context, req Request) (Result, error) {
	if p.Dir == "" {
		if p.Inner == nil {
			return cacheMissResult(req, ""), ErrCacheMiss
		}
		result, err := p.Inner.Lookup(ctx, req)
		return p.withDefaults(req, result, err, ""), err
	}
	path := CachePath(p.Dir, req)
	if b, err := os.ReadFile(path); err == nil {
		var entry cacheEntry
		if err := json.Unmarshal(b, &entry); err != nil {
			result := failureResult(req, path, fmt.Sprintf("decode enrichment cache %s: %v", path, err))
			result.CacheStatus = CacheStatusUnknownSchema
			return result, fmt.Errorf("decode enrichment cache %s: %w", path, err)
		}
		if entry.SchemaVersion != CacheSchemaVersion {
			message := fmt.Sprintf("%s %d in %s", ErrUnknownCacheSchema, entry.SchemaVersion, path)
			result := Result{
				Provider:    firstNonEmpty(entry.Provider, providerName(p.Inner), "unknown"),
				Request:     firstNonEmptyRequest(entry.Request, requestSnapshot(req)),
				Candidates:  entry.Candidates,
				Status:      StatusUnknownSchema,
				Error:       message,
				RetrievedAt: entry.RetrievedAt,
				CacheStatus: CacheStatusUnknownSchema,
				CachePath:   path,
			}
			return result, fmt.Errorf("%w: schemaVersion %d in %s", ErrUnknownCacheSchema, entry.SchemaVersion, path)
		}
		result := p.resultFromEntry(req, path, entry)
		switch result.Status {
		case StatusHit:
			return result, nil
		case StatusFailure:
			return result, errors.New(firstNonEmpty(result.Error, "cached enrichment failure"))
		case StatusAmbiguous:
			return result, fmt.Errorf("%w: %s", ErrAmbiguousMatch, firstNonEmpty(result.Error, "cached ambiguous enrichment match"))
		case StatusCacheMiss:
			return result, ErrCacheMiss
		case StatusUnknownSchema:
			return result, fmt.Errorf("%w: %s", ErrUnknownCacheSchema, firstNonEmpty(result.Error, "cached unknown schema"))
		default:
			return result, fmt.Errorf("unknown cached enrichment status %q in %s", result.Status, path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		result := failureResult(req, path, fmt.Sprintf("read enrichment cache %s: %v", path, err))
		result.Provider = firstNonEmpty(providerName(p.Inner), "cache")
		return result, err
	}
	if p.Inner == nil {
		return cacheMissResult(req, path), ErrCacheMiss
	}
	result, err := p.Inner.Lookup(ctx, req)
	result = p.withDefaults(req, result, err, path)
	result.CacheStatus = CacheStatusMiss
	if mkErr := os.MkdirAll(p.Dir, 0o755); mkErr != nil {
		return result, mkErr
	}
	entry := cacheEntry{
		SchemaVersion: CacheSchemaVersion,
		Provider:      result.Provider,
		Request:       result.Request,
		Profile:       result.Profile,
		Candidates:    result.Candidates,
		Status:        result.Status,
		Error:         result.Error,
		RetrievedAt:   result.RetrievedAt,
	}
	b, encErr := json.MarshalIndent(entry, "", "  ")
	if encErr != nil {
		return result, encErr
	}
	if writeErr := os.WriteFile(path, append(b, '\n'), 0o644); writeErr != nil {
		return result, writeErr
	}
	return result, err
}

type YahooProvider struct {
	HTTPClient *http.Client
	BaseURL    string
}

func (p YahooProvider) Name() string {
	return "yahoo"
}

func (p YahooProvider) Lookup(ctx context.Context, req Request) (Result, error) {
	result := Result{
		Provider: "yahoo",
		Request:  requestSnapshot(req),
		Status:   StatusFailure,
	}
	symbols, candidates, err := p.resolveSymbols(ctx, req)
	result.Candidates = candidates
	result.AttemptedSymbols = appendUniqueStrings(result.AttemptedSymbols, symbols...)
	if err != nil {
		result.Status = statusForError(err)
		result.Error = err.Error()
		return result, err
	}
	var lastErr error
	for _, symbol := range symbols {
		profile, err := p.quoteSummary(ctx, symbol)
		if err == nil {
			profile.Source = firstNonEmpty(profile.Source, p.Name())
			result.Profile = profile
			result.Status = StatusHit
			return result, nil
		}
		lastErr = err
	}
	message := fmt.Sprintf("no yahoo profile matched %s using %s", req.Ticker, strings.Join(symbols, ";"))
	if lastErr != nil {
		message += ": " + lastErr.Error()
	}
	result.Error = message
	return result, errors.New(message)
}

func (p YahooProvider) resolveSymbols(ctx context.Context, req Request) ([]string, []Candidate, error) {
	var symbols []string
	var candidates []Candidate
	var searchErrors []string
	if req.ISIN != "" {
		found, err := p.searchCandidates(ctx, req.ISIN, "isin_search")
		if err != nil {
			searchErrors = append(searchErrors, err.Error())
		} else {
			plausible := plausibleCandidates(found)
			candidates = appendCandidates(candidates, plausible...)
			switch len(plausible) {
			case 0:
			case 1:
				symbols = appendUniqueStrings(symbols, plausible[0].Symbol)
			default:
				return nil, candidates, ambiguousError(req, "ISIN", plausible)
			}
		}
	}

	symbols = appendUniqueStrings(symbols, CandidateSymbols(req.Ticker)...)
	if len(symbols) == 0 && req.Name != "" {
		found, err := p.searchCandidates(ctx, req.Name, "name_search")
		if err != nil {
			searchErrors = append(searchErrors, err.Error())
		} else {
			plausible := plausibleCandidates(found)
			candidates = appendCandidates(candidates, plausible...)
			switch len(plausible) {
			case 0:
			case 1:
				symbols = appendUniqueStrings(symbols, plausible[0].Symbol)
			default:
				return nil, candidates, ambiguousError(req, "name", plausible)
			}
		}
	}

	if len(symbols) == 0 {
		message := fmt.Sprintf("no yahoo symbol candidates for %s", req.Ticker)
		if len(searchErrors) > 0 {
			message += ": " + strings.Join(searchErrors, "; ")
		}
		return nil, candidates, errors.New(message)
	}
	return symbols, candidates, nil
}

func (p YahooProvider) searchCandidates(ctx context.Context, query string, source string) ([]Candidate, error) {
	u := strings.TrimRight(p.baseURL(), "/") + "/v1/finance/search?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	var payload struct {
		Quotes []struct {
			Symbol       string `json:"symbol"`
			ShortName    string `json:"shortname"`
			LongName     string `json:"longname"`
			Exchange     string `json:"exchange"`
			ExchangeDisp string `json:"exchDisp"`
			Currency     string `json:"currency"`
			QuoteType    string `json:"quoteType"`
		} `json:"quotes"`
	}
	if err := p.doJSON(req, &payload); err != nil {
		return nil, err
	}
	var out []Candidate
	for _, quote := range payload.Quotes {
		name := firstNonEmpty(quote.LongName, quote.ShortName)
		exchange := firstNonEmpty(quote.ExchangeDisp, quote.Exchange)
		if quote.Symbol != "" {
			out = append(out, Candidate{
				Symbol:    quote.Symbol,
				Name:      name,
				Exchange:  exchange,
				Currency:  quote.Currency,
				QuoteType: quote.QuoteType,
				Source:    source,
			})
		}
	}
	return out, nil
}

func (p YahooProvider) quoteSummary(ctx context.Context, symbol string) (Profile, error) {
	u := strings.TrimRight(p.baseURL(), "/") + "/v10/finance/quoteSummary/" + url.PathEscape(symbol) + "?modules=price,summaryProfile,summaryDetail"
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
		Source:    p.Name(),
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

func (p YahooProvider) baseURL() string {
	return firstNonEmpty(p.BaseURL, DefaultYahooBaseURL)
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
	var candidates []string
	if len(parts) >= 3 {
		base := strings.Join(parts[:len(parts)-2], "_")
		exchange := parts[len(parts)-2]
		if base != "" {
			if suffix := yahooSuffix(exchange); suffix != "" {
				candidates = append(candidates, base+suffix)
			}
			candidates = append(candidates, base)
		}
	}
	candidates = append(candidates, t)
	seen := map[string]bool{}
	out := candidates[:0]
	for _, candidate := range candidates {
		key := strings.ToUpper(candidate)
		if candidate != "" && !seen[key] {
			seen[key] = true
			out = append(out, candidate)
		}
	}
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

func CachePath(dir string, req Request) string {
	return filepath.Join(dir, cacheKey(req)+".json")
}

func requestSnapshot(req Request) RequestSnapshot {
	return RequestSnapshot{
		Ticker:           req.Ticker,
		ISIN:             req.ISIN,
		Name:             req.Name,
		CandidateSymbols: CandidateSymbols(req.Ticker),
	}
}

func (p CacheProvider) resultFromEntry(req Request, path string, entry cacheEntry) Result {
	result := Result{
		Provider:    firstNonEmpty(entry.Provider, providerName(p.Inner), "cache"),
		Request:     firstNonEmptyRequest(entry.Request, requestSnapshot(req)),
		Profile:     entry.Profile,
		Candidates:  entry.Candidates,
		Status:      firstNonEmpty(entry.Status, StatusHit),
		Error:       entry.Error,
		RetrievedAt: firstNonEmpty(entry.RetrievedAt, entry.Profile.RetrievedAt),
		CacheStatus: CacheStatusHit,
		CachePath:   path,
	}
	if result.Error != "" && entry.Status == "" {
		result.Status = StatusFailure
	}
	if result.Profile.RetrievedAt == "" {
		result.Profile.RetrievedAt = result.RetrievedAt
	}
	if result.Profile.Source == "" {
		result.Profile.Source = result.Provider
	}
	result.AttemptedSymbols = attemptedSymbols(result)
	result.Stale = p.isStale(result.RetrievedAt)
	return result
}

func (p CacheProvider) withDefaults(req Request, result Result, err error, path string) Result {
	result.Provider = firstNonEmpty(result.Provider, providerName(p.Inner), "cache")
	result.Request = firstNonEmptyRequest(result.Request, requestSnapshot(req))
	result.CachePath = path
	now := p.now().UTC().Format(time.RFC3339)
	if result.RetrievedAt == "" {
		result.RetrievedAt = firstNonEmpty(result.Profile.RetrievedAt, now)
	}
	if result.Profile.RetrievedAt == "" {
		result.Profile.RetrievedAt = result.RetrievedAt
	}
	if result.Profile.Source == "" && result.Status == StatusHit {
		result.Profile.Source = result.Provider
	}
	if err != nil && result.Error == "" {
		result.Error = err.Error()
	}
	if result.Status == "" {
		if err != nil {
			result.Status = statusForError(err)
		} else {
			result.Status = StatusHit
		}
	}
	if result.AttemptedSymbols == nil {
		result.AttemptedSymbols = attemptedSymbols(result)
	}
	return result
}

func (p CacheProvider) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
}

func (p CacheProvider) ttl() time.Duration {
	if p.TTL < 0 {
		return 0
	}
	if p.TTL == 0 {
		return DefaultCacheTTL
	}
	return p.TTL
}

func (p CacheProvider) isStale(retrievedAt string) bool {
	ttl := p.ttl()
	if ttl == 0 || retrievedAt == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339, retrievedAt)
	if err != nil {
		return false
	}
	return p.now().Sub(parsed.UTC()) > ttl
}

func cacheMissResult(req Request, path string) Result {
	return Result{
		Provider:         "cache",
		Request:          requestSnapshot(req),
		Status:           StatusCacheMiss,
		Error:            ErrCacheMiss.Error(),
		AttemptedSymbols: CandidateSymbols(req.Ticker),
		CacheStatus:      CacheStatusMiss,
		CachePath:        path,
	}
}

func failureResult(req Request, path string, message string) Result {
	return Result{
		Provider:         "cache",
		Request:          requestSnapshot(req),
		Status:           StatusFailure,
		Error:            message,
		AttemptedSymbols: CandidateSymbols(req.Ticker),
		CachePath:        path,
	}
}

func FailureFromResult(req Request, result Result, err error) Failure {
	message := result.Error
	if message == "" && err != nil {
		message = err.Error()
	}
	status := result.Status
	if status == "" {
		status = statusForError(err)
	}
	return Failure{
		Ticker:           req.Ticker,
		ISIN:             req.ISIN,
		Name:             req.Name,
		Provider:         firstNonEmpty(result.Provider, "cache"),
		AttemptedSymbols: strings.Join(attemptedSymbols(result), ";"),
		Status:           status,
		Error:            message,
		NextAction:       NextAction(status, message),
	}
}

func NextAction(status string, message string) string {
	switch status {
	case StatusCacheMiss:
		return "run with STATOS_ENRICHMENT_PROVIDER=yahoo to populate cache or add manual ticker override"
	case StatusAmbiguous:
		return "review candidate symbols and add yahoo_symbol or manual enrichment fields to data/manual/ticker_overrides.csv"
	case StatusUnknownSchema:
		return "delete or refresh the stale cache file so it can be rewritten with the current schema"
	case StatusFailure:
		if strings.Contains(strings.ToLower(message), "cache miss") {
			return "populate enrichment cache or add manual ticker override"
		}
		return "retry provider later or add manual ticker override"
	default:
		return "review enrichment diagnostics"
	}
}

func attemptedSymbols(result Result) []string {
	var out []string
	out = appendUniqueStrings(out, result.AttemptedSymbols...)
	out = appendUniqueStrings(out, result.Request.CandidateSymbols...)
	for _, candidate := range result.Candidates {
		out = appendUniqueStrings(out, candidate.Symbol)
	}
	if result.Profile.Symbol != "" {
		out = appendUniqueStrings(out, result.Profile.Symbol)
	}
	return out
}

func providerName(provider Provider) string {
	if provider == nil {
		return ""
	}
	if named, ok := provider.(NamedProvider); ok {
		return named.Name()
	}
	name := fmt.Sprintf("%T", provider)
	name = strings.TrimPrefix(name, "*")
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	return strings.ToLower(strings.TrimSuffix(name, "Provider"))
}

func statusForError(err error) string {
	switch {
	case err == nil:
		return StatusHit
	case errors.Is(err, ErrCacheMiss):
		return StatusCacheMiss
	case errors.Is(err, ErrAmbiguousMatch):
		return StatusAmbiguous
	case errors.Is(err, ErrUnknownCacheSchema):
		return StatusUnknownSchema
	default:
		return StatusFailure
	}
}

func plausibleCandidates(candidates []Candidate) []Candidate {
	var out []Candidate
	for _, candidate := range candidates {
		typ := strings.ToUpper(strings.TrimSpace(candidate.QuoteType))
		if candidate.Symbol == "" {
			continue
		}
		if typ == "" || typ == "EQUITY" || typ == "ETF" || typ == "MUTUALFUND" {
			out = append(out, candidate)
		}
	}
	return out
}

func ambiguousError(req Request, source string, candidates []Candidate) error {
	var symbols []string
	for _, candidate := range candidates {
		symbols = append(symbols, candidate.Symbol)
	}
	sort.Strings(symbols)
	return fmt.Errorf("%w: %s %s matched multiple yahoo candidates: %s", ErrAmbiguousMatch, req.Ticker, source, strings.Join(symbols, ";"))
}

func appendCandidates(existing []Candidate, additions ...Candidate) []Candidate {
	seen := map[string]bool{}
	for _, candidate := range existing {
		seen[strings.ToUpper(candidate.Symbol)+"|"+candidate.Source] = true
	}
	for _, candidate := range additions {
		key := strings.ToUpper(candidate.Symbol) + "|" + candidate.Source
		if candidate.Symbol != "" && !seen[key] {
			existing = append(existing, candidate)
			seen[key] = true
		}
	}
	return existing
}

func appendUniqueStrings(existing []string, additions ...string) []string {
	seen := map[string]bool{}
	for _, value := range existing {
		seen[strings.ToUpper(value)] = true
	}
	for _, value := range additions {
		value = strings.TrimSpace(value)
		key := strings.ToUpper(value)
		if value != "" && !seen[key] {
			existing = append(existing, value)
			seen[key] = true
		}
	}
	return existing
}

func firstNonEmptyRequest(values ...RequestSnapshot) RequestSnapshot {
	for _, value := range values {
		if value.Ticker != "" || value.ISIN != "" || value.Name != "" || len(value.CandidateSymbols) > 0 {
			return value
		}
	}
	return RequestSnapshot{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
