package enrichment

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCandidateSymbols(t *testing.T) {
	got := CandidateSymbols("VOD_L_EQ")
	want := []string{"VOD.L", "VOD", "VOD_L_EQ"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CandidateSymbols() = %#v, want %#v", got, want)
	}
}

func TestCacheProviderHitReturningProfile(t *testing.T) {
	dir := t.TempDir()
	req := Request{Ticker: "VOD_L_EQ", ISIN: "GB00BH4HKS39", Name: "Vodafone Group plc"}
	writeTestCacheEntry(t, dir, req, cacheEntry{
		SchemaVersion: CacheSchemaVersion,
		Provider:      "yahoo",
		Request:       requestSnapshot(req),
		Profile:       Profile{Symbol: "VOD.L", Sector: "Communication Services", Source: "yahoo"},
		Status:        StatusHit,
		RetrievedAt:   "2026-05-09T12:00:00Z",
	})

	result, err := (CacheProvider{Dir: dir}).Lookup(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Profile.Symbol != "VOD.L" || result.Profile.Sector != "Communication Services" {
		t.Fatalf("profile = %#v", result.Profile)
	}
	if result.CacheStatus != CacheStatusHit || result.Status != StatusHit {
		t.Fatalf("result status = %#v", result)
	}
}

func TestCacheProviderMissCacheOnly(t *testing.T) {
	req := Request{Ticker: "MISS_US_EQ", ISIN: "US0000000001", Name: "Missing Corp"}
	result, err := (CacheProvider{Dir: t.TempDir()}).Lookup(context.Background(), req)
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("err = %v, want ErrCacheMiss", err)
	}
	if result.Status != StatusCacheMiss || result.CacheStatus != CacheStatusMiss {
		t.Fatalf("result = %#v", result)
	}
}

func TestCacheProviderWritesSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	req := Request{Ticker: "ABC_US_EQ", ISIN: "US0000000001", Name: "ABC Corp"}
	inner := &stubProvider{result: Result{Status: StatusHit, Profile: Profile{Symbol: "ABC", Sector: "Technology"}}}
	_, err := (CacheProvider{
		Dir:   dir,
		Inner: inner,
		Now:   func() time.Time { return time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC) },
	}).Lookup(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(CachePath(dir, req))
	if err != nil {
		t.Fatal(err)
	}
	var entry cacheEntry
	if err := json.Unmarshal(b, &entry); err != nil {
		t.Fatal(err)
	}
	if entry.SchemaVersion != CacheSchemaVersion || entry.Status != StatusHit || entry.Provider != "stub" {
		t.Fatalf("entry = %#v", entry)
	}
}

func TestCacheProviderUnknownSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	req := Request{Ticker: "OLD_US_EQ", ISIN: "US0000000002", Name: "Old Cache Corp"}
	writeTestCacheEntry(t, dir, req, cacheEntry{SchemaVersion: 999, Provider: "yahoo", Status: StatusHit})

	result, err := (CacheProvider{Dir: dir}).Lookup(context.Background(), req)
	if !errors.Is(err, ErrUnknownCacheSchema) {
		t.Fatalf("err = %v, want ErrUnknownCacheSchema", err)
	}
	if result.Status != StatusUnknownSchema || result.CacheStatus != CacheStatusUnknownSchema {
		t.Fatalf("result = %#v", result)
	}
}

func TestCacheProviderStaleCacheDetectionDoesNotFetch(t *testing.T) {
	dir := t.TempDir()
	req := Request{Ticker: "STALE_US_EQ", ISIN: "US0000000003", Name: "Stale Corp"}
	writeTestCacheEntry(t, dir, req, cacheEntry{
		SchemaVersion: CacheSchemaVersion,
		Provider:      "yahoo",
		Request:       requestSnapshot(req),
		Profile:       Profile{Symbol: "STALE", Sector: "Industrials"},
		Status:        StatusHit,
		RetrievedAt:   "2026-05-01T12:00:00Z",
	})
	inner := &stubProvider{err: errors.New("inner should not be called")}
	result, err := (CacheProvider{
		Dir:   dir,
		Inner: inner,
		TTL:   24 * time.Hour,
		Now:   func() time.Time { return time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC) },
	}).Lookup(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Stale || inner.calls != 0 {
		t.Fatalf("stale=%v inner calls=%d", result.Stale, inner.calls)
	}
}

func TestCacheProviderCachedFailureReplay(t *testing.T) {
	dir := t.TempDir()
	req := Request{Ticker: "FAIL_US_EQ", ISIN: "US0000000004", Name: "Failure Corp"}
	writeTestCacheEntry(t, dir, req, cacheEntry{
		SchemaVersion: CacheSchemaVersion,
		Provider:      "yahoo",
		Request:       requestSnapshot(req),
		Status:        StatusFailure,
		Error:         "yahoo returned 502",
		RetrievedAt:   "2026-05-09T12:00:00Z",
	})

	result, err := (CacheProvider{Dir: dir}).Lookup(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "yahoo returned 502") {
		t.Fatalf("err = %v", err)
	}
	if result.Status != StatusFailure || result.Error != "yahoo returned 502" {
		t.Fatalf("result = %#v", result)
	}
}

func TestCacheProviderProviderFailureCached(t *testing.T) {
	dir := t.TempDir()
	req := Request{Ticker: "DOWN_US_EQ", ISIN: "US0000000005", Name: "Provider Down Corp"}
	inner := &stubProvider{
		result: Result{Status: StatusFailure, Error: "provider unavailable"},
		err:    errors.New("provider unavailable"),
	}
	result, err := (CacheProvider{Dir: dir, Inner: inner}).Lookup(context.Background(), req)
	if err == nil || result.Status != StatusFailure {
		t.Fatalf("result=%#v err=%v", result, err)
	}

	b, err := os.ReadFile(CachePath(dir, req))
	if err != nil {
		t.Fatal(err)
	}
	var entry cacheEntry
	if err := json.Unmarshal(b, &entry); err != nil {
		t.Fatal(err)
	}
	if entry.SchemaVersion != CacheSchemaVersion || entry.Status != StatusFailure || entry.Error != "provider unavailable" {
		t.Fatalf("entry = %#v", entry)
	}
}

func TestYahooProviderAmbiguousMatch(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/quoteSummary/") {
			t.Fatalf("quoteSummary should not be called for ambiguous search")
		}
		return jsonHTTPResponse(t, map[string]any{
			"quotes": []map[string]any{
				{"symbol": "ABC", "longname": "ABC plc", "quoteType": "EQUITY"},
				{"symbol": "ABC.L", "longname": "ABC plc London", "quoteType": "EQUITY"},
			},
		}), nil
	})}

	result, err := (YahooProvider{BaseURL: "https://yahoo.test", HTTPClient: client}).Lookup(context.Background(), Request{
		Ticker: "ABC_L_EQ",
		ISIN:   "GB0000000001",
		Name:   "ABC plc",
	})
	if !errors.Is(err, ErrAmbiguousMatch) {
		t.Fatalf("err = %v, want ErrAmbiguousMatch", err)
	}
	if result.Status != StatusAmbiguous || len(result.Candidates) != 2 || result.Profile.Symbol != "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestYahooProviderISINFirstLookupOrdering(t *testing.T) {
	var calls []string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.URL.String())
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/finance/search"):
			if r.URL.Query().Get("q") != "GB00BH4HKS39" {
				t.Fatalf("first search query = %q, want ISIN", r.URL.Query().Get("q"))
			}
			return jsonHTTPResponse(t, map[string]any{
				"quotes": []map[string]any{
					{"symbol": "VOD.L", "longname": "Vodafone Group plc", "quoteType": "EQUITY", "exchDisp": "London", "currency": "GBp"},
				},
			}), nil
		case strings.HasPrefix(r.URL.Path, "/v10/finance/quoteSummary/VOD.L"):
			return jsonHTTPResponse(t, map[string]any{
				"quoteSummary": map[string]any{
					"result": []map[string]any{{
						"price": map[string]any{
							"symbol":       "VOD.L",
							"longName":     "Vodafone Group plc",
							"exchangeName": "LSE",
							"currency":     "GBp",
							"marketCap":    map[string]any{"raw": 123},
						},
						"summaryProfile": map[string]any{
							"sector":   "Communication Services",
							"industry": "Telecom Services",
							"country":  "United Kingdom",
						},
					}},
					"error": nil,
				},
			}), nil
		default:
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
		return nil, errors.New("unreachable")
	})}

	result, err := (YahooProvider{BaseURL: "https://yahoo.test", HTTPClient: client}).Lookup(context.Background(), Request{
		Ticker: "VOD_L_EQ",
		ISIN:   "GB00BH4HKS39",
		Name:   "Vodafone Group plc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Profile.Symbol != "VOD.L" || result.Profile.MarketCap != 123 {
		t.Fatalf("profile = %#v", result.Profile)
	}
	if len(calls) != 2 || !strings.Contains(calls[0], "/v1/finance/search") || !strings.Contains(calls[0], "q=GB00BH4HKS39") {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestYahooProviderSymbolFallbackPrefersExchangeSuffix(t *testing.T) {
	var quoteSummaryCalls []string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/finance/search"):
			if r.URL.Query().Get("q") != "GB00BH4HKS39" {
				t.Fatalf("search query = %q, want ISIN only before symbol fallback", r.URL.Query().Get("q"))
			}
			return jsonHTTPResponse(t, map[string]any{"quotes": []map[string]any{}}), nil
		case strings.HasPrefix(r.URL.Path, "/v10/finance/quoteSummary/"):
			quoteSummaryCalls = append(quoteSummaryCalls, r.URL.Path)
			if strings.HasPrefix(r.URL.Path, "/v10/finance/quoteSummary/VOD") && !strings.HasPrefix(r.URL.Path, "/v10/finance/quoteSummary/VOD.L") {
				t.Fatalf("base symbol was attempted before exchange-suffixed symbol: calls=%#v", quoteSummaryCalls)
			}
			return jsonHTTPResponse(t, map[string]any{
				"quoteSummary": map[string]any{
					"result": []map[string]any{{
						"price": map[string]any{
							"symbol":       "VOD.L",
							"longName":     "Vodafone Group plc",
							"exchangeName": "LSE",
							"currency":     "GBp",
						},
						"summaryProfile": map[string]any{
							"sector":   "Communication Services",
							"industry": "Telecom Services",
							"country":  "United Kingdom",
						},
					}},
					"error": nil,
				},
			}), nil
		default:
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
		return nil, errors.New("unreachable")
	})}

	result, err := (YahooProvider{BaseURL: "https://yahoo.test", HTTPClient: client}).Lookup(context.Background(), Request{
		Ticker: "VOD_L_EQ",
		ISIN:   "GB00BH4HKS39",
		Name:   "Vodafone Group plc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Profile.Symbol != "VOD.L" {
		t.Fatalf("profile = %#v", result.Profile)
	}
	if len(quoteSummaryCalls) != 1 || !strings.HasPrefix(quoteSummaryCalls[0], "/v10/finance/quoteSummary/VOD.L") {
		t.Fatalf("quoteSummary calls = %#v", quoteSummaryCalls)
	}
}

type stubProvider struct {
	result Result
	err    error
	calls  int
}

func (p *stubProvider) Name() string {
	return "stub"
}

func (p *stubProvider) Lookup(context.Context, Request) (Result, error) {
	p.calls++
	return p.result, p.err
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func writeTestCacheEntry(t *testing.T, dir string, req Request, entry cacheEntry) {
	t.Helper()
	b, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(CachePath(dir, req), append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func jsonHTTPResponse(t *testing.T, value any) *http.Response {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}
