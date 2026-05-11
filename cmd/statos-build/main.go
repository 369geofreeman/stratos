package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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
	if command == "taxonomy" {
		return runTaxonomy(args, os.Stdout)
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
	enrichmentDelayValue := fs.String("enrichment-delay", getenvDefault("STATOS_ENRICHMENT_DELAY", ""), "optional delay between enrichment lookups, such as 500ms or 2s")
	if err := fs.Parse(args); err != nil {
		return err
	}

	switch command {
	case "refresh", "sample":
	default:
		return fmt.Errorf("unknown command %q; use refresh, sample, or taxonomy", command)
	}
	if command == "sample" {
		*forceSample = true
	}
	if *noFetch && *forceSample {
		return fmt.Errorf("choose either sample data or --no-fetch raw replay, not both")
	}
	enrichmentDelay, err := parseOptionalDuration(*enrichmentDelayValue)
	if err != nil {
		return fmt.Errorf("invalid --enrichment-delay: %w", err)
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
		ForceSample:     *forceSample,
		NoFetch:         *noFetch,
		BuiltAt:         builtAt,
		RawDir:          *rawDir,
		InputRawDir:     *inputRawDir,
		ProviderName:    *providerName,
		CacheDir:        *cacheDir,
		EnrichmentDelay: enrichmentDelay,
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
	previousManifest, err := readPreviousManifest(filepath.Join(*siteDataDir, "build_manifest.json"))
	if err != nil {
		return err
	}

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
		PreviousManifest:      previousManifest,
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

func readPreviousManifest(path string) (*catalogue.BuildManifest, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var manifest catalogue.BuildManifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return nil, fmt.Errorf("decode previous build manifest %s: %w", path, err)
	}
	return &manifest, nil
}

func runTaxonomy(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("taxonomy requires a subcommand: coverage or exposure-template")
	}
	subcommand := args[0]
	args = args[1:]
	switch subcommand {
	case "coverage":
		fs := flag.NewFlagSet("taxonomy coverage", flag.ExitOnError)
		cataloguePath := fs.String("catalogue", "site/data/catalogue.json", "generated catalogue JSON path")
		if err := fs.Parse(args); err != nil {
			return err
		}
		cat, err := readCatalogue(*cataloguePath)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(stdout, taxonomyCoverageReport(cat))
		return err
	case "exposure-template":
		fs := flag.NewFlagSet("taxonomy exposure-template", flag.ExitOnError)
		unclassifiedPath := fs.String("unclassified", "site/data/unclassified.csv", "generated unclassified CSV path")
		outPath := fs.String("out", "", "output CSV path; defaults to stdout")
		allowManualOut := fs.Bool("allow-manual-out", false, "allow writing directly to data/manual/exposures.csv")
		if err := fs.Parse(args); err != nil {
			return err
		}
		rows, err := readUnclassifiedForTemplate(*unclassifiedPath)
		if err != nil {
			return err
		}
		if *outPath == "" {
			return writeExposureTemplate(stdout, rows)
		}
		if !*allowManualOut && sameCleanPath(*outPath, filepath.Join("data", "manual", "exposures.csv")) {
			return fmt.Errorf("refusing to write directly to data/manual/exposures.csv without --allow-manual-out")
		}
		file, err := os.Create(*outPath)
		if err != nil {
			return err
		}
		defer file.Close()
		return writeExposureTemplate(file, rows)
	default:
		return fmt.Errorf("unknown taxonomy subcommand %q; use coverage or exposure-template", subcommand)
	}
}

func readCatalogue(path string) (*catalogue.Catalogue, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cat catalogue.Catalogue
	if err := json.Unmarshal(b, &cat); err != nil {
		return nil, fmt.Errorf("decode catalogue %s: %w", path, err)
	}
	return &cat, nil
}

func taxonomyCoverageReport(cat *catalogue.Catalogue) string {
	var b strings.Builder
	layerByTheme := supplyChainLayers(cat)
	exposuresByLayer := map[string][]taxonomy.Exposure{}
	for _, exposure := range cat.Exposures {
		exposuresByLayer[exposure.ThemeID+"|"+exposure.LayerID] = append(exposuresByLayer[exposure.ThemeID+"|"+exposure.LayerID], exposure)
	}

	fmt.Fprintln(&b, "# Theme coverage")
	fmt.Fprintln(&b, "theme_id\ttheme_name\texposed_tickers\texposed_companies\tcovered_layers\ttotal_layers")
	themes := append([]taxonomy.Theme(nil), cat.Themes...)
	sort.SliceStable(themes, func(i, j int) bool { return themes[i].ID < themes[j].ID })
	for _, theme := range themes {
		tickerCount := countTickersForTheme(cat.Tickers, theme.ID)
		companyCount := countCompaniesForTheme(cat.Companies, theme.ID)
		coveredLayers := countCoveredLayers(theme.ID, layerByTheme[theme.ID], exposuresByLayer)
		fmt.Fprintf(&b, "%s\t%s\t%d\t%d\t%d\t%d\n", theme.ID, theme.Name, tickerCount, companyCount, coveredLayers, len(layerByTheme[theme.ID]))
	}

	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "# Layer coverage")
	fmt.Fprintln(&b, "theme_id\tlayer_id\tlayer_name\texposure_rows\tconfidence_mix")
	for _, theme := range themes {
		layers := append([]taxonomy.SupplyChainLayer(nil), layerByTheme[theme.ID]...)
		sort.SliceStable(layers, func(i, j int) bool {
			if layers[i].Order == layers[j].Order {
				return layers[i].ID < layers[j].ID
			}
			return layers[i].Order < layers[j].Order
		})
		for _, layer := range layers {
			exposures := exposuresByLayer[theme.ID+"|"+layer.ID]
			fmt.Fprintf(&b, "%s\t%s\t%s\t%d\t%s\n", theme.ID, layer.ID, layer.Name, len(exposures), confidenceMix(exposures))
		}
	}

	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "# Sector counts")
	fmt.Fprintln(&b, "sector\tcount")
	for _, group := range sortedGroupCounts(cat.Sectors) {
		fmt.Fprintf(&b, "%s\t%d\n", group.Name, group.Count)
	}

	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "# Industry counts")
	fmt.Fprintln(&b, "industry\tcount")
	for _, group := range sortedGroupCounts(cat.Industries) {
		fmt.Fprintf(&b, "%s\t%d\n", group.Name, group.Count)
	}

	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "# Unclassified")
	fmt.Fprintln(&b, "unclassified_count")
	fmt.Fprintf(&b, "%d\n", len(cat.Unclassified))
	return b.String()
}

func supplyChainLayers(cat *catalogue.Catalogue) map[string][]taxonomy.SupplyChainLayer {
	out := map[string][]taxonomy.SupplyChainLayer{}
	for _, chain := range cat.SupplyChains {
		out[chain.ThemeID] = append(out[chain.ThemeID], chain.Layers...)
	}
	return out
}

func countTickersForTheme(tickers []catalogue.Ticker, themeID string) int {
	count := 0
	for _, ticker := range tickers {
		if contains(themeID, ticker.ThemeIDs) {
			count++
		}
	}
	return count
}

func countCompaniesForTheme(companies []catalogue.Company, themeID string) int {
	count := 0
	for _, company := range companies {
		if contains(themeID, company.ThemeIDs) {
			count++
		}
	}
	return count
}

func countCoveredLayers(themeID string, layers []taxonomy.SupplyChainLayer, exposuresByLayer map[string][]taxonomy.Exposure) int {
	count := 0
	for _, layer := range layers {
		if len(exposuresByLayer[themeID+"|"+layer.ID]) > 0 {
			count++
		}
	}
	return count
}

func confidenceMix(exposures []taxonomy.Exposure) string {
	if len(exposures) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, exposure := range exposures {
		counts[exposure.Confidence]++
	}
	order := []string{"manual_high", "manual_medium", "manual_low", "rule_high", "rule_medium", "rule_low"}
	parts := []string{}
	for _, confidence := range order {
		if counts[confidence] > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", confidence, counts[confidence]))
		}
	}
	extras := []string{}
	for confidence := range counts {
		if !contains(confidence, order) {
			extras = append(extras, confidence)
		}
	}
	sort.Strings(extras)
	for _, confidence := range extras {
		parts = append(parts, fmt.Sprintf("%s=%d", confidence, counts[confidence]))
	}
	return strings.Join(parts, ";")
}

func sortedGroupCounts(groups []catalogue.GroupCount) []catalogue.GroupCount {
	out := append([]catalogue.GroupCount(nil), groups...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func readUnclassifiedForTemplate(path string) ([]catalogue.UnclassifiedRow, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	wantHeaders := []string{"ticker", "company_id", "name", "isin", "reason"}
	if len(records[0]) != len(wantHeaders) && len(records[0]) != len(wantHeaders)+1 {
		return nil, fmt.Errorf("%s has unexpected unclassified header", path)
	}
	for i, header := range wantHeaders {
		if strings.TrimSpace(records[0][i]) != header {
			return nil, fmt.Errorf("%s has unexpected unclassified header column %d %q", path, i+1, records[0][i])
		}
	}
	hasReasonCodes := len(records[0]) == len(wantHeaders)+1
	if hasReasonCodes && strings.TrimSpace(records[0][len(wantHeaders)]) != "reason_codes" {
		return nil, fmt.Errorf("%s has unexpected unclassified header column %d %q", path, len(wantHeaders)+1, records[0][len(wantHeaders)])
	}
	rows := []catalogue.UnclassifiedRow{}
	seen := map[string]bool{}
	for i, record := range records[1:] {
		if len(record) != len(records[0]) {
			return nil, fmt.Errorf("%s row %d has %d fields, want %d", path, i+2, len(record), len(records[0]))
		}
		row := catalogue.UnclassifiedRow{
			Ticker:    strings.TrimSpace(record[0]),
			CompanyID: strings.TrimSpace(record[1]),
			Name:      strings.TrimSpace(record[2]),
			ISIN:      strings.TrimSpace(record[3]),
			Reason:    strings.TrimSpace(record[4]),
		}
		if hasReasonCodes {
			row.ReasonCodes = splitSemicolonCSVList(record[5])
		}
		if row.Ticker == "" || seen[row.Ticker] {
			continue
		}
		seen[row.Ticker] = true
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Ticker < rows[j].Ticker })
	return rows, nil
}

func splitSemicolonCSVList(value string) []string {
	parts := strings.Split(value, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func writeExposureTemplate(w io.Writer, rows []catalogue.UnclassifiedRow) error {
	writer := csv.NewWriter(w)
	headers := []string{"theme_id", "layer_id", "ticker", "isin", "company_id", "exposure_score", "confidence", "source_url", "rationale", "last_reviewed"}
	if err := writer.Write(headers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write([]string{"", "", row.Ticker, row.ISIN, row.CompanyID, "", "", "", "", ""}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func sameCleanPath(a, b string) bool {
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	if cleanA == cleanB {
		return true
	}
	absA, errA := filepath.Abs(cleanA)
	absB, errB := filepath.Abs(cleanB)
	return errA == nil && errB == nil && absA == absB
}

func contains(value string, values []string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}

type sourceOptions struct {
	ForceSample     bool
	NoFetch         bool
	BuiltAt         time.Time
	RawDir          string
	InputRawDir     string
	ProviderName    string
	CacheDir        string
	EnrichmentDelay time.Duration
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
	enrichmentRun := enrichAll(ctx, instruments, opts.ProviderName, opts.CacheDir, opts.EnrichmentDelay)
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

const enrichmentProgressInterval = 60 * time.Second
const enrichmentProgressMinInstruments = 1000

type enrichmentGroup struct {
	Key         string
	Request     enrichment.Request
	Instruments []trading212.Instrument
}

func enrichAll(ctx context.Context, instruments []trading212.Instrument, providerName, cacheDir string, delay time.Duration) enrichmentRun {
	var inner enrichment.Provider
	if strings.EqualFold(providerName, "yahoo") {
		inner = enrichment.YahooProvider{}
	}
	provider := enrichment.CacheProvider{Dir: cacheDir, Inner: inner}
	return enrichAllWithProvider(ctx, instruments, provider, normalizedEnrichmentProvider(providerName), delay)
}

func enrichAllWithProvider(ctx context.Context, instruments []trading212.Instrument, provider enrichment.Provider, providerName string, delay time.Duration) enrichmentRun {
	run := enrichmentRun{
		Profiles: map[string]enrichment.Profile{},
		Diagnostics: catalogue.EnrichmentDiagnostics{
			CacheSchemaVersion: enrichment.CacheSchemaVersion,
			Provider:           providerName,
		},
	}
	groups := buildEnrichmentGroups(instruments)
	started := time.Now()
	nextProgressAt := started.Add(enrichmentProgressInterval)
	reportProgress := len(instruments) >= enrichmentProgressMinInstruments
	if reportProgress {
		log.Printf("enrichment started: provider=%s tickers=%d identities=%d delay=%s", run.Diagnostics.Provider, len(instruments), len(groups), delay)
	}
	processedTickers := 0
	for index, group := range groups {
		result, err := provider.Lookup(ctx, group.Request)
		observeEnrichmentProviderResult(&run, result)
		if err == nil && result.Status == enrichment.StatusHit {
			for _, instrument := range group.Instruments {
				run.Profiles[instrument.Ticker] = result.Profile
			}
		} else {
			for _, instrument := range group.Instruments {
				recordEnrichmentFailure(&run, enrichmentRequestForInstrument(instrument), result, err)
			}
		}
		processedTickers += len(group.Instruments)
		if reportProgress && time.Now().After(nextProgressAt) {
			logEnrichmentProgress(run, index+1, len(groups), processedTickers, len(instruments), started)
			nextProgressAt = time.Now().Add(enrichmentProgressInterval)
		}
		if delay > 0 && index+1 < len(groups) {
			select {
			case <-ctx.Done():
				return run
			case <-time.After(delay):
			}
		}
	}
	if reportProgress {
		logEnrichmentProgress(run, len(groups), len(groups), len(instruments), len(instruments), started)
	}
	return run
}

func buildEnrichmentGroups(instruments []trading212.Instrument) []enrichmentGroup {
	groupByKey := map[string]int{}
	var groups []enrichmentGroup
	for _, instrument := range instruments {
		key := enrichmentIdentityKey(instrument)
		index, ok := groupByKey[key]
		if !ok {
			groupByKey[key] = len(groups)
			groups = append(groups, enrichmentGroup{Key: key})
			index = len(groups) - 1
		}
		groups[index].Instruments = append(groups[index].Instruments, instrument)
	}
	for index := range groups {
		sort.SliceStable(groups[index].Instruments, func(i, j int) bool {
			return strings.ToUpper(groups[index].Instruments[i].Ticker) < strings.ToUpper(groups[index].Instruments[j].Ticker)
		})
		groups[index].Request = enrichmentRequestForGroup(groups[index].Instruments)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].Key < groups[j].Key
	})
	return groups
}

func enrichmentIdentityKey(instrument trading212.Instrument) string {
	if isin := strings.ToUpper(strings.TrimSpace(instrument.ISIN)); isin != "" {
		return "isin:" + isin
	}
	if ticker := strings.ToUpper(strings.TrimSpace(instrument.Ticker)); ticker != "" {
		return "ticker:" + ticker
	}
	return "name:" + strings.ToUpper(strings.TrimSpace(instrument.Name))
}

func enrichmentRequestForGroup(instruments []trading212.Instrument) enrichment.Request {
	if len(instruments) == 0 {
		return enrichment.Request{}
	}
	req := enrichmentRequestForInstrument(instruments[0])
	var candidates []string
	for _, instrument := range instruments {
		candidates = appendUniqueCandidateSymbols(candidates, enrichment.CandidateSymbols(instrument.Ticker)...)
	}
	req.CandidateSymbols = candidates
	return req
}

func enrichmentRequestForInstrument(instrument trading212.Instrument) enrichment.Request {
	parts := catalogue.ParseBrokerTicker(instrument.Ticker)
	return enrichment.Request{
		Ticker:       instrument.Ticker,
		ISIN:         instrument.ISIN,
		Name:         instrument.Name,
		CurrencyCode: instrument.CurrencyCode,
		ExchangeCode: parts.ExchangeCode,
	}
}

func appendUniqueCandidateSymbols(existing []string, additions ...string) []string {
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

func logEnrichmentProgress(run enrichmentRun, processedIdentities, totalIdentities, processedTickers, totalTickers int, started time.Time) {
	elapsed := time.Since(started).Round(time.Second)
	log.Printf(
		"enrichment progress: provider=%s identities=%d/%d tickers=%d/%d hits=%d failures=%d ambiguous=%d cache_hits=%d cache_misses=%d stale=%d elapsed=%s",
		run.Diagnostics.Provider,
		processedIdentities,
		totalIdentities,
		processedTickers,
		totalTickers,
		len(run.Profiles),
		run.Diagnostics.FailureCount,
		run.Diagnostics.AmbiguousCount,
		run.Diagnostics.CacheHitCount,
		run.Diagnostics.CacheMissCount,
		run.Diagnostics.CacheStaleCount,
		elapsed,
	)
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
		req := enrichmentRequestForInstrument(instrument)
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
	observeEnrichmentProviderResult(run, result)
	if err != nil || result.Status != enrichment.StatusHit {
		recordEnrichmentFailure(run, req, result, err)
	}
}

func observeEnrichmentProviderResult(run *enrichmentRun, result enrichment.Result) {
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
}

func recordEnrichmentFailure(run *enrichmentRun, req enrichment.Request, result enrichment.Result, err error) {
	run.Diagnostics.FailureCount++
	run.Failures = append(run.Failures, enrichment.FailureFromResult(req, result, err))
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
	return enrichAll(ctx, instruments, "cache", cacheDir, 0)
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

func parseOptionalDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" {
		return 0, nil
	}
	return time.ParseDuration(value)
}
