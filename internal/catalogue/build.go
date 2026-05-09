package catalogue

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"statos/internal/enrichment"
	"statos/internal/taxonomy"
	"statos/internal/trading212"
)

type BuildInput struct {
	Instruments           []trading212.Instrument
	Exchanges             []trading212.Exchange
	Profiles              map[string]enrichment.Profile
	Manual                taxonomy.ManualData
	BuiltAt               time.Time
	Trading212Environment string
	RawSnapshotAt         string
	EnrichmentAttempted   int
	EnrichmentSucceeded   int
	EnrichmentFailed      int
}

func Build(input BuildInput) (*Catalogue, error) {
	if input.BuiltAt.IsZero() {
		input.BuiltAt = time.Now().UTC()
	}
	if err := taxonomy.Validate(input.Manual); err != nil {
		return nil, err
	}

	exchanges := map[int64]trading212.Exchange{}
	for _, exchange := range input.Exchanges {
		exchanges[exchange.ID] = exchange
	}

	instruments := append([]trading212.Instrument(nil), input.Instruments...)
	sort.SliceStable(instruments, func(i, j int) bool {
		return instruments[i].Ticker < instruments[j].Ticker
	})

	securityByID := map[string]*Security{}
	companyByID := map[string]*Company{}
	listingByID := map[string]*Listing{}
	tickerByID := map[string]*Ticker{}
	companyBySecurity := map[string]string{}
	securityByISIN := map[string]string{}

	for _, raw := range instruments {
		if override := input.Manual.TickerOverrides[raw.Ticker]; override.CompanyID != "" {
			companyBySecurity[securityID(raw)] = override.CompanyID
		}
	}

	for _, raw := range instruments {
		if raw.Ticker == "" {
			continue
		}
		parts := ParseBrokerTicker(raw.Ticker)
		profile := input.Profiles[raw.Ticker]
		tickerOverride := input.Manual.TickerOverrides[raw.Ticker]

		name := firstNonEmpty(tickerOverride.Name, profile.Name, raw.Name, raw.ShortName, raw.Ticker)
		securityID := securityID(raw)
		if raw.ISIN != "" {
			securityByISIN[raw.ISIN] = securityID
		}
		companyID := firstNonEmpty(tickerOverride.CompanyID, companyBySecurity[securityID])
		if companyID == "" {
			companyID = Slug(name)
		}
		if companyID == "" {
			companyID = Slug(raw.Ticker)
		}
		companyBySecurity[securityID] = companyID
		companyOverride := input.Manual.CompanyOverrides[companyID]

		sector := firstNonEmpty(tickerOverride.Sector, companyOverride.Sector, profile.Sector)
		industry := firstNonEmpty(tickerOverride.Industry, companyOverride.Industry, profile.Industry)
		country := firstNonEmpty(tickerOverride.Country, companyOverride.Country, profile.Country)
		yahooSymbol := firstNonEmpty(tickerOverride.YahooSymbol, profile.Symbol)
		companyName := firstNonEmpty(companyOverride.Name, tickerOverride.Name, profile.Name, raw.Name, raw.ShortName, raw.Ticker)
		lastReviewed := firstNonEmpty(tickerOverride.LastReviewed, companyOverride.LastReviewed)

		exchange := exchanges[raw.WorkingScheduleID]
		listingID := raw.Ticker
		listing := &Listing{
			ID:           listingID,
			Ticker:       raw.Ticker,
			SecurityID:   securityID,
			CompanyID:    companyID,
			ExchangeCode: parts.ExchangeCode,
			ExchangeName: firstNonEmpty(exchange.Name, profile.Exchange),
			CurrencyCode: raw.CurrencyCode,
		}
		listingByID[listingID] = listing

		security := securityByID[securityID]
		if security == nil {
			security = &Security{
				ID:        securityID,
				ISIN:      raw.ISIN,
				Name:      name,
				Type:      raw.Type,
				CompanyID: companyID,
			}
			securityByID[securityID] = security
		}
		security.ListingIDs = appendUnique(security.ListingIDs, listingID)
		security.TickerIDs = appendUnique(security.TickerIDs, raw.Ticker)
		security.CurrencySet = appendUnique(security.CurrencySet, raw.CurrencyCode)

		company := companyByID[companyID]
		if company == nil {
			company = &Company{
				ID:            companyID,
				Name:          companyName,
				PrimaryTicker: raw.Ticker,
			}
			companyByID[companyID] = company
		}
		company.Name = firstNonEmpty(company.Name, companyName)
		company.Sector = firstNonEmpty(company.Sector, sector)
		company.Industry = firstNonEmpty(company.Industry, industry)
		company.Country = firstNonEmpty(company.Country, country)
		company.YahooSymbol = firstNonEmpty(company.YahooSymbol, yahooSymbol)
		if company.MarketCap == 0 {
			company.MarketCap = profile.MarketCap
		}
		company.SecurityIDs = appendUnique(company.SecurityIDs, securityID)
		company.ListingIDs = appendUnique(company.ListingIDs, listingID)
		company.TickerIDs = appendUnique(company.TickerIDs, raw.Ticker)
		company.LastReviewed = firstNonEmpty(company.LastReviewed, lastReviewed)
		company.LastRefreshed = input.BuiltAt.Format(time.RFC3339)
		company.Sources = appendSources(company.Sources, sourceFromOverride(tickerOverride), sourceFromCompanyOverride(companyOverride), sourceFromProfile(profile))

		ticker := &Ticker{
			Ticker:            raw.Ticker,
			BrokerSymbol:      parts.Symbol,
			Name:              name,
			ShortName:         raw.ShortName,
			Type:              raw.Type,
			CurrencyCode:      raw.CurrencyCode,
			ISIN:              raw.ISIN,
			ExchangeCode:      parts.ExchangeCode,
			ExchangeName:      listing.ExchangeName,
			WorkingScheduleID: raw.WorkingScheduleID,
			MaxOpenQuantity:   raw.MaxOpenQuantity,
			ExtendedHours:     raw.ExtendedHours,
			SecurityID:        securityID,
			CompanyID:         companyID,
			ListingID:         listingID,
			Directionality:    DetectDirectionality(raw.Ticker, raw.Name, raw.ShortName),
			YahooSymbol:       yahooSymbol,
			Sector:            sector,
			Industry:          industry,
			Country:           country,
			MarketCap:         profile.MarketCap,
			Sources:           appendSources(nil, sourceFromOverride(tickerOverride), sourceFromCompanyOverride(companyOverride), sourceFromProfile(profile)),
			LastReviewed:      lastReviewed,
			LastRefreshed:     input.BuiltAt.Format(time.RFC3339),
		}
		tickerByID[raw.Ticker] = ticker
	}

	applyExposures(input.Manual.Exposures, tickerByID, companyByID, securityByISIN)
	addRelatedTickers(tickerByID, companyByID)

	tickers := sortedValues(tickerByID)
	securities := sortedValues(securityByID)
	listings := sortedValues(listingByID)
	companies := sortedValues(companyByID)
	for _, ticker := range tickers {
		ticker.Unclassified = isUnclassified(ticker)
	}
	unclassified := buildUnclassified(tickers)
	sectors := groupTickers(tickers, func(t *Ticker) string { return t.Sector })
	industries := groupTickers(tickers, func(t *Ticker) string { return t.Industry })

	cat := &Catalogue{
		Tickers:      derefTickers(tickers),
		Securities:   derefSecurities(securities),
		Listings:     derefListings(listings),
		Companies:    derefCompanies(companies),
		Sectors:      sectors,
		Industries:   industries,
		Themes:       input.Manual.Themes,
		SupplyChains: input.Manual.SupplyChains,
		Exposures:    input.Manual.Exposures,
		Notes:        input.Manual.Notes,
		Unclassified: unclassified,
	}
	cat.Manifest = BuildManifest{
		BuiltAt:               input.BuiltAt.UTC().Format(time.RFC3339),
		Trading212Environment: input.Trading212Environment,
		InstrumentCount:       len(input.Instruments),
		ExchangeCount:         len(input.Exchanges),
		SecurityCount:         len(cat.Securities),
		CompanyCount:          len(cat.Companies),
		ListingCount:          len(cat.Listings),
		ThemeCount:            len(cat.Themes),
		ExposureCount:         len(cat.Exposures),
		EnrichmentAttempted:   input.EnrichmentAttempted,
		EnrichmentSucceeded:   input.EnrichmentSucceeded,
		EnrichmentFailed:      input.EnrichmentFailed,
		UnclassifiedCount:     len(cat.Unclassified),
		RawSnapshotAt:         input.RawSnapshotAt,
		DataFreshness:         freshness(input.RawSnapshotAt, input.BuiltAt),
	}
	return cat, nil
}

type BrokerTickerParts struct {
	Symbol       string
	ExchangeCode string
	AssetCode    string
}

func ParseBrokerTicker(ticker string) BrokerTickerParts {
	parts := strings.Split(ticker, "_")
	if len(parts) >= 3 {
		return BrokerTickerParts{
			Symbol:       strings.Join(parts[:len(parts)-2], "_"),
			ExchangeCode: parts[len(parts)-2],
			AssetCode:    parts[len(parts)-1],
		}
	}
	return BrokerTickerParts{Symbol: ticker}
}

func DetectDirectionality(values ...string) string {
	joined := " " + strings.ToUpper(strings.Join(values, " ")) + " "
	shortMarkers := []string{" SHORT ", " INVERSE ", " -1X ", " -2X ", " -3X ", " 1S ", " 2S ", " 3S ", " X1S ", " X2S ", " X3S "}
	for _, marker := range shortMarkers {
		if strings.Contains(joined, marker) || strings.Contains(joined, strings.TrimSpace(marker)) {
			return "inverse_or_short"
		}
	}
	leveragedMarkers := []string{" LEVERAGED ", " LEVERAGE ", " 2X ", " 3X ", " X2 ", " X3 "}
	for _, marker := range leveragedMarkers {
		if strings.Contains(joined, marker) {
			return "leveraged_long"
		}
	}
	return "long_or_unlevered"
}

func Slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteRune('_')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func securityID(raw trading212.Instrument) string {
	if raw.ISIN != "" {
		return "isin:" + raw.ISIN
	}
	return "ticker:" + raw.Ticker
}

func applyExposures(exposures []taxonomy.Exposure, tickers map[string]*Ticker, companies map[string]*Company, securityByISIN map[string]string) {
	companyIDsForExposure := func(exposure taxonomy.Exposure) []string {
		ids := []string{}
		if exposure.CompanyID != "" {
			ids = append(ids, exposure.CompanyID)
		}
		if exposure.Ticker != "" {
			if ticker := tickers[exposure.Ticker]; ticker != nil {
				ids = append(ids, ticker.CompanyID)
			}
		}
		if exposure.ISIN != "" {
			securityID := securityByISIN[exposure.ISIN]
			for _, ticker := range tickers {
				if ticker.SecurityID == securityID {
					ids = append(ids, ticker.CompanyID)
				}
			}
		}
		return unique(ids)
	}

	for _, exposure := range exposures {
		companyIDs := companyIDsForExposure(exposure)
		for _, companyID := range companyIDs {
			company := companies[companyID]
			if company == nil {
				continue
			}
			company.ThemeIDs = appendUnique(company.ThemeIDs, exposure.ThemeID)
			company.LayerIDs = appendUnique(company.LayerIDs, exposure.LayerID)
			for _, tickerID := range company.TickerIDs {
				ticker := tickers[tickerID]
				if ticker == nil {
					continue
				}
				ticker.ThemeIDs = appendUnique(ticker.ThemeIDs, exposure.ThemeID)
				ticker.LayerIDs = appendUnique(ticker.LayerIDs, exposure.LayerID)
			}
		}
	}
}

func addRelatedTickers(tickers map[string]*Ticker, companies map[string]*Company) {
	byIndustry := map[string][]string{}
	for _, ticker := range tickers {
		if ticker.Industry != "" {
			byIndustry[ticker.Industry] = append(byIndustry[ticker.Industry], ticker.Ticker)
		}
	}
	for _, company := range companies {
		related := []string{}
		related = append(related, company.TickerIDs...)
		if company.Industry != "" {
			related = append(related, byIndustry[company.Industry]...)
		}
		related = unique(related)
		for _, tickerID := range company.TickerIDs {
			ticker := tickers[tickerID]
			if ticker != nil {
				ticker.RelatedTickers = removeValue(related, tickerID)
			}
		}
		company.RelatedTickers = removeValue(related, company.PrimaryTicker)
	}
}

func buildUnclassified(tickers []*Ticker) []UnclassifiedRow {
	var out []UnclassifiedRow
	for _, ticker := range tickers {
		reasons := []string{}
		if ticker.Sector == "" {
			reasons = append(reasons, "missing sector")
		}
		if ticker.Industry == "" {
			reasons = append(reasons, "missing industry")
		}
		if len(ticker.ThemeIDs) == 0 {
			reasons = append(reasons, "missing theme exposure")
		}
		if len(reasons) == 0 {
			continue
		}
		out = append(out, UnclassifiedRow{
			Ticker:    ticker.Ticker,
			CompanyID: ticker.CompanyID,
			Name:      ticker.Name,
			ISIN:      ticker.ISIN,
			Reason:    strings.Join(reasons, "; "),
		})
	}
	return out
}

func isUnclassified(ticker *Ticker) bool {
	return ticker.Sector == "" || ticker.Industry == "" || len(ticker.ThemeIDs) == 0
}

func groupTickers(tickers []*Ticker, keyFn func(*Ticker) string) []GroupCount {
	byKey := map[string][]string{}
	for _, ticker := range tickers {
		key := keyFn(ticker)
		if key == "" {
			key = "Unclassified"
		}
		byKey[key] = append(byKey[key], ticker.Ticker)
	}
	out := make([]GroupCount, 0, len(byKey))
	for key, ids := range byKey {
		sort.Strings(ids)
		out = append(out, GroupCount{ID: Slug(key), Name: key, Count: len(ids), Tickers: ids})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Name < out[j].Name
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func freshness(rawSnapshotAt string, builtAt time.Time) string {
	if rawSnapshotAt == "" {
		return "sample_or_unknown"
	}
	parsed, err := time.Parse(time.RFC3339, rawSnapshotAt)
	if err != nil {
		return "unknown"
	}
	return fmt.Sprintf("%.0fh", builtAt.Sub(parsed).Hours())
}

func sourceFromOverride(override taxonomy.TickerOverride) Source {
	if override.SourceURL == "" {
		return Source{}
	}
	return Source{Kind: "manual_ticker_override", URL: override.SourceURL, Label: "Ticker override", LastReviewed: override.LastReviewed}
}

func sourceFromCompanyOverride(override taxonomy.CompanyOverride) Source {
	if override.SourceURL == "" {
		return Source{}
	}
	return Source{Kind: "manual_company_override", URL: override.SourceURL, Label: "Company override", LastReviewed: override.LastReviewed}
}

func sourceFromProfile(profile enrichment.Profile) Source {
	if profile.Source == "" {
		return Source{}
	}
	return Source{Kind: "enrichment", Label: profile.Source, LastReviewed: profile.RetrievedAt}
}

func appendSources(existing []Source, sources ...Source) []Source {
	seen := map[string]bool{}
	for _, source := range existing {
		seen[source.Kind+"|"+source.URL+"|"+source.Label] = true
	}
	for _, source := range sources {
		if source.Kind == "" && source.URL == "" && source.Label == "" {
			continue
		}
		key := source.Kind + "|" + source.URL + "|" + source.Label
		if !seen[key] {
			existing = append(existing, source)
			seen[key] = true
		}
	}
	return existing
}

type sortable interface {
	*Ticker | *Security | *Listing | *Company
}

func sortedValues[T sortable](m map[string]T) []T {
	out := make([]T, 0, len(m))
	for _, value := range m {
		out = append(out, value)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return objectID(out[i]) < objectID(out[j])
	})
	return out
}

func objectID(value any) string {
	switch v := value.(type) {
	case *Ticker:
		return v.Ticker
	case *Security:
		return v.ID
	case *Listing:
		return v.ID
	case *Company:
		return v.ID
	default:
		return ""
	}
}

func derefTickers(values []*Ticker) []Ticker {
	out := make([]Ticker, len(values))
	for i, value := range values {
		sort.Strings(value.ThemeIDs)
		sort.Strings(value.LayerIDs)
		sort.Strings(value.RelatedTickers)
		out[i] = *value
	}
	return out
}

func derefSecurities(values []*Security) []Security {
	out := make([]Security, len(values))
	for i, value := range values {
		sort.Strings(value.ListingIDs)
		sort.Strings(value.TickerIDs)
		sort.Strings(value.CurrencySet)
		out[i] = *value
	}
	return out
}

func derefListings(values []*Listing) []Listing {
	out := make([]Listing, len(values))
	for i, value := range values {
		out[i] = *value
	}
	return out
}

func derefCompanies(values []*Company) []Company {
	out := make([]Company, len(values))
	for i, value := range values {
		sort.Strings(value.SecurityIDs)
		sort.Strings(value.ListingIDs)
		sort.Strings(value.TickerIDs)
		sort.Strings(value.ThemeIDs)
		sort.Strings(value.LayerIDs)
		sort.Strings(value.RelatedTickers)
		out[i] = *value
	}
	return out
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func unique(values []string) []string {
	out := []string{}
	for _, value := range values {
		out = appendUnique(out, value)
	}
	sort.Strings(out)
	return out
}

func removeValue(values []string, remove string) []string {
	out := []string{}
	for _, value := range values {
		if value != remove {
			out = appendUnique(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
