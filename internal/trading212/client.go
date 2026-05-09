package trading212

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	DemoBaseURL = "https://demo.trading212.com/api/v0"
	LiveBaseURL = "https://live.trading212.com/api/v0"
)

type Instrument struct {
	AddedOn           string  `json:"addedOn,omitempty"`
	CurrencyCode      string  `json:"currencyCode,omitempty"`
	ExtendedHours     bool    `json:"extendedHours,omitempty"`
	ISIN              string  `json:"isin,omitempty"`
	MaxOpenQuantity   float64 `json:"maxOpenQuantity,omitempty"`
	Name              string  `json:"name,omitempty"`
	ShortName         string  `json:"shortName,omitempty"`
	Ticker            string  `json:"ticker,omitempty"`
	Type              string  `json:"type,omitempty"`
	WorkingScheduleID int64   `json:"workingScheduleId,omitempty"`
}

type Exchange struct {
	ID               int64             `json:"id"`
	Name             string            `json:"name"`
	WorkingSchedules []WorkingSchedule `json:"workingSchedules,omitempty"`
}

type WorkingSchedule struct {
	ID     int64           `json:"id,omitempty"`
	Name   string          `json:"name,omitempty"`
	Events json.RawMessage `json:"events,omitempty"`
}

type AccountSummary struct {
	ID       string `json:"id,omitempty"`
	Currency string `json:"currencyCode,omitempty"`
}

type RateLimitHeaders struct {
	Limit     string `json:"limit,omitempty"`
	Period    string `json:"period,omitempty"`
	Remaining string `json:"remaining,omitempty"`
	Reset     string `json:"reset,omitempty"`
	ResetAt   string `json:"resetAt,omitempty"`
	Used      string `json:"used,omitempty"`
}

type EndpointDiagnostic struct {
	EndpointName       string           `json:"endpointName"`
	Path               string           `json:"path"`
	RequestStartedAt   string           `json:"requestStartedAt,omitempty"`
	ResponseReceivedAt string           `json:"responseReceivedAt,omitempty"`
	DurationMillis     int64            `json:"durationMillis,omitempty"`
	StatusCode         int              `json:"statusCode,omitempty"`
	Status             string           `json:"status,omitempty"`
	ErrorCategory      string           `json:"errorCategory,omitempty"`
	ErrorMessage       string           `json:"errorMessage,omitempty"`
	RateLimit          RateLimitHeaders `json:"rateLimit,omitempty"`
}

type RateLimitObservation struct {
	EndpointName string `json:"endpointName"`
	Path         string `json:"path"`
	Limit        string `json:"limit,omitempty"`
	Period       string `json:"period,omitempty"`
	Remaining    string `json:"remaining,omitempty"`
	Reset        string `json:"reset,omitempty"`
	ResetAt      string `json:"resetAt,omitempty"`
	Used         string `json:"used,omitempty"`
}

type APIError struct {
	Diagnostic EndpointDiagnostic
}

func (e APIError) Error() string {
	endpoint := e.Diagnostic.EndpointName
	if endpoint == "" {
		endpoint = e.Diagnostic.Path
	}
	status := e.Diagnostic.Status
	if status == "" && e.Diagnostic.StatusCode != 0 {
		status = http.StatusText(e.Diagnostic.StatusCode)
	}
	switch e.Diagnostic.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Sprintf("trading212 %s request returned 401 Unauthorized: check TRADING212_API_KEY and TRADING212_API_SECRET for the selected Trading 212 environment", endpoint)
	case http.StatusForbidden:
		return fmt.Sprintf("trading212 %s request returned 403 Forbidden: the API key may not have Invest/Stocks ISA access, may be blocked by IP restrictions, or may belong to a different Trading 212 environment", endpoint)
	case http.StatusRequestTimeout:
		return fmt.Sprintf("trading212 %s request returned 408 Request Timeout: Trading 212 did not complete the metadata request; retry later without using partial output", endpoint)
	case http.StatusTooManyRequests:
		reset := e.Diagnostic.RateLimit.Reset
		if reset == "" {
			return fmt.Sprintf("trading212 %s request returned 429 Too Many Requests: wait for the Trading 212 rate limit to reset before retrying", endpoint)
		}
		if e.Diagnostic.RateLimit.ResetAt != "" {
			return fmt.Sprintf("trading212 %s request returned 429 Too Many Requests: wait until x-ratelimit-reset=%s (%s) before retrying", endpoint, reset, e.Diagnostic.RateLimit.ResetAt)
		}
		return fmt.Sprintf("trading212 %s request returned 429 Too Many Requests: wait until x-ratelimit-reset=%s before retrying", endpoint, reset)
	default:
		return fmt.Sprintf("trading212 %s request returned unexpected HTTP status %d %s", endpoint, e.Diagnostic.StatusCode, status)
	}
}

type Client struct {
	BaseURL    string
	APIKey     string
	APISecret  string
	HTTPClient *http.Client
}

func NewClient(baseURL, apiKey, apiSecret string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = DemoBaseURL
	}
	return &Client{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		APISecret:  apiSecret,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func BaseURLForEnvironment(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "live":
		return LiveBaseURL
	default:
		return DemoBaseURL
	}
}

func (c *Client) GetInstruments(ctx context.Context) ([]Instrument, error) {
	out, _, err := c.GetInstrumentsWithDiagnostics(ctx)
	return out, err
}

func (c *Client) GetInstrumentsWithDiagnostics(ctx context.Context) ([]Instrument, EndpointDiagnostic, error) {
	var out []Instrument
	diag, err := c.getJSON(ctx, "instruments", "/equity/metadata/instruments", &out)
	if err != nil {
		return nil, diag, err
	}
	return out, diag, nil
}

func (c *Client) GetExchanges(ctx context.Context) ([]Exchange, error) {
	out, _, err := c.GetExchangesWithDiagnostics(ctx)
	return out, err
}

func (c *Client) GetExchangesWithDiagnostics(ctx context.Context) ([]Exchange, EndpointDiagnostic, error) {
	var out []Exchange
	diag, err := c.getJSON(ctx, "exchanges", "/equity/metadata/exchanges", &out)
	if err != nil {
		return nil, diag, err
	}
	return out, diag, nil
}

func (c *Client) GetAccountSummary(ctx context.Context) (AccountSummary, error) {
	var out AccountSummary
	if _, err := c.getJSON(ctx, "account_summary", "/equity/account/summary", &out); err != nil {
		return AccountSummary{}, err
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, endpointName, path string, out any) (EndpointDiagnostic, error) {
	diag := EndpointDiagnostic{
		EndpointName: endpointName,
		Path:         path,
	}
	if c.APIKey == "" || c.APISecret == "" {
		diag.ErrorCategory = "credentials_missing"
		diag.ErrorMessage = "Trading 212 credentials are not configured"
		return diag, fmt.Errorf("trading212 credentials are not configured; set TRADING212_API_KEY and TRADING212_API_SECRET in .env")
	}
	url := strings.TrimRight(c.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		diag.ErrorCategory = "request_build_failed"
		diag.ErrorMessage = err.Error()
		return diag, err
	}
	req.SetBasicAuth(c.APIKey, c.APISecret)
	req.Header.Set("Accept", "application/json")

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	startedAt := time.Now().UTC()
	diag.RequestStartedAt = startedAt.Format(time.RFC3339Nano)
	resp, err := client.Do(req)
	responseAt := time.Now().UTC()
	diag.ResponseReceivedAt = responseAt.Format(time.RFC3339Nano)
	diag.DurationMillis = responseAt.Sub(startedAt).Milliseconds()
	if err != nil {
		diag.ErrorCategory = "request_failed"
		diag.ErrorMessage = err.Error()
		return diag, fmt.Errorf("trading212 %s request failed: %w", endpointName, err)
	}
	defer resp.Body.Close()
	diag.StatusCode = resp.StatusCode
	diag.Status = resp.Status
	diag.RateLimit = rateLimitHeaders(resp.Header)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		diag.ErrorCategory = statusCategory(resp.StatusCode)
		apiErr := APIError{Diagnostic: diag}
		diag.ErrorMessage = apiErr.Error()
		return diag, apiErr
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		diag.ErrorCategory = "decode_failed"
		diag.ErrorMessage = err.Error()
		return diag, fmt.Errorf("decode trading212 %s response: %w", endpointName, err)
	}
	return diag, nil
}

func rateLimitHeaders(header http.Header) RateLimitHeaders {
	out := RateLimitHeaders{
		Limit:     header.Get("x-ratelimit-limit"),
		Period:    header.Get("x-ratelimit-period"),
		Remaining: header.Get("x-ratelimit-remaining"),
		Reset:     header.Get("x-ratelimit-reset"),
		Used:      header.Get("x-ratelimit-used"),
	}
	if out.Reset != "" {
		if reset, err := strconv.ParseInt(out.Reset, 10, 64); err == nil {
			out.ResetAt = time.Unix(reset, 0).UTC().Format(time.RFC3339)
		}
	}
	return out
}

func statusCategory(statusCode int) string {
	switch statusCode {
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusRequestTimeout:
		return "request_timeout"
	case http.StatusTooManyRequests:
		return "rate_limited"
	default:
		return "unexpected_status"
	}
}

func RateLimitObservations(diags []EndpointDiagnostic) []RateLimitObservation {
	out := []RateLimitObservation{}
	for _, diag := range diags {
		if diag.RateLimit == (RateLimitHeaders{}) {
			continue
		}
		out = append(out, RateLimitObservation{
			EndpointName: diag.EndpointName,
			Path:         diag.Path,
			Limit:        diag.RateLimit.Limit,
			Period:       diag.RateLimit.Period,
			Remaining:    diag.RateLimit.Remaining,
			Reset:        diag.RateLimit.Reset,
			ResetAt:      diag.RateLimit.ResetAt,
			Used:         diag.RateLimit.Used,
		})
	}
	return out
}
