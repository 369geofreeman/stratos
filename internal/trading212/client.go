package trading212

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	var out []Instrument
	if err := c.getJSON(ctx, "/equity/metadata/instruments", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetExchanges(ctx context.Context) ([]Exchange, error) {
	var out []Exchange
	if err := c.getJSON(ctx, "/equity/metadata/exchanges", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetAccountSummary(ctx context.Context) (AccountSummary, error) {
	var out AccountSummary
	if err := c.getJSON(ctx, "/equity/account/summary", &out); err != nil {
		return AccountSummary{}, err
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	if c.APIKey == "" || c.APISecret == "" {
		return fmt.Errorf("trading212 credentials are not configured")
	}
	url := strings.TrimRight(c.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.APIKey, c.APISecret)
	req.Header.Set("Accept", "application/json")

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("trading212 %s returned %s", path, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode trading212 %s: %w", path, err)
	}
	return nil
}
