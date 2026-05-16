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
	SourceMode            string
	Trading212Environment string
	Trading212BaseURL     string
	Trading212FetchAt     string
	RawSnapshotAt         string
	RawSnapshots          RawSnapshotSummary
	HTTPDiagnostics       []trading212.EndpointDiagnostic
	RateLimits            []trading212.RateLimitObservation
	EnrichmentAttempted   int
	EnrichmentSucceeded   int
	EnrichmentFailed      int
	EnrichmentDiagnostics EnrichmentDiagnostics
	EnrichmentFailures    []EnrichmentFailure
	PreviousManifest      *BuildManifest
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

	state := newIdentityBuildState()
	processable := make([]trading212.Instrument, 0, len(instruments))
	seenTickers := map[string]bool{}
	for _, raw := range instruments {
		raw.Ticker = strings.TrimSpace(raw.Ticker)
		if raw.Ticker == "" {
			state.emptyTickerCount++
			state.addIssue(IdentityIssue{
				IssueCode:       "missing_ticker",
				Name:            firstNonEmpty(raw.Name, raw.ShortName),
				ISIN:            raw.ISIN,
				Reason:          "Trading 212 instrument has an empty ticker and cannot be exported as a broker-level row",
				SuggestedAction: "review the raw snapshot or wait for corrected broker metadata",
			})
			continue
		}
		if seenTickers[raw.Ticker] {
			state.duplicateTickerCount++
			state.addIssue(IdentityIssue{
				IssueCode:       "duplicate_ticker",
				Ticker:          raw.Ticker,
				ISIN:            raw.ISIN,
				Name:            firstNonEmpty(raw.Name, raw.ShortName),
				Reason:          "multiple Trading 212 instruments share the same broker ticker; only the first sorted row is exported",
				SuggestedAction: "add a manual identity override or inspect the raw Trading 212 snapshot",
			})
			continue
		}
		seenTickers[raw.Ticker] = true
		if raw.ISIN == "" {
			state.missingISINCount++
		}
		processable = append(processable, raw)
	}

	securityByID := map[string]*Security{}
	companyByID := map[string]*Company{}
	listingByID := map[string]*Listing{}
	tickerByID := map[string]*Ticker{}
	companyBySecurity := map[string]string{}
	companyConfidenceBySecurity := map[string]string{}
	companyReasonsBySecurity := map[string][]string{}
	securityByISIN := map[string][]string{}
	tickersByISIN := map[string][]string{}
	companiesByISIN := map[string][]string{}
	isinsBySecurity := map[string][]string{}
	companiesBySecurity := map[string][]string{}
	categoriesBySecurity := map[string][]string{}

	for _, raw := range processable {
		if override := input.Manual.TickerOverrides[raw.Ticker]; override.CompanyID != "" {
			companyBySecurity[securityID(raw)] = override.CompanyID
			companyConfidenceBySecurity[securityID(raw)] = "manual_high"
			companyReasonsBySecurity[securityID(raw)] = []string{"company_id_from_ticker_override"}
		}
	}

	for _, raw := range processable {
		parts := ParseBrokerTicker(raw.Ticker)
		profile := input.Profiles[raw.Ticker]
		tickerOverride := input.Manual.TickerOverrides[raw.Ticker]

		name := firstNonEmpty(tickerOverride.Name, profile.Name, raw.Name, raw.ShortName, raw.Ticker)
		baseSecurityID := securityID(raw)
		companyID := firstNonEmpty(tickerOverride.CompanyID, companyBySecurity[baseSecurityID])
		companyConfidence := ""
		companyReasons := []string{}
		switch {
		case tickerOverride.CompanyID != "":
			companyConfidence = "manual_high"
			companyReasons = append(companyReasons, "company_id_from_ticker_override")
		case companyBySecurity[baseSecurityID] != "":
			companyConfidence = firstNonEmpty(companyConfidenceBySecurity[baseSecurityID], "rule_medium")
			companyReasons = append(companyReasons, companyReasonsBySecurity[baseSecurityID]...)
		}
		if companyID == "" {
			companyID = Slug(name)
			companyConfidence = "rule_medium"
			companyReasons = append(companyReasons, "company_id_from_name_slug")
		}
		if companyID == "" {
			companyID = Slug(raw.Ticker)
			companyConfidence = "rule_low"
			companyReasons = append(companyReasons, "company_id_from_ticker_slug")
		}
		identityOverride := resolveIdentityOverrides(raw, baseSecurityID, companyID, input.Manual.IdentityOverrides, state)
		securityID := firstNonEmpty(identityOverride.SecurityID, baseSecurityID)
		if identityOverride.CompanyID == "" && tickerOverride.CompanyID == "" && companyBySecurity[securityID] != "" {
			companyID = companyBySecurity[securityID]
			companyConfidence = firstNonEmpty(companyConfidenceBySecurity[securityID], companyConfidence)
			companyReasons = append(companyReasons, companyReasonsBySecurity[securityID]...)
		}
		if identityOverride.CompanyID != "" {
			companyID = identityOverride.CompanyID
			companyConfidence = firstNonEmpty(identityOverride.Confidence, "manual_high")
			companyReasons = append(companyReasons, "company_id_from_identity_override")
		}

		category, categoryReasons := ClassifyInstrumentCategory(raw)
		if identityOverride.Category != "" {
			category = identityOverride.Category
			categoryReasons = append(categoryReasons, "category_from_identity_override")
		}
		structureFlags := DetectStructureFlags(category, raw.Ticker, raw.Name, raw.ShortName)
		if len(identityOverride.Flags) > 0 {
			structureFlags = mergeFlags(structureFlags, identityOverride.Flags)
		}
		flagReasons := []string{}
		if len(structureFlags) > 0 {
			flagReasons = append(flagReasons, "structure_flags_from_name_or_ticker")
		}
		if len(identityOverride.Flags) > 0 {
			flagReasons = append(flagReasons, "structure_flags_from_identity_override")
		}
		if companyConfidence == "rule_medium" {
			switch {
			case raw.ISIN == "":
				companyConfidence = "rule_low"
				companyReasons = append(companyReasons, "missing_isin_company_identity_needs_review")
			case containsString(structureFlags, "adr"), containsString(structureFlags, "gdr"):
				companyConfidence = "rule_low"
				companyReasons = append(companyReasons, "depositary_receipt_company_identity_needs_review")
			case containsString(structureFlags, "fund_like"):
				companyReasons = append(companyReasons, "fund_like_company_identity_from_isin")
			}
		}
		if companyBySecurity[securityID] == "" {
			companyBySecurity[securityID] = companyID
			companyConfidenceBySecurity[securityID] = companyConfidence
			companyReasonsBySecurity[securityID] = append([]string(nil), companyReasons...)
		}
		if raw.ISIN != "" {
			securityByISIN[raw.ISIN] = appendUnique(securityByISIN[raw.ISIN], securityID)
			tickersByISIN[raw.ISIN] = appendUnique(tickersByISIN[raw.ISIN], raw.Ticker)
			companiesByISIN[raw.ISIN] = appendUnique(companiesByISIN[raw.ISIN], companyID)
		}
		isinsBySecurity[securityID] = appendUnique(isinsBySecurity[securityID], raw.ISIN)
		companiesBySecurity[securityID] = appendUnique(companiesBySecurity[securityID], companyID)
		categoriesBySecurity[securityID] = appendUnique(categoriesBySecurity[securityID], category)
		state.categoryCounts[category]++
		for _, flag := range structureFlags {
			state.flagCounts[flag]++
		}

		companyOverride := input.Manual.CompanyOverrides[companyID]
		classificationOverride := resolveClassificationOverrides(raw, companyID, input.Manual.ClassificationOverrides)
		ruleSector, ruleIndustry := ruleClassification(raw, category, structureFlags, name)
		sector := firstNonEmpty(classificationOverride.Sector, tickerOverride.Sector, companyOverride.Sector, profile.Sector, ruleSector)
		industry := firstNonEmpty(classificationOverride.Industry, tickerOverride.Industry, companyOverride.Industry, profile.Industry, ruleIndustry)
		country := firstNonEmpty(classificationOverride.Country, tickerOverride.Country, companyOverride.Country, profile.Country)
		yahooSymbol := firstNonEmpty(tickerOverride.YahooSymbol, profile.Symbol)
		marketCap := firstNonZero(tickerOverride.MarketCap, profile.MarketCap)
		companyName := firstNonEmpty(companyOverride.Name, tickerOverride.Name, profile.Name, raw.Name, raw.ShortName, raw.Ticker)
		lastReviewed := firstNonEmpty(classificationOverride.LastReviewed, tickerOverride.LastReviewed, companyOverride.LastReviewed)
		identitySources := identityOverride.Sources()

		exchange := exchanges[raw.WorkingScheduleID]
		listingID := raw.Ticker
		listingExchangeName := firstNonEmpty(tickerOverride.Exchange, exchange.Name, profile.Exchange)
		listingCurrencyCode := firstNonEmpty(tickerOverride.Currency, raw.CurrencyCode, profile.Currency)
		listing := &Listing{
			ID:           listingID,
			Ticker:       raw.Ticker,
			SecurityID:   securityID,
			CompanyID:    companyID,
			ExchangeCode: parts.ExchangeCode,
			ExchangeName: listingExchangeName,
			CurrencyCode: listingCurrencyCode,
		}
		listingByID[listingID] = listing

		security := securityByID[securityID]
		securityConfidence, securityReasons := securityIdentity(raw, identityOverride)
		if security == nil {
			security = &Security{
				ID:                 securityID,
				ISIN:               raw.ISIN,
				Name:               name,
				Type:               raw.Type,
				InstrumentCategory: category,
				StructureFlags:     structureFlags,
				CompanyID:          companyID,
				IdentityConfidence: securityConfidence,
				IdentityReasons:    append([]string(nil), securityReasons...),
			}
			securityByID[securityID] = security
		}
		if security.CompanyID != companyID {
			state.addIssue(IdentityIssue{
				IssueCode:       "security_id_multiple_companies",
				Ticker:          raw.Ticker,
				ISIN:            raw.ISIN,
				SecurityID:      securityID,
				CompanyID:       companyID,
				Name:            name,
				Reason:          fmt.Sprintf("security already belongs to company_id %q but ticker maps to %q", security.CompanyID, companyID),
				SuggestedAction: "split the security with identity_overrides.csv or force one company mapping manually",
			})
		}
		if security.InstrumentCategory != "" && security.InstrumentCategory != category {
			state.addIssue(IdentityIssue{
				IssueCode:       "security_category_collision",
				Ticker:          raw.Ticker,
				ISIN:            raw.ISIN,
				SecurityID:      securityID,
				CompanyID:       companyID,
				Name:            name,
				Reason:          fmt.Sprintf("security already has category %q but ticker classified as %q", security.InstrumentCategory, category),
				SuggestedAction: "review the ISIN grouping or add an identity override",
			})
		}
		security.ListingIDs = appendUnique(security.ListingIDs, listingID)
		security.TickerIDs = appendUnique(security.TickerIDs, raw.Ticker)
		security.CurrencySet = appendUnique(security.CurrencySet, listingCurrencyCode)
		security.StructureFlags = mergeFlags(security.StructureFlags, structureFlags)
		security.IdentityConfidence = lowerConfidence(security.IdentityConfidence, securityConfidence)
		security.IdentityReasons = appendUniqueMany(security.IdentityReasons, securityReasons...)

		company := companyByID[companyID]
		if company == nil {
			company = &Company{
				ID:                 companyID,
				Name:               companyName,
				PrimaryTicker:      raw.Ticker,
				IdentityConfidence: companyConfidence,
				IdentityReasons:    append([]string(nil), companyReasons...),
			}
			companyByID[companyID] = company
		}
		company.Name = firstNonEmpty(company.Name, companyName)
		company.Sector = firstNonEmpty(company.Sector, sector)
		company.Industry = firstNonEmpty(company.Industry, industry)
		company.Country = firstNonEmpty(company.Country, country)
		company.YahooSymbol = firstNonEmpty(company.YahooSymbol, yahooSymbol)
		if company.MarketCap == 0 {
			company.MarketCap = marketCap
		}
		company.SecurityIDs = appendUnique(company.SecurityIDs, securityID)
		company.ListingIDs = appendUnique(company.ListingIDs, listingID)
		company.TickerIDs = appendUnique(company.TickerIDs, raw.Ticker)
		company.LastReviewed = firstNonEmpty(company.LastReviewed, lastReviewed)
		company.LastRefreshed = input.BuiltAt.Format(time.RFC3339)
		company.Sources = appendSources(company.Sources, append(append(identitySources, classificationOverride.Sources()...), sourceFromOverride(tickerOverride), sourceFromCompanyOverride(companyOverride), sourceFromProfile(profile))...)
		company.IdentityConfidence = lowerConfidence(company.IdentityConfidence, companyConfidence)
		company.IdentityReasons = appendUniqueMany(company.IdentityReasons, companyReasons...)

		identityReasons := []string{}
		if parts.Uncertain {
			identityReasons = append(identityReasons, "broker_ticker_parse_"+parts.Reason)
			state.addIssue(IdentityIssue{
				IssueCode:       "broker_ticker_parse_uncertain",
				Ticker:          raw.Ticker,
				ISIN:            raw.ISIN,
				SecurityID:      securityID,
				CompanyID:       companyID,
				Name:            name,
				Reason:          parts.Reason,
				SuggestedAction: "review whether the broker ticker suffix should be handled by the parser",
			})
		} else {
			identityReasons = append(identityReasons, "broker_ticker_parsed")
		}
		if raw.ISIN == "" {
			identityReasons = append(identityReasons, "missing_isin_fallback_to_ticker")
			state.addIssue(IdentityIssue{
				IssueCode:       "missing_isin",
				Ticker:          raw.Ticker,
				SecurityID:      securityID,
				CompanyID:       companyID,
				Name:            name,
				Reason:          "security_id falls back to broker ticker because ISIN is missing",
				SuggestedAction: "add a manual identity override if the ticker should merge with another security",
			})
		}
		if category == CategoryOther {
			state.addIssue(IdentityIssue{
				IssueCode:       "unknown_instrument_category",
				Ticker:          raw.Ticker,
				ISIN:            raw.ISIN,
				SecurityID:      securityID,
				CompanyID:       companyID,
				Name:            name,
				Reason:          "Trading 212 type did not map to a normalized instrument category",
				SuggestedAction: "add category to identity_overrides.csv if this instrument should be classified",
			})
		}
		if companyConfidence == "rule_low" {
			state.addIssue(IdentityIssue{
				IssueCode:       "low_confidence_company_identity",
				Ticker:          raw.Ticker,
				ISIN:            raw.ISIN,
				SecurityID:      securityID,
				CompanyID:       companyID,
				Name:            name,
				Reason:          strings.Join(companyReasons, "; "),
				SuggestedAction: "review company identity and add override_company_id when appropriate",
			})
		}
		identityReasons = append(identityReasons, securityReasons...)
		identityReasons = append(identityReasons, companyReasons...)
		identityReasons = append(identityReasons, categoryReasons...)
		identityReasons = append(identityReasons, flagReasons...)
		identityReasons = append(identityReasons, identityOverride.Reasons...)
		tickerConfidence := lowerConfidence(securityConfidence, companyConfidence, identityOverride.Confidence)

		ticker := &Ticker{
			Ticker:             raw.Ticker,
			BrokerSymbol:       parts.Symbol,
			BrokerAssetCode:    parts.AssetCode,
			Name:               name,
			ShortName:          raw.ShortName,
			Type:               raw.Type,
			InstrumentCategory: category,
			StructureFlags:     structureFlags,
			CurrencyCode:       listingCurrencyCode,
			ISIN:               raw.ISIN,
			ExchangeCode:       parts.ExchangeCode,
			ExchangeName:       listing.ExchangeName,
			WorkingScheduleID:  raw.WorkingScheduleID,
			MaxOpenQuantity:    raw.MaxOpenQuantity,
			ExtendedHours:      raw.ExtendedHours,
			SecurityID:         securityID,
			CompanyID:          companyID,
			ListingID:          listingID,
			Directionality:     DetectDirectionality(raw.Ticker, raw.Name, raw.ShortName),
			YahooSymbol:        yahooSymbol,
			Sector:             sector,
			Industry:           industry,
			Country:            country,
			MarketCap:          marketCap,
			Sources:            appendSources(nil, append(append(identitySources, classificationOverride.Sources()...), sourceFromOverride(tickerOverride), sourceFromCompanyOverride(companyOverride), sourceFromProfile(profile))...),
			IdentityConfidence: tickerConfidence,
			IdentityReasons:    appendUniqueMany(nil, identityReasons...),
			LastReviewed:       lastReviewed,
			LastRefreshed:      input.BuiltAt.Format(time.RFC3339),
		}
		tickerByID[raw.Ticker] = ticker
	}

	addIdentityGroupIssues(tickersByISIN, companiesByISIN, isinsBySecurity, companiesBySecurity, categoriesBySecurity, state)
	addUnknownOverrideIssues(input.Manual.IdentityOverrides, state)
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
	identityIssues := sortedIdentityIssues(state.issues)
	sectors := groupTickers(tickers, func(t *Ticker) string { return t.Sector })
	industries := groupTickers(tickers, func(t *Ticker) string { return t.Industry })
	reviewQueues := BuildReviewQueues(ReviewInput{
		Tickers:            derefTickers(tickers),
		Unclassified:       unclassified,
		IdentityIssues:     identityIssues,
		EnrichmentFailures: input.EnrichmentFailures,
		Manual:             input.Manual,
		BuiltAt:            input.BuiltAt,
	})
	reviewSummary := BuildReviewSummary(reviewQueues, input.BuiltAt)

	cat := &Catalogue{
		DataContractVersion: DataContractVersion,
		SchemaVersion:       DataContractSchemaVersion,
		GeneratedAt:         input.BuiltAt.UTC().Format(time.RFC3339),
		Tickers:             derefTickers(tickers),
		Securities:          derefSecurities(securities),
		Listings:            derefListings(listings),
		Companies:           derefCompanies(companies),
		Sectors:             sectors,
		Industries:          industries,
		Themes:              input.Manual.Themes,
		SupplyChains:        input.Manual.SupplyChains,
		Exposures:           input.Manual.Exposures,
		Relationships:       sortedRelationships(input.Manual.Relationships),
		Notes:               input.Manual.Notes,
		Unclassified:        unclassified,
		ReviewQueues:        reviewQueues,
		ReviewSummary:       reviewSummary,
		IdentityIssues:      identityIssues,
		EnrichmentFailures:  append([]EnrichmentFailure(nil), input.EnrichmentFailures...),
	}
	cat.Manifest = BuildManifest{
		DataContractVersion:          DataContractVersion,
		SchemaVersion:                DataContractSchemaVersion,
		BuiltAt:                      input.BuiltAt.UTC().Format(time.RFC3339),
		SourceMode:                   input.SourceMode,
		Trading212Environment:        input.Trading212Environment,
		Trading212BaseURL:            input.Trading212BaseURL,
		Trading212FetchAt:            input.Trading212FetchAt,
		InstrumentCount:              len(input.Instruments),
		ExchangeCount:                len(input.Exchanges),
		SecurityCount:                len(cat.Securities),
		CompanyCount:                 len(cat.Companies),
		ListingCount:                 len(cat.Listings),
		ThemeCount:                   len(cat.Themes),
		ExposureCount:                len(cat.Exposures),
		RelationshipCount:            len(cat.Relationships),
		EnrichmentAttempted:          input.EnrichmentAttempted,
		EnrichmentSucceeded:          input.EnrichmentSucceeded,
		EnrichmentFailed:             input.EnrichmentFailed,
		EnrichmentCacheSchemaVersion: input.EnrichmentDiagnostics.CacheSchemaVersion,
		EnrichmentProvider:           input.EnrichmentDiagnostics.Provider,
		EnrichmentCacheHitCount:      input.EnrichmentDiagnostics.CacheHitCount,
		EnrichmentCacheMissCount:     input.EnrichmentDiagnostics.CacheMissCount,
		EnrichmentCacheStaleCount:    input.EnrichmentDiagnostics.CacheStaleCount,
		EnrichmentAmbiguousCount:     input.EnrichmentDiagnostics.AmbiguousCount,
		EnrichmentFailureCount:       input.EnrichmentDiagnostics.FailureCount,
		EnrichmentFailureCSV:         input.EnrichmentDiagnostics.FailureCSV,
		EnrichmentOldestRetrievedAt:  input.EnrichmentDiagnostics.OldestRetrievedAt,
		EnrichmentNewestRetrievedAt:  input.EnrichmentDiagnostics.NewestRetrievedAt,
		UnclassifiedCount:            len(cat.Unclassified),
		EmptyTickerCount:             state.emptyTickerCount,
		DuplicateTickerCount:         state.duplicateTickerCount,
		DuplicateISINCount:           duplicateISINCount(tickersByISIN),
		MissingISINCount:             state.missingISINCount,
		IdentityCollisionCount:       identityCollisionCount(identityIssues),
		IdentityOverrideCount:        len(state.matchedOverrideKeys),
		IdentityIssueCount:           len(identityIssues),
		ReviewQueueCounts:            reviewSummary.ByQueue,
		ReviewReasonCounts:           reviewSummary.ByReasonCode,
		InstrumentCategoryCounts:     state.categoryCounts,
		StructureFlagCounts:          state.flagCounts,
		RawSnapshotAt:                input.RawSnapshotAt,
		RawSnapshots:                 input.RawSnapshots,
		Trading212HTTPDiagnostics:    input.HTTPDiagnostics,
		Trading212RateLimits:         input.RateLimits,
		DataFreshness:                freshness(input.RawSnapshotAt, input.BuiltAt),
	}
	addReviewManifestDeltas(&cat.Manifest, input.PreviousManifest)
	return cat, nil
}

func ruleClassification(raw trading212.Instrument, category string, structureFlags []string, name string) (string, string) {
	text := normaliseIdentityText(raw.Ticker, raw.Name, raw.ShortName, name)
	switch category {
	case CategoryETF:
		return "Funds", fundLikeIndustry(text, structureFlags, "Equity ETF")
	case CategoryFund:
		return "Funds", fundLikeIndustry(text, structureFlags, "Fund")
	case CategoryInvestmentTrust:
		return "Funds", "Investment Trust"
	case CategoryWarrant:
		return "Structured Products", "Warrant"
	default:
		return "", ""
	}
}

func fundLikeIndustry(text string, structureFlags []string, defaultIndustry string) string {
	switch {
	case containsString(structureFlags, "inverse") || containsString(structureFlags, "short"):
		return "Inverse ETP"
	case containsString(structureFlags, "leveraged"):
		return "Leveraged ETP"
	case containsAnyToken(text, "YIELDMAX", "INCOMESHARES") || strings.Contains(text, " COVERED CALL ") || strings.Contains(text, " OPTION INCOME "):
		return "Covered Call ETF"
	case containsAnyToken(text, "BITCOIN", "ETHEREUM", "CRYPTO", "CRYPTOCURRENCY"):
		return "Crypto ETP"
	case containsAnyToken(text, "COMMODITY", "COMMODITIES", "GOLD", "SILVER", "PRECIOUS", "PLATINUM", "PALLADIUM", "COPPER", "URANIUM", "OIL", "GAS", "WHEAT", "AGRICULTURE"):
		return "Commodity ETP"
	case containsAnyToken(text, "BOND", "BONDS", "TREASURY", "GILT", "GOVERNMENT", "AGGREGATE", "DEBT", "CREDIT"):
		return "Bond ETF"
	case strings.Contains(text, " MONEY MARKET ") || containsAnyToken(text, "CASH"):
		return "Money Market Fund"
	case strings.Contains(text, " MULTI ASSET ") || strings.Contains(text, " MULTI-ASSET "):
		return "Multi-Asset Fund"
	case containsAnyToken(text, "QUALITY", "VALUE", "MOMENTUM", "DIVIDEND") || strings.Contains(text, " EQUAL WEIGHT ") || strings.Contains(text, " MINIMUM VOLATILITY "):
		return "Factor ETF"
	default:
		return defaultIndustry
	}
}

func sortedRelationships(rows []taxonomy.Relationship) []taxonomy.Relationship {
	out := make([]taxonomy.Relationship, 0, len(rows))
	out = append(out, rows...)
	sort.SliceStable(out, func(i, j int) bool {
		return relationshipSortKey(out[i]) < relationshipSortKey(out[j])
	})
	return out
}

func relationshipSortKey(row taxonomy.Relationship) string {
	return strings.Join([]string{
		row.RelationshipType,
		row.SourceTicker,
		row.SourceISIN,
		row.SourceCompanyID,
		row.TargetTicker,
		row.TargetISIN,
		row.TargetCompanyID,
		row.ThemeID,
		row.LayerID,
		row.Confidence,
		row.SourceURL,
		row.Rationale,
		row.LastReviewed,
	}, "\x00")
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

type identityBuildState struct {
	issues               []IdentityIssue
	emptyTickerCount     int
	duplicateTickerCount int
	missingISINCount     int
	categoryCounts       map[string]int
	flagCounts           map[string]int
	matchedOverrideKeys  map[string]bool
}

func newIdentityBuildState() *identityBuildState {
	return &identityBuildState{
		categoryCounts:      map[string]int{},
		flagCounts:          map[string]int{},
		matchedOverrideKeys: map[string]bool{},
	}
}

func (state *identityBuildState) addIssue(issue IdentityIssue) {
	state.issues = append(state.issues, issue)
}

type resolvedIdentityOverride struct {
	SecurityID string
	CompanyID  string
	Category   string
	Flags      []string
	Confidence string
	Reasons    []string
	sources    []Source
}

func (override resolvedIdentityOverride) Sources() []Source {
	return override.sources
}

type resolvedClassificationOverride struct {
	Sector       string
	Industry     string
	Country      string
	LastReviewed string
	sources      []Source
}

func (override resolvedClassificationOverride) Sources() []Source {
	return override.sources
}

func resolveClassificationOverrides(raw trading212.Instrument, companyID string, overrides []taxonomy.ClassificationOverride) resolvedClassificationOverride {
	resolved := resolvedClassificationOverride{}
	// Apply broad rows first and specific rows last inside the dedicated classification file.
	for _, targetType := range []string{"company", "isin", "ticker"} {
		for _, override := range overrides {
			if override.TargetType != targetType {
				continue
			}
			switch override.TargetType {
			case "company":
				if override.CompanyID != companyID {
					continue
				}
			case "isin":
				if override.ISIN == "" || override.ISIN != raw.ISIN {
					continue
				}
			case "ticker":
				if override.Ticker != raw.Ticker {
					continue
				}
			}
			if override.Sector != "" {
				resolved.Sector = override.Sector
			}
			if override.Industry != "" {
				resolved.Industry = override.Industry
			}
			if override.Country != "" {
				resolved.Country = override.Country
			}
			if override.LastReviewed != "" {
				resolved.LastReviewed = override.LastReviewed
			}
			resolved.sources = appendSources(resolved.sources, sourceFromClassificationOverride(override))
		}
	}
	return resolved
}

func resolveIdentityOverrides(raw trading212.Instrument, baseSecurityID string, companyID string, overrides []taxonomy.IdentityOverride, state *identityBuildState) resolvedIdentityOverride {
	matches := matchingIdentityOverrides(raw, baseSecurityID, companyID, overrides)
	resolved := resolvedIdentityOverride{}
	for _, override := range matches {
		key := catalogueIdentityOverrideKey(override)
		state.matchedOverrideKeys[key] = true
		if override.OverrideSecurityID != "" {
			if resolved.SecurityID != "" && resolved.SecurityID != override.OverrideSecurityID {
				state.addIssue(identityOverrideConflictIssue(raw, baseSecurityID, companyID, "override_security_id", resolved.SecurityID, override.OverrideSecurityID))
			}
			resolved.SecurityID = override.OverrideSecurityID
			resolved.Reasons = appendUnique(resolved.Reasons, "security_id_from_identity_override")
		}
		if override.OverrideCompanyID != "" {
			if resolved.CompanyID != "" && resolved.CompanyID != override.OverrideCompanyID {
				state.addIssue(identityOverrideConflictIssue(raw, baseSecurityID, companyID, "override_company_id", resolved.CompanyID, override.OverrideCompanyID))
			}
			resolved.CompanyID = override.OverrideCompanyID
			resolved.Reasons = appendUnique(resolved.Reasons, "company_id_from_identity_override")
		}
		if override.Category != "" {
			if resolved.Category != "" && resolved.Category != override.Category {
				state.addIssue(identityOverrideConflictIssue(raw, baseSecurityID, companyID, "category", resolved.Category, override.Category))
			}
			resolved.Category = override.Category
			resolved.Reasons = appendUnique(resolved.Reasons, "category_from_identity_override")
		}
		if len(override.Flags) > 0 {
			resolved.Flags = mergeFlags(resolved.Flags, override.Flags)
			resolved.Reasons = appendUnique(resolved.Reasons, "structure_flags_from_identity_override")
		}
		if override.Confidence != "" {
			if resolved.Confidence != "" && resolved.Confidence != override.Confidence {
				state.addIssue(identityOverrideConflictIssue(raw, baseSecurityID, companyID, "confidence", resolved.Confidence, override.Confidence))
			}
			resolved.Confidence = override.Confidence
		}
		if override.Reason != "" {
			resolved.Reasons = appendUnique(resolved.Reasons, "manual_identity_reason:"+override.Reason)
		}
		if override.SourceURL != "" {
			resolved.sources = appendSources(resolved.sources, Source{
				Kind:         "manual_identity_override",
				URL:          override.SourceURL,
				Label:        "Identity override",
				LastReviewed: override.LastReviewed,
			})
		}
	}
	return resolved
}

func matchingIdentityOverrides(raw trading212.Instrument, baseSecurityID string, companyID string, overrides []taxonomy.IdentityOverride) []taxonomy.IdentityOverride {
	precedence := []string{"company", "security", "isin", "ticker"}
	out := []taxonomy.IdentityOverride{}
	for _, targetType := range precedence {
		for _, override := range overrides {
			if override.TargetType != targetType {
				continue
			}
			switch override.TargetType {
			case "ticker":
				if override.Ticker == raw.Ticker {
					out = append(out, override)
				}
			case "isin":
				if override.ISIN != "" && override.ISIN == raw.ISIN {
					out = append(out, override)
				}
			case "security":
				if override.SecurityID == baseSecurityID {
					out = append(out, override)
				}
			case "company":
				if override.CompanyID == companyID {
					out = append(out, override)
				}
			}
		}
	}
	return out
}

func identityOverrideConflictIssue(raw trading212.Instrument, securityID string, companyID string, field string, oldValue string, newValue string) IdentityIssue {
	return IdentityIssue{
		IssueCode:       "manual_override_conflict",
		Ticker:          raw.Ticker,
		ISIN:            raw.ISIN,
		SecurityID:      securityID,
		CompanyID:       companyID,
		Name:            firstNonEmpty(raw.Name, raw.ShortName, raw.Ticker),
		Reason:          fmt.Sprintf("identity overrides set conflicting %s values %q and %q; the more specific later override won", field, oldValue, newValue),
		SuggestedAction: "remove the conflict or make the intended override target more specific",
	}
}

func securityIdentity(raw trading212.Instrument, override resolvedIdentityOverride) (string, []string) {
	if override.SecurityID != "" {
		return firstNonEmpty(override.Confidence, "manual_high"), []string{"security_id_from_identity_override"}
	}
	if raw.ISIN != "" {
		return "rule_high", []string{"security_id_from_isin"}
	}
	return "rule_low", []string{"security_id_from_missing_isin_ticker_fallback"}
}

func addIdentityGroupIssues(tickersByISIN map[string][]string, companiesByISIN map[string][]string, isinsBySecurity map[string][]string, companiesBySecurity map[string][]string, categoriesBySecurity map[string][]string, state *identityBuildState) {
	for isin, companies := range companiesByISIN {
		if len(companies) <= 1 {
			continue
		}
		state.addIssue(IdentityIssue{
			IssueCode:       "shared_isin_multiple_companies",
			Ticker:          strings.Join(sortedStringSet(tickersByISIN[isin]), ";"),
			ISIN:            isin,
			CompanyID:       strings.Join(sortedStringSet(companies), ";"),
			Reason:          "one ISIN maps to multiple company IDs",
			SuggestedAction: "review whether the ISIN should be split with override_security_id or mapped to one company",
		})
	}
	for securityID, isins := range isinsBySecurity {
		nonEmptyISINs := []string{}
		for _, isin := range isins {
			if isin != "" {
				nonEmptyISINs = appendUnique(nonEmptyISINs, isin)
			}
		}
		if len(nonEmptyISINs) > 1 {
			state.addIssue(IdentityIssue{
				IssueCode:       "security_id_multiple_isins",
				ISIN:            strings.Join(sortedStringSet(nonEmptyISINs), ";"),
				SecurityID:      securityID,
				Reason:          "one security ID contains multiple ISINs",
				SuggestedAction: "confirm the manual merge is intentional or split the securities",
			})
		}
		if len(companiesBySecurity[securityID]) > 1 {
			state.addIssue(IdentityIssue{
				IssueCode:       "security_id_multiple_companies",
				SecurityID:      securityID,
				CompanyID:       strings.Join(sortedStringSet(companiesBySecurity[securityID]), ";"),
				Reason:          "one security ID maps to multiple companies",
				SuggestedAction: "split the security or force one company mapping manually",
			})
		}
		if len(categoriesBySecurity[securityID]) > 1 {
			state.addIssue(IdentityIssue{
				IssueCode:       "security_category_collision",
				SecurityID:      securityID,
				Reason:          "one security ID maps to multiple instrument categories: " + strings.Join(sortedStringSet(categoriesBySecurity[securityID]), ";"),
				SuggestedAction: "review the grouping and add an identity override if needed",
			})
		}
	}
}

func addUnknownOverrideIssues(overrides []taxonomy.IdentityOverride, state *identityBuildState) {
	for _, override := range overrides {
		key := catalogueIdentityOverrideKey(override)
		if state.matchedOverrideKeys[key] {
			continue
		}
		issue := IdentityIssue{
			IssueCode:       "manual_override_unknown_" + override.TargetType,
			Ticker:          override.Ticker,
			ISIN:            override.ISIN,
			SecurityID:      override.SecurityID,
			CompanyID:       override.CompanyID,
			Reason:          "manual identity override did not match any Trading 212 instrument in this build",
			SuggestedAction: "remove the row, correct the target, or keep it only if the instrument is expected to reappear",
		}
		if override.TargetType == "ticker" {
			issue.IssueCode = "manual_override_unknown_ticker"
		}
		state.addIssue(issue)
	}
}

func catalogueIdentityOverrideKey(override taxonomy.IdentityOverride) string {
	switch override.TargetType {
	case "ticker":
		return "ticker:" + override.Ticker
	case "isin":
		return "isin:" + override.ISIN
	case "security":
		return "security:" + override.SecurityID
	case "company":
		return "company:" + override.CompanyID
	default:
		return override.TargetType + ":"
	}
}

func duplicateISINCount(tickersByISIN map[string][]string) int {
	count := 0
	for _, tickers := range tickersByISIN {
		if len(tickers) > 1 {
			count++
		}
	}
	return count
}

func identityCollisionCount(issues []IdentityIssue) int {
	count := 0
	for _, issue := range issues {
		switch issue.IssueCode {
		case "duplicate_ticker", "shared_isin_multiple_companies", "security_id_multiple_isins", "security_id_multiple_companies", "security_category_collision", "manual_override_conflict":
			count++
		}
	}
	return count
}

func sortedIdentityIssues(issues []IdentityIssue) []IdentityIssue {
	out := append([]IdentityIssue(nil), issues...)
	sort.SliceStable(out, func(i, j int) bool {
		a := out[i].IssueCode + "|" + out[i].Ticker + "|" + out[i].ISIN + "|" + out[i].SecurityID + "|" + out[i].CompanyID + "|" + out[i].Reason
		b := out[j].IssueCode + "|" + out[j].Ticker + "|" + out[j].ISIN + "|" + out[j].SecurityID + "|" + out[j].CompanyID + "|" + out[j].Reason
		return a < b
	})
	return out
}

func lowerConfidence(values ...string) string {
	out := ""
	for _, value := range values {
		if value == "" {
			continue
		}
		if out == "" || confidenceRank(value) < confidenceRank(out) {
			out = value
		}
	}
	return out
}

func confidenceRank(value string) int {
	switch value {
	case "rule_low":
		return 1
	case "manual_low":
		return 2
	case "rule_medium":
		return 3
	case "manual_medium":
		return 4
	case "rule_high":
		return 5
	case "manual_high":
		return 6
	default:
		return 0
	}
}

func appendUniqueMany(values []string, additions ...string) []string {
	for _, value := range additions {
		values = appendUnique(values, value)
	}
	return values
}

func applyExposures(exposures []taxonomy.Exposure, tickers map[string]*Ticker, companies map[string]*Company, securityByISIN map[string][]string) {
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
			for _, securityID := range securityByISIN[exposure.ISIN] {
				for _, ticker := range tickers {
					if ticker.SecurityID == securityID {
						ids = append(ids, ticker.CompanyID)
					}
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

const maxRelatedTickersPerIndustry = 50

func addRelatedTickers(tickers map[string]*Ticker, companies map[string]*Company) {
	byIndustry := map[string][]string{}
	for _, ticker := range tickers {
		if ticker.Industry != "" {
			byIndustry[ticker.Industry] = append(byIndustry[ticker.Industry], ticker.Ticker)
		}
	}
	for industry, tickerIDs := range byIndustry {
		sort.Strings(tickerIDs)
		if len(tickerIDs) > maxRelatedTickersPerIndustry {
			tickerIDs = tickerIDs[:maxRelatedTickersPerIndustry]
		}
		byIndustry[industry] = tickerIDs
	}
	for _, company := range companies {
		related := []string{}
		ownTickers := append([]string(nil), company.TickerIDs...)
		sort.Strings(ownTickers)
		related = append(related, ownTickers...)
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
		if len(ticker.ThemeIDs) == 0 && shouldReviewMissingThemeExposure(ticker) {
			reasons = append(reasons, "missing theme exposure")
		}
		if len(reasons) == 0 {
			continue
		}
		out = append(out, UnclassifiedRow{
			Ticker:      ticker.Ticker,
			CompanyID:   ticker.CompanyID,
			Name:        ticker.Name,
			ISIN:        ticker.ISIN,
			Reason:      strings.Join(reasons, "; "),
			ReasonCodes: ReasonCodesForUnclassifiedReason(strings.Join(reasons, "; ")),
		})
	}
	return out
}

func isUnclassified(ticker *Ticker) bool {
	return ticker.Sector == "" || ticker.Industry == "" || (len(ticker.ThemeIDs) == 0 && shouldReviewMissingThemeExposure(ticker))
}

func shouldReviewMissingThemeExposure(ticker *Ticker) bool {
	if ticker == nil || len(ticker.ThemeIDs) > 0 {
		return false
	}
	if ticker.InstrumentCategory != CategoryStock || ticker.Sector == "" || ticker.Industry == "" {
		return false
	}
	industry := strings.TrimSpace(ticker.Industry)
	switch strings.TrimSpace(ticker.Sector) {
	case "Healthcare", "Energy":
		return true
	case "Technology":
		return industryMatchesAny(industry,
			"Communication Equipment",
			"Computer Hardware",
			"Electronic Components",
			"Information Technology Services",
			"Scientific & Technical Instruments",
			"Security & Protection Services",
			"Semiconductor Equipment",
			"Semiconductor Equipment & Materials",
			"Semiconductors",
			"Software - Application",
			"Software - Infrastructure",
			"Solar",
		)
	case "Financial Services":
		return industryMatchesAny(industry,
			"Banks - Diversified",
			"Banks - Regional",
			"Capital Markets",
			"Credit Services",
			"Financial Data & Stock Exchanges",
		)
	case "Basic Materials":
		return industryMatchesAny(industry,
			"Agricultural Inputs",
			"Aluminum",
			"Chemicals",
			"Coking Coal",
			"Copper",
			"Gold",
			"Lithium",
			"Other Industrial Metals & Mining",
			"Other Precious Metals & Mining",
			"Silver",
			"Specialty Chemicals",
			"Steel",
			"Thermal Coal",
			"Uranium",
		)
	case "Industrials":
		return industryMatchesAny(industry,
			"Aerospace & Defense",
			"Electrical Equipment",
			"Electrical Equipment & Parts",
			"Engineering & Construction",
			"Farm & Heavy Construction Machinery",
			"Industrial Distribution",
			"Infrastructure Operations",
			"Pollution & Treatment Controls",
			"Specialty Industrial Machinery",
		)
	case "Utilities":
		return industryMatchesAny(industry,
			"Independent Power Producers",
			"Utilities - Diversified",
			"Utilities - Independent Power Producers",
			"Utilities - Regulated Electric",
			"Utilities - Renewable",
		)
	default:
		return false
	}
}

func industryMatchesAny(industry string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.EqualFold(industry, candidate) {
			return true
		}
	}
	return false
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
	if override.SourceURL == "" && !hasTickerEnrichmentOverride(override) {
		return Source{}
	}
	return Source{Kind: "manual_ticker_override", URL: override.SourceURL, Label: "Ticker override", LastReviewed: override.LastReviewed}
}

func sourceFromClassificationOverride(override taxonomy.ClassificationOverride) Source {
	if override.SourceURL == "" && override.Sector == "" && override.Industry == "" && override.Country == "" {
		return Source{}
	}
	return Source{Kind: "manual_classification_override", URL: override.SourceURL, Label: "Classification override", LastReviewed: override.LastReviewed}
}

func hasTickerEnrichmentOverride(override taxonomy.TickerOverride) bool {
	return override.Name != "" ||
		override.Sector != "" ||
		override.Industry != "" ||
		override.Country != "" ||
		override.YahooSymbol != "" ||
		override.MarketCap != 0 ||
		override.Exchange != "" ||
		override.Currency != ""
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

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
