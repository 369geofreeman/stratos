package trading212

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestClientDecodesTrading212Fixtures(t *testing.T) {
	client := NewClient("https://example.test", "api-key", "api-secret")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "api-key" || pass != "api-secret" {
			t.Fatalf("request did not use expected HTTP Basic auth")
		}
		header := http.Header{}
		header.Set("Content-Type", "application/json")
		header.Set("x-ratelimit-limit", "1")
		header.Set("x-ratelimit-period", "50")
		header.Set("x-ratelimit-remaining", "0")
		header.Set("x-ratelimit-reset", "1760346100")
		header.Set("x-ratelimit-used", "1")
		switch r.URL.Path {
		case "/equity/metadata/instruments":
			b, err := os.ReadFile("testdata/instruments.json")
			if err != nil {
				t.Fatal(err)
			}
			return testResponse(http.StatusOK, header, string(b)), nil
		case "/equity/metadata/exchanges":
			b, err := os.ReadFile("testdata/exchanges.json")
			if err != nil {
				t.Fatal(err)
			}
			return testResponse(http.StatusOK, header, string(b)), nil
		default:
			return testResponse(http.StatusNotFound, header, ""), nil
		}
	})}

	instruments, instrumentDiag, err := client.GetInstrumentsWithDiagnostics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(instruments) != 2 {
		t.Fatalf("instruments len = %d, want 2", len(instruments))
	}
	if got := instruments[0].Ticker; got != "AAPL_US_EQ" {
		t.Fatalf("first ticker = %q, want AAPL_US_EQ", got)
	}
	if got := instruments[0].WorkingScheduleID; got != 1 {
		t.Fatalf("workingScheduleId = %d, want 1", got)
	}
	if instrumentDiag.StatusCode != http.StatusOK || instrumentDiag.RateLimit.ResetAt == "" {
		t.Fatalf("instrument diagnostics missing status/rate limit: %#v", instrumentDiag)
	}

	exchanges, exchangeDiag, err := client.GetExchangesWithDiagnostics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(exchanges) != 2 {
		t.Fatalf("exchanges len = %d, want 2", len(exchanges))
	}
	if got := exchanges[0].WorkingSchedules[0].ID; got != 101 {
		t.Fatalf("working schedule id = %d, want 101", got)
	}
	if len(exchanges[0].WorkingSchedules[0].Events) == 0 {
		t.Fatalf("working schedule events were not preserved")
	}
	if exchangeDiag.EndpointName != "exchanges" || exchangeDiag.RequestStartedAt == "" || exchangeDiag.ResponseReceivedAt == "" {
		t.Fatalf("exchange diagnostics missing timing/endpoint: %#v", exchangeDiag)
	}
}

func TestClientFriendlyHTTPErrors(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		category string
		wants    []string
	}{
		{
			name:     "unauthorized",
			status:   http.StatusUnauthorized,
			category: "unauthorized",
			wants:    []string{"401 Unauthorized", "TRADING212_API_KEY", "TRADING212_API_SECRET"},
		},
		{
			name:     "forbidden",
			status:   http.StatusForbidden,
			category: "forbidden",
			wants:    []string{"403 Forbidden", "Invest/Stocks ISA", "IP restrictions"},
		},
		{
			name:     "timeout",
			status:   http.StatusRequestTimeout,
			category: "request_timeout",
			wants:    []string{"408 Request Timeout", "retry later", "partial output"},
		},
		{
			name:     "rate limited",
			status:   http.StatusTooManyRequests,
			category: "rate_limited",
			wants:    []string{"429 Too Many Requests", "x-ratelimit-reset=1760346100", "2025-10-13T09:01:40Z"},
		},
		{
			name:     "unexpected",
			status:   http.StatusInternalServerError,
			category: "unexpected_status",
			wants:    []string{"unexpected HTTP status", "500"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient("https://example.test", "api-key", "api-secret")
			client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				header := http.Header{}
				header.Set("x-ratelimit-reset", "1760346100")
				return testResponse(tt.status, header, ""), nil
			})}
			_, diag, err := client.GetInstrumentsWithDiagnostics(context.Background())
			if err == nil {
				t.Fatal("expected error")
			}
			for _, want := range tt.wants {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q did not contain %q", err.Error(), want)
				}
			}
			if diag.StatusCode != tt.status || diag.ErrorCategory != tt.category {
				t.Fatalf("diagnostic = %#v, want status %d category %s", diag, tt.status, tt.category)
			}
		})
	}
}

func TestClientReportsMalformedResponses(t *testing.T) {
	client := NewClient("https://example.test", "api-key", "api-secret")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		header := http.Header{}
		header.Set("Content-Type", "application/json")
		return testResponse(http.StatusOK, header, `[`), nil
	})}
	_, diag, err := client.GetInstrumentsWithDiagnostics(context.Background())
	if err == nil {
		t.Fatal("expected decode error")
	}
	if diag.StatusCode != http.StatusOK || diag.ErrorCategory != "decode_failed" {
		t.Fatalf("diagnostic = %#v, want decode failure after 200 response", diag)
	}
	if !strings.Contains(err.Error(), "decode trading212 instruments response") {
		t.Fatalf("unexpected decode error: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func testResponse(statusCode int, header http.Header, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
