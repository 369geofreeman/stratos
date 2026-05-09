package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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
	noFetch := fs.Bool("no-fetch", false, "replay from raw Trading 212 snapshots without fetching")
	inputRawDir := fs.String("input-raw-dir", "", "raw Trading 212 snapshot directory to replay; defaults to --raw-dir")
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
	if *noFetch && *forceSample {
		return fmt.Errorf("choose either sample data or --no-fetch raw replay, not both")
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
	source, err := loadSourceData(ctx, sourceOptions{
		ForceSample:  *forceSample,
		NoFetch:      *noFetch,
		BuiltAt:      builtAt,
		RawDir:       *rawDir,
		InputRawDir:  *inputRawDir,
		ProviderName: *providerName,
		CacheDir:     *cacheDir,
	})
	if err != nil {
		return err
	}
	if source.SourceMode == "raw_replay" {
		if parsed, err := time.Parse(time.RFC3339, source.RawSnapshotAt); err == nil {
			builtAt = parsed.UTC()
		} else {
			builtAt = time.Unix(0, 0).UTC()
		}
	}
	source.EnrichmentDiagnostics.FailureCSV = manifestPath(filepath.Join(*siteDataDir, "enrichment_failures.csv"))

	enrichmentAttempted := len(source.Instruments)
	enrichmentSucceeded := len(source.Profiles)
	enrichmentFailed := source.EnrichmentDiagnostics.FailureCount
	if enrichmentFailed == 0 {
		enrichmentFailed = enrichmentAttempted - enrichmentSucceeded
	}
	if enrichmentFailed < 0 {
		enrichmentFailed = 0
	}

	cat, err := catalogue.Build(catalogue.BuildInput{
		Instruments:           source.Instruments,
		Exchanges:             source.Exchanges,
		Profiles:              source.Profiles,
		Manual:                manual,
		BuiltAt:               builtAt,
		SourceMode:            source.SourceMode,
		Trading212Environment: source.Trading212Environment,
		Trading212BaseURL:     source.Trading212BaseURL,
		Trading212FetchAt:     source.Trading212FetchAt,
		RawSnapshotAt:         source.RawSnapshotAt,
		RawSnapshots:          source.RawSnapshots,
		HTTPDiagnostics:       source.HTTPDiagnostics,
		RateLimits:            source.RateLimits,
		EnrichmentAttempted:   enrichmentAttempted,
		EnrichmentSucceeded:   enrichmentSucceeded,
		EnrichmentFailed:      enrichmentFailed,
		EnrichmentDiagnostics: source.EnrichmentDiagnostics,
		EnrichmentFailures:    source.EnrichmentFailures,
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

type sourceOptions struct {
	ForceSample  bool
	NoFetch      bool
	BuiltAt      time.Time
	RawDir       string
	InputRawDir  string
	ProviderName string
	CacheDir     string
}

type sourceData struct {
	Instruments           []trading212.Instrument
	Exchanges             []trading212.Exchange
	Profiles              map[string]enrichment.Profile
	SourceMode            string
	Trading212Environment string
	Trading212BaseURL     string
	Trading212FetchAt     string
	RawSnapshotAt         string
	RawSnapshots          catalogue.RawSnapshotSummary
	HTTPDiagnostics       []trading212.EndpointDiagnostic
	RateLimits            []trading212.RateLimitObservation
	EnrichmentDiagnostics catalogue.EnrichmentDiagnostics
	EnrichmentFailures    []catalogue.EnrichmentFailure
}

func loadSourceData(ctx context.Context, opts sourceOptions) (sourceData, error) {
	apiKey := os.Getenv("TRADING212_API_KEY")
	apiSecret := os.Getenv("TRADING212_API_SECRET")
	envName := getenvDefault("STATOS_TRADING212_ENV", "demo")
	if os.Getenv("STATOS_SAMPLE") == "1" {
		opts.ForceSample = true
	}
	if opts.NoFetch {
		inputDir := opts.InputRawDir
		if inputDir == "" {
			inputDir = opts.RawDir
		}
		instruments, exchanges, rawSnapshots, err := readRawSnapshots(inputDir)
		if err != nil {
			return sourceData{}, err
		}
		replay := replayEnrichment(ctx, instruments, exchanges, opts.CacheDir)
		return sourceData{
			Instruments:           instruments,
			Exchanges:             exchanges,
			Profiles:              replay.Profiles,
			SourceMode:            "raw_replay",
			Trading212Environment: "raw_replay",
			RawSnapshotAt:         rawSnapshots.Timestamp,
			RawSnapshots:          rawSnapshots,
			EnrichmentDiagnostics: replay.Diagnostics,
			EnrichmentFailures:    replay.Failures,
		}, nil
	}

	if opts.ForceSample || apiKey == "" || apiSecret == "" {
		instruments, exchanges, profiles := catalogue.SampleData()
		enrichmentRun := sampleEnrichment(instruments, profiles)
		rawSnapshots, err := writeRawSnapshots(opts.RawDir, opts.BuiltAt, instruments, exchanges)
		if err != nil {
			return sourceData{}, err
		}
		return sourceData{
			Instruments:           instruments,
			Exchanges:             exchanges,
			Profiles:              profiles,
			SourceMode:            "sample",
			Trading212Environment: "sample",
			RawSnapshotAt:         rawSnapshots.Timestamp,
			RawSnapshots:          rawSnapshots,
			EnrichmentDiagnostics: enrichmentRun.Diagnostics,
			EnrichmentFailures:    enrichmentRun.Failures,
		}, nil
	}

	baseURL := os.Getenv("STATOS_TRADING212_BASE_URL")
	if baseURL == "" {
		baseURL = trading212.BaseURLForEnvironment(envName)
	}
	client := trading212.NewClient(baseURL, apiKey, apiSecret)
	fetchAt := time.Now().UTC().Format(time.RFC3339Nano)
	diagnostics := []trading212.EndpointDiagnostic{}
	exchanges, exchangeDiag, err := client.GetExchangesWithDiagnostics(ctx)
	diagnostics = append(diagnostics, exchangeDiag)
	if err != nil {
		return sourceData{}, err
	}
	instruments, instrumentDiag, err := client.GetInstrumentsWithDiagnostics(ctx)
	diagnostics = append(diagnostics, instrumentDiag)
	if err != nil {
		return sourceData{}, err
	}
	rawSnapshots, err := writeRawSnapshots(opts.RawDir, opts.BuiltAt, instruments, exchanges)
	if err != nil {
		return sourceData{}, err
	}
	enrichmentRun := enrichAll(ctx, instruments, opts.ProviderName, opts.CacheDir)
	return sourceData{
		Instruments:           instruments,
		Exchanges:             exchanges,
		Profiles:              enrichmentRun.Profiles,
		SourceMode:            "live_fetch",
		Trading212Environment: envName,
		Trading212BaseURL:     baseURL,
		Trading212FetchAt:     fetchAt,
		RawSnapshotAt:         rawSnapshots.Timestamp,
		RawSnapshots:          rawSnapshots,
		HTTPDiagnostics:       diagnostics,
		RateLimits:            trading212.RateLimitObservations(diagnostics),
		EnrichmentDiagnostics: enrichmentRun.Diagnostics,
		EnrichmentFailures:    enrichmentRun.Failures,
	}, nil
}

type enrichmentRun struct {
	Profiles    map[string]enrichment.Profile
	Diagnostics catalogue.EnrichmentDiagnostics
	Failures    []catalogue.EnrichmentFailure
}

func enrichAll(ctx context.Context, instruments []trading212.Instrument, providerName, cacheDir string) enrichmentRun {
	var inner enrichment.Provider
	if strings.EqualFold(providerName, "yahoo") {
		inner = enrichment.YahooProvider{}
	}
	provider := enrichment.CacheProvider{Dir: cacheDir, Inner: inner}
	run := enrichmentRun{
		Profiles: map[string]enrichment.Profile{},
		Diagnostics: catalogue.EnrichmentDiagnostics{
			CacheSchemaVersion: enrichment.CacheSchemaVersion,
			Provider:           normalizedEnrichmentProvider(providerName),
		},
	}
	for _, instrument := range instruments {
		parts := catalogue.ParseBrokerTicker(instrument.Ticker)
		req := enrichment.Request{
			Ticker:       instrument.Ticker,
			ISIN:         instrument.ISIN,
			Name:         instrument.Name,
			CurrencyCode: instrument.CurrencyCode,
			ExchangeCode: parts.ExchangeCode,
		}
		result, err := provider.Lookup(ctx, req)
		observeEnrichmentResult(&run, req, result, err)
		if err == nil && result.Status == enrichment.StatusHit {
			run.Profiles[instrument.Ticker] = result.Profile
		}
	}
	return run
}

func sampleEnrichment(instruments []trading212.Instrument, profiles map[string]enrichment.Profile) enrichmentRun {
	run := enrichmentRun{
		Profiles: profiles,
		Diagnostics: catalogue.EnrichmentDiagnostics{
			CacheSchemaVersion: enrichment.CacheSchemaVersion,
			Provider:           "sample",
		},
	}
	for _, instrument := range instruments {
		req := enrichment.Request{Ticker: instrument.Ticker, ISIN: instrument.ISIN, Name: instrument.Name, CurrencyCode: instrument.CurrencyCode}
		if profile, ok := profiles[instrument.Ticker]; ok {
			result := enrichment.Result{
				Provider:    "sample",
				Request:     enrichment.RequestSnapshot{Ticker: instrument.Ticker, ISIN: instrument.ISIN, Name: instrument.Name, CandidateSymbols: enrichment.CandidateSymbols(instrument.Ticker)},
				Profile:     profile,
				Status:      enrichment.StatusHit,
				RetrievedAt: profile.RetrievedAt,
			}
			observeEnrichmentResult(&run, req, result, nil)
			continue
		}
		result := enrichment.Result{
			Provider: "sample",
			Request: enrichment.RequestSnapshot{
				Ticker:           instrument.Ticker,
				ISIN:             instrument.ISIN,
				Name:             instrument.Name,
				CandidateSymbols: enrichment.CandidateSymbols(instrument.Ticker),
			},
			Status:           enrichment.StatusFailure,
			Error:            "sample enrichment profile not defined",
			AttemptedSymbols: enrichment.CandidateSymbols(instrument.Ticker),
		}
		observeEnrichmentResult(&run, req, result, errors.New(result.Error))
	}
	return run
}

func observeEnrichmentResult(run *enrichmentRun, req enrichment.Request, result enrichment.Result, err error) {
	switch result.CacheStatus {
	case enrichment.CacheStatusHit:
		run.Diagnostics.CacheHitCount++
	case enrichment.CacheStatusMiss:
		run.Diagnostics.CacheMissCount++
	}
	if result.Stale {
		run.Diagnostics.CacheStaleCount++
	}
	if result.RetrievedAt != "" {
		observeEnrichmentRetrievedAt(&run.Diagnostics, result.RetrievedAt)
	}
	if result.Status == enrichment.StatusAmbiguous {
		run.Diagnostics.AmbiguousCount++
	}
	if err != nil || result.Status != enrichment.StatusHit {
		run.Diagnostics.FailureCount++
		run.Failures = append(run.Failures, enrichment.FailureFromResult(req, result, err))
	}
}

func observeEnrichmentRetrievedAt(diagnostics *catalogue.EnrichmentDiagnostics, value string) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return
	}
	normalized := parsed.UTC().Format(time.RFC3339)
	if diagnostics.OldestRetrievedAt == "" {
		diagnostics.OldestRetrievedAt = normalized
	} else if oldest, err := time.Parse(time.RFC3339, diagnostics.OldestRetrievedAt); err == nil && parsed.Before(oldest) {
		diagnostics.OldestRetrievedAt = normalized
	}
	if diagnostics.NewestRetrievedAt == "" {
		diagnostics.NewestRetrievedAt = normalized
	} else if newest, err := time.Parse(time.RFC3339, diagnostics.NewestRetrievedAt); err == nil && parsed.After(newest) {
		diagnostics.NewestRetrievedAt = normalized
	}
}

func normalizedEnrichmentProvider(providerName string) string {
	if strings.EqualFold(providerName, "yahoo") {
		return "yahoo"
	}
	return "cache"
}

const rawSnapshotStampLayout = "20060102T150405Z"

func writeRawSnapshots(dir string, builtAt time.Time, instruments []trading212.Instrument, exchanges []trading212.Exchange) (catalogue.RawSnapshotSummary, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return catalogue.RawSnapshotSummary{}, err
	}
	stamp := builtAt.UTC().Format(rawSnapshotStampLayout)
	instrumentsPath := filepath.Join(dir, "instruments_"+stamp+".json")
	exchangesPath := filepath.Join(dir, "exchanges_"+stamp+".json")
	instrumentsLatestPath := filepath.Join(dir, "instruments_latest.json")
	exchangesLatestPath := filepath.Join(dir, "exchanges_latest.json")
	if err := writeJSON(instrumentsPath, instruments); err != nil {
		return catalogue.RawSnapshotSummary{}, err
	}
	if err := writeJSON(exchangesPath, exchanges); err != nil {
		return catalogue.RawSnapshotSummary{}, err
	}
	if err := writeJSON(instrumentsLatestPath, instruments); err != nil {
		return catalogue.RawSnapshotSummary{}, err
	}
	if err := writeJSON(exchangesLatestPath, exchanges); err != nil {
		return catalogue.RawSnapshotSummary{}, err
	}
	return catalogue.RawSnapshotSummary{
		Timestamp:           builtAt.UTC().Format(time.RFC3339),
		Directory:           manifestPath(dir),
		InstrumentsPath:     manifestPath(instrumentsPath),
		ExchangesPath:       manifestPath(exchangesPath),
		InstrumentsLatest:   manifestPath(instrumentsLatestPath),
		ExchangesLatest:     manifestPath(exchangesLatestPath),
		InstrumentFileCount: len(instruments),
		ExchangeFileCount:   len(exchanges),
	}, nil
}

func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func readRawSnapshots(dir string) ([]trading212.Instrument, []trading212.Exchange, catalogue.RawSnapshotSummary, error) {
	instrumentsLatestPath := filepath.Join(dir, "instruments_latest.json")
	exchangesLatestPath := filepath.Join(dir, "exchanges_latest.json")
	instrumentBytes, err := os.ReadFile(instrumentsLatestPath)
	if err != nil {
		return nil, nil, catalogue.RawSnapshotSummary{}, rawReplayFileError(instrumentsLatestPath, err)
	}
	exchangeBytes, err := os.ReadFile(exchangesLatestPath)
	if err != nil {
		return nil, nil, catalogue.RawSnapshotSummary{}, rawReplayFileError(exchangesLatestPath, err)
	}

	var instruments []trading212.Instrument
	if err := json.Unmarshal(instrumentBytes, &instruments); err != nil {
		return nil, nil, catalogue.RawSnapshotSummary{}, fmt.Errorf("decode raw replay instruments from %s: %w", instrumentsLatestPath, err)
	}
	var exchanges []trading212.Exchange
	if err := json.Unmarshal(exchangeBytes, &exchanges); err != nil {
		return nil, nil, catalogue.RawSnapshotSummary{}, fmt.Errorf("decode raw replay exchanges from %s: %w", exchangesLatestPath, err)
	}

	stamp := matchingRawSnapshotStamp(dir, instrumentBytes, exchangeBytes)
	timestamp := rawStampToRFC3339(stamp)
	instrumentsPath := ""
	exchangesPath := ""
	if stamp != "" {
		instrumentsPath = filepath.Join(dir, "instruments_"+stamp+".json")
		exchangesPath = filepath.Join(dir, "exchanges_"+stamp+".json")
	}
	return instruments, exchanges, catalogue.RawSnapshotSummary{
		Timestamp:           timestamp,
		Directory:           manifestPath(dir),
		InstrumentsPath:     manifestPath(instrumentsPath),
		ExchangesPath:       manifestPath(exchangesPath),
		InstrumentsLatest:   manifestPath(instrumentsLatestPath),
		ExchangesLatest:     manifestPath(exchangesLatestPath),
		InstrumentFileCount: len(instruments),
		ExchangeFileCount:   len(exchanges),
	}, nil
}

func rawReplayFileError(path string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("raw replay requested but %s is missing; run make sample or a credentialed refresh first, or pass --input-raw-dir to a directory containing instruments_latest.json and exchanges_latest.json", path)
	}
	return fmt.Errorf("read raw replay file %s: %w", path, err)
}

func matchingRawSnapshotStamp(dir string, instrumentBytes, exchangeBytes []byte) string {
	instrumentStamps := matchingStamps(dir, "instruments_", instrumentBytes)
	exchangeStamps := matchingStamps(dir, "exchanges_", exchangeBytes)
	exchangeSet := map[string]bool{}
	for _, stamp := range exchangeStamps {
		exchangeSet[stamp] = true
	}
	matches := []string{}
	for _, stamp := range instrumentStamps {
		if exchangeSet[stamp] {
			matches = append(matches, stamp)
		}
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

func matchingStamps(dir, prefix string, want []byte) []string {
	paths, err := filepath.Glob(filepath.Join(dir, prefix+"*.json"))
	if err != nil {
		return nil
	}
	stamps := []string{}
	for _, path := range paths {
		base := filepath.Base(path)
		stamp := strings.TrimSuffix(strings.TrimPrefix(base, prefix), ".json")
		if stamp == "latest" {
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if bytes.Equal(got, want) {
			stamps = append(stamps, stamp)
		}
	}
	return stamps
}

func rawStampToRFC3339(stamp string) string {
	if stamp == "" {
		return ""
	}
	parsed, err := time.Parse(rawSnapshotStampLayout, stamp)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339)
}

func replayEnrichment(ctx context.Context, instruments []trading212.Instrument, exchanges []trading212.Exchange, cacheDir string) enrichmentRun {
	if matchesEmbeddedSampleRaw(instruments, exchanges) {
		_, _, profiles := catalogue.SampleData()
		return sampleEnrichment(instruments, profiles)
	}
	return enrichAll(ctx, instruments, "cache", cacheDir)
}

func matchesEmbeddedSampleRaw(instruments []trading212.Instrument, exchanges []trading212.Exchange) bool {
	sampleInstruments, sampleExchanges, _ := catalogue.SampleData()
	return instrumentsByTickerEqual(instruments, sampleInstruments) && exchangesByIDEqual(exchanges, sampleExchanges)
}

func instrumentsByTickerEqual(a, b []trading212.Instrument) bool {
	if len(a) != len(b) {
		return false
	}
	byTicker := map[string]trading212.Instrument{}
	for _, instrument := range a {
		byTicker[instrument.Ticker] = instrument
	}
	for _, instrument := range b {
		if !reflect.DeepEqual(byTicker[instrument.Ticker], instrument) {
			return false
		}
	}
	return true
}

func exchangesByIDEqual(a, b []trading212.Exchange) bool {
	if len(a) != len(b) {
		return false
	}
	byID := map[int64]trading212.Exchange{}
	for _, exchange := range a {
		byID[exchange.ID] = exchange
	}
	for _, exchange := range b {
		if !reflect.DeepEqual(byID[exchange.ID], exchange) {
			return false
		}
	}
	return true
}

func manifestPath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.ToSlash(path)
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
