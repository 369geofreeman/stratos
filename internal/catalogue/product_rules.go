package catalogue

import (
	"fmt"
	"sort"
	"strings"

	"statos/internal/taxonomy"
)

const (
	themeFundsCore              = "funds_core"
	themeLeveragedStructured    = "leveraged_structured"
	themeCommodityCryptoETPs    = "commodity_crypto_etps"
	confidenceProductRule       = "rule_low"
	ruleProductExposureScore    = 5
	ruleRationaleTrading212Base = "Rule from Trading 212-derived instrument metadata"
)

type productRuleAssignment struct {
	themeID string
	layerID string
	reason  string
}

func combinedExposures(manual []taxonomy.Exposure, generated []taxonomy.Exposure) []taxonomy.Exposure {
	out := make([]taxonomy.Exposure, 0, len(manual)+len(generated))
	out = append(out, manual...)
	out = append(out, generated...)
	return out
}

func buildProductRuleExposures(tickers map[string]*Ticker, manual taxonomy.ManualData) []taxonomy.Exposure {
	validLayers := productRuleLayerSet(manual)
	if len(validLayers) == 0 {
		return nil
	}

	tickerIDs := make([]string, 0, len(tickers))
	for tickerID := range tickers {
		tickerIDs = append(tickerIDs, tickerID)
	}
	sort.Strings(tickerIDs)

	assignmentsByTicker := map[string][]productRuleAssignment{}
	assignmentKeysByISIN := map[string]map[string]bool{}
	for _, tickerID := range tickerIDs {
		ticker := tickers[tickerID]
		if ticker == nil {
			continue
		}
		assignments := []productRuleAssignment{}
		if isProductRuleCandidate(*ticker) {
			assignments = validProductRuleAssignments(*ticker, validLayers)
			assignmentsByTicker[tickerID] = assignments
		}
		if ticker.ISIN != "" {
			if assignmentKeysByISIN[ticker.ISIN] == nil {
				assignmentKeysByISIN[ticker.ISIN] = map[string]bool{}
			}
			assignmentKeysByISIN[ticker.ISIN][productRuleAssignmentSetKey(assignments)] = true
		}
	}
	conflictedISINs := map[string]bool{}
	for isin, keys := range assignmentKeysByISIN {
		if len(keys) > 1 {
			conflictedISINs[isin] = true
		}
	}

	seen := map[string]bool{}
	var out []taxonomy.Exposure
	for _, tickerID := range tickerIDs {
		ticker := tickers[tickerID]
		if ticker == nil || !isProductRuleCandidate(*ticker) {
			continue
		}
		for _, assignment := range assignmentsByTicker[tickerID] {
			exposure := productRuleExposure(*ticker, assignment, conflictedISINs[ticker.ISIN])
			key := strings.Join([]string{
				exposure.ThemeID,
				exposure.LayerID,
				exposure.Ticker,
				exposure.ISIN,
			}, "\x00")
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, exposure)
		}
	}
	return out
}

func productRuleLayerSet(manual taxonomy.ManualData) map[string]bool {
	themeIDs := map[string]bool{}
	for _, theme := range manual.Themes {
		themeIDs[theme.ID] = true
	}
	out := map[string]bool{}
	for _, chain := range manual.SupplyChains {
		if !themeIDs[chain.ThemeID] {
			continue
		}
		for _, layer := range chain.Layers {
			out[chain.ThemeID+"|"+layer.ID] = true
		}
	}
	return out
}

func validProductRuleAssignments(ticker Ticker, validLayers map[string]bool) []productRuleAssignment {
	var out []productRuleAssignment
	for _, assignment := range productRuleAssignments(ticker) {
		if validLayers[assignment.themeID+"|"+assignment.layerID] {
			out = append(out, assignment)
		}
	}
	return out
}

func productRuleAssignmentSetKey(assignments []productRuleAssignment) string {
	if len(assignments) == 0 {
		return "<none>"
	}
	keys := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		keys = append(keys, assignment.themeID+"|"+assignment.layerID)
	}
	sort.Strings(keys)
	return strings.Join(keys, ";")
}

func productRuleExposure(ticker Ticker, assignment productRuleAssignment, forceTickerTarget bool) taxonomy.Exposure {
	exposure := taxonomy.Exposure{
		ThemeID:       assignment.themeID,
		LayerID:       assignment.layerID,
		ExposureScore: ruleProductExposureScore,
		Confidence:    confidenceProductRule,
		Rationale:     fmt.Sprintf("%s: %s.", ruleRationaleTrading212Base, assignment.reason),
	}
	if ticker.ISIN != "" && !forceTickerTarget {
		exposure.ISIN = ticker.ISIN
	} else {
		exposure.Ticker = ticker.Ticker
	}
	return exposure
}

func isProductRuleCandidate(ticker Ticker) bool {
	switch strings.TrimSpace(ticker.Sector) {
	case "Funds":
		return isFundProductCategory(ticker.InstrumentCategory)
	case "Structured Products":
		return true
	default:
		return false
	}
}

func isFundProductCategory(category string) bool {
	switch category {
	case CategoryETF, CategoryFund, CategoryInvestmentTrust, CategoryCommodity, CategoryCrypto:
		return true
	default:
		return false
	}
}

func productRuleAssignments(ticker Ticker) []productRuleAssignment {
	var out []productRuleAssignment
	industry := strings.TrimSpace(ticker.Industry)
	flags := ticker.StructureFlags
	text := productRuleText(ticker)
	isLeveraged := industry == "Leveraged ETP" || containsString(flags, "leveraged") || ticker.Directionality == "leveraged_long"
	isInverse := industry == "Inverse ETP" || containsString(flags, "inverse") || containsString(flags, "short") || ticker.Directionality == "inverse_or_short"

	if layer, ok := coreFundLayer(industry); ok && !isLeveraged && !isInverse {
		out = append(out, productRuleAssignment{
			themeID: themeFundsCore,
			layerID: layer,
			reason:  industry + " maps to core fund product layer",
		})
	}

	switch {
	case isInverse && isLeveraged:
		out = append(out,
			productRuleAssignment{
				themeID: themeLeveragedStructured,
				layerID: "inverse_etps",
				reason:  "inverse or short product metadata",
			},
			productRuleAssignment{
				themeID: themeLeveragedStructured,
				layerID: "leveraged_inverse_etps",
				reason:  "combined leveraged and inverse product metadata",
			},
		)
	case isInverse:
		out = append(out, productRuleAssignment{
			themeID: themeLeveragedStructured,
			layerID: "inverse_etps",
			reason:  "inverse or short product metadata",
		})
	case isLeveraged:
		out = append(out, productRuleAssignment{
			themeID: themeLeveragedStructured,
			layerID: "leveraged_etps",
			reason:  "leveraged product metadata",
		})
	}

	if ticker.InstrumentCategory == CategoryWarrant || industry == "Warrant" {
		out = append(out,
			productRuleAssignment{
				themeID: themeLeveragedStructured,
				layerID: "warrants",
				reason:  "warrant instrument category or industry",
			},
			productRuleAssignment{
				themeID: themeLeveragedStructured,
				layerID: "structured_products",
				reason:  "warrant instrument category or industry",
			},
		)
	} else if strings.TrimSpace(ticker.Sector) == "Structured Products" {
		out = append(out, productRuleAssignment{
			themeID: themeLeveragedStructured,
			layerID: "structured_products",
			reason:  "structured product sector",
		})
	}
	if isLeveraged || isInverse {
		out = append(out, productRuleAssignment{
			themeID: themeLeveragedStructured,
			layerID: "complex_payoff_products",
			reason:  "leveraged, inverse, or short exchange-traded product metadata",
		})
	}

	isFundSector := strings.TrimSpace(ticker.Sector) == "Funds"
	if isFundSector && (industry == "Crypto ETP" || ticker.InstrumentCategory == CategoryCrypto || ((isLeveraged || isInverse) && productHasCryptoMarker(text))) {
		out = append(out, productRuleAssignment{
			themeID: themeCommodityCryptoETPs,
			layerID: "crypto_etps",
			reason:  "crypto exchange-traded product metadata",
		})
	} else if isFundSector && (industry == "Commodity ETP" || ((isLeveraged || isInverse) && productHasCommodityMarker(text))) {
		layer := commodityProductLayer(text)
		if layer == "" {
			layer = "broad_commodity_etps"
		}
		out = append(out, productRuleAssignment{
			themeID: themeCommodityCryptoETPs,
			layerID: layer,
			reason:  "commodity exchange-traded product metadata",
		})
	}

	return out
}

func coreFundLayer(industry string) (string, bool) {
	switch industry {
	case "Equity ETF":
		return "equity_etfs", true
	case "Bond ETF":
		return "bond_etfs", true
	case "Factor ETF":
		return "factor_etfs", true
	case "Covered Call ETF":
		return "covered_call_etfs", true
	case "Money Market Fund":
		return "money_market_funds", true
	case "Multi-Asset Fund":
		return "multi_asset_funds", true
	case "Investment Trust":
		return "investment_trusts", true
	default:
		return "", false
	}
}

func productRuleText(ticker Ticker) string {
	return normaliseIdentityText(ticker.Ticker, ticker.Name, ticker.ShortName, ticker.Industry, ticker.InstrumentCategory)
}

func productHasCryptoMarker(text string) bool {
	return containsAnyToken(text,
		"BITCOIN",
		"ETHEREUM",
		"ETHER",
		"CRYPTO",
		"CRYPTOCURRENCY",
	)
}

func productHasCommodityMarker(text string) bool {
	return commodityProductLayer(text) != ""
}

func commodityProductLayer(text string) string {
	switch {
	case containsAnyToken(text, "GOLD"):
		return "gold_etps"
	case containsAnyToken(text, "SILVER"):
		return "silver_etps"
	case containsAnyToken(text, "PRECIOUS", "PLATINUM", "PALLADIUM"):
		return "precious_metals_etps"
	case containsAnyToken(text, "OIL", "BRENT", "WTI", "GAS", "CARBON", "URANIUM", "ENERGY", "PETROLEUM", "GASOLINE"):
		return "energy_commodity_etps"
	case containsAnyToken(text, "AGRICULTURE", "AGRICULTURAL", "WHEAT", "CORN", "SOY", "SOYBEAN", "SOYBEANS", "SUGAR", "COFFEE", "COCOA", "COTTON", "CATTLE", "LIVESTOCK", "GRAIN", "GRAINS"):
		return "agriculture_commodity_etps"
	case containsAnyToken(text, "COPPER", "ALUMINIUM", "ALUMINUM", "NICKEL", "ZINC", "TIN", "LEAD", "INDUSTRIAL", "BATTERY"):
		return "industrial_metals_etps"
	case containsAnyToken(text, "BROAD", "BASKET", "COMMODITY", "COMMODITIES", "DIVERSIFIED", "CMCI", "GSCI", "BLOOMBERG"):
		return "broad_commodity_etps"
	default:
		return ""
	}
}
