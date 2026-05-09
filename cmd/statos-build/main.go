package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"statos/internal/catalogue"
	"statos/internal/enrichment"
	siteexport "statos/internal/export"
	"statos/internal/taxonomy"
	"statos/internal/trading212"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if err := loadDotEnv(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	command := "refresh"
	if len(args) > 0 {
		command = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet(command, flag.ExitOnError)
	forceSample := fs.Bool("sample", false, "use embedded sample data")
	manualDir := fs.String("manual-dir", "data/manual", "manual taxonomy directory")
	siteDataDir := fs.String("site-data-dir", "site/data", "generated static data directory")
	rawDir := fs.String("raw-dir", "data/raw/trading212", "raw Trading 212 snapshot directory")
	cacheDir := fs.String("cache-dir", "data/cache/enrichment", "enrichment cache directory")
	providerName := fs.String("provider", getenvDefault("STATOS_ENRICHMENT_PROVIDER", "cache"), "enrichment provider: cache or yahoo")
	if err := fs.Parse(args); err != nil {
		return err
	}

	switch command {
	case "refresh", "sample":
	default:
		return fmt.Errorf("unknown command %q; use refresh or sample", command)
	}
	if command == "sample" {
		*forceSample = true
	}

	builtAt := time.Now().UTC()
	if command == "sample" {
		builtAt = time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	}

	manual, err := taxonomy.Load(*manualDir)
	if err != nil {
		return err
	}
	if err := taxonomy.Validate(manual); err != nil {
		return err
	}

	ctx := context.Background()
	instruments, exchanges, profiles, envName, rawSnapshotAt, err := loadSourceData(ctx, *forceSample, builtAt, *rawDir, *providerName, *cacheDir)
	if err != nil {
		return err
	}

	enrichmentAttempted := len(instruments)
	enrichmentSucceeded := len(profiles)
	enrichmentFailed := enrichmentAttempted - enrichmentSucceeded
	if enrichmentFailed < 0 {
		enrichmentFailed = 0
	}

	cat, err := catalogue.Build(catalogue.BuildInput{
		Instruments:           instruments,
		Exchanges:             exchanges,
		Profiles:              profiles,
		Manual:                manual,
		BuiltAt:               builtAt,
		Trading212Environment: envName,
		RawSnapshotAt:         rawSnapshotAt,
		EnrichmentAttempted:   enrichmentAttempted,
		EnrichmentSucceeded:   enrichmentSucceeded,
		EnrichmentFailed:      enrichmentFailed,
	})
	if err != nil {
		return err
	}
	if err := siteexport.WriteSiteData(*siteDataDir, cat); err != nil {
		return err
	}
	log.Printf("wrote %s: %d tickers, %d companies, %d unclassified", *siteDataDir, len(cat.Tickers), len(cat.Companies), len(cat.Unclassified))
	return nil
}

func loadSourceData(ctx context.Context, forceSample bool, builtAt time.Time, rawDir, providerName, cacheDir string) ([]trading212.Instrument, []trading212.Exchange, map[string]enrichment.Profile, string, string, error) {
	apiKey := os.Getenv("TRADING212_API_KEY")
	apiSecret := os.Getenv("TRADING212_API_SECRET")
	envName := getenvDefault("STATOS_TRADING212_ENV", "demo")
	if os.Getenv("STATOS_SAMPLE") == "1" {
		forceSample = true
	}
	if forceSample || apiKey == "" || apiSecret == "" {
		instruments, exchanges, profiles := catalogue.SampleData()
		rawSnapshotAt := builtAt.Format(time.RFC3339)
		if err := writeRawSnapshots(rawDir, builtAt, instruments, exchanges); err != nil {
			return nil, nil, nil, "", "", err
		}
		return instruments, exchanges, profiles, "sample", rawSnapshotAt, nil
	}

	baseURL := os.Getenv("STATOS_TRADING212_BASE_URL")
	if baseURL == "" {
		baseURL = trading212.BaseURLForEnvironment(envName)
	}
	client := trading212.NewClient(baseURL, apiKey, apiSecret)
	exchanges, err := client.GetExchanges(ctx)
	if err != nil {
		return nil, nil, nil, "", "", err
	}
	instruments, err := client.GetInstruments(ctx)
	if err != nil {
		return nil, nil, nil, "", "", err
	}
	rawSnapshotAt := builtAt.Format(time.RFC3339)
	if err := writeRawSnapshots(rawDir, builtAt, instruments, exchanges); err != nil {
		return nil, nil, nil, "", "", err
	}
	profiles := enrichAll(ctx, instruments, providerName, cacheDir)
	return instruments, exchanges, profiles, envName, rawSnapshotAt, nil
}

func enrichAll(ctx context.Context, instruments []trading212.Instrument, providerName, cacheDir string) map[string]enrichment.Profile {
	var inner enrichment.Provider
	if strings.EqualFold(providerName, "yahoo") {
		inner = enrichment.YahooProvider{}
	}
	provider := enrichment.CacheProvider{Dir: cacheDir, Inner: inner}
	out := map[string]enrichment.Profile{}
	for _, instrument := range instruments {
		parts := catalogue.ParseBrokerTicker(instrument.Ticker)
		profile, err := provider.Lookup(ctx, enrichment.Request{
			Ticker:       instrument.Ticker,
			ISIN:         instrument.ISIN,
			Name:         instrument.Name,
			CurrencyCode: instrument.CurrencyCode,
			ExchangeCode: parts.ExchangeCode,
		})
		if err != nil {
			continue
		}
		out[instrument.Ticker] = profile
	}
	return out
}

func writeRawSnapshots(dir string, builtAt time.Time, instruments []trading212.Instrument, exchanges []trading212.Exchange) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	stamp := builtAt.Format("20060102T150405Z")
	if err := writeJSON(filepath.Join(dir, "instruments_"+stamp+".json"), instruments); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "exchanges_"+stamp+".json"), exchanges); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "instruments_latest.json"), instruments); err != nil {
		return err
	}
	return writeJSON(filepath.Join(dir, "exchanges_latest.json"), exchanges)
}

func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func loadDotEnv(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key != "" && os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func getenvDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
