package catalogue

import (
	"sort"
	"strings"
	"unicode"

	"statos/internal/trading212"
)

const (
	CategoryStock           = "stock"
	CategoryETF             = "etf"
	CategoryFund            = "fund"
	CategoryInvestmentTrust = "investment_trust"
	CategoryWarrant         = "warrant"
	CategoryCrypto          = "crypto"
	CategoryForex           = "forex"
	CategoryBond            = "bond"
	CategoryCommodity       = "commodity"
	CategoryOther           = "other"
)

var structureFlagOrder = []string{
	"inverse",
	"short",
	"leveraged",
	"synthetic",
	"hedged",
	"accumulating",
	"distributing",
	"adr",
	"gdr",
	"fund_like",
}

type BrokerTickerParts struct {
	Symbol       string
	ExchangeCode string
	AssetCode    string
	Parsed       bool
	Uncertain    bool
	Reason       string
}

func ParseBrokerTicker(ticker string) BrokerTickerParts {
	trimmed := strings.TrimSpace(ticker)
	if trimmed == "" {
		return BrokerTickerParts{Uncertain: true, Reason: "missing_ticker"}
	}
	rawParts := strings.Split(trimmed, "_")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) >= 3 {
		if looksLikeAssetCode(parts[len(parts)-2]) && looksLikeVenueCode(parts[len(parts)-1]) {
			return BrokerTickerParts{
				Symbol:       strings.Join(parts[:len(parts)-2], "_"),
				ExchangeCode: parts[len(parts)-1],
				AssetCode:    strings.ToUpper(parts[len(parts)-2]),
				Parsed:       true,
			}
		}
		exchangeCode := parts[len(parts)-2]
		assetCode := parts[len(parts)-1]
		symbol := strings.Join(parts[:len(parts)-2], "_")
		parsed := BrokerTickerParts{
			Symbol:       symbol,
			ExchangeCode: exchangeCode,
			AssetCode:    assetCode,
			Parsed:       true,
		}
		if symbol == "" || !looksLikeVenueCode(exchangeCode) || !looksLikeAssetCode(assetCode) {
			parsed.Uncertain = true
			parsed.Reason = "unrecognised_broker_ticker_suffix"
		}
		return parsed
	}
	if len(parts) == 2 && looksLikeAssetCode(parts[1]) {
		return BrokerTickerParts{
			Symbol:    compactBrokerSymbol(parts[0]),
			AssetCode: strings.ToUpper(parts[1]),
			Parsed:    true,
		}
	}
	if len(parts) == 1 {
		if symbol, assetCode, ok := compactAssetSuffix(trimmed); ok {
			return BrokerTickerParts{
				Symbol:    symbol,
				AssetCode: assetCode,
				Parsed:    true,
			}
		}
		if looksLikeStandaloneBrokerSymbol(trimmed) {
			return BrokerTickerParts{Symbol: trimmed, Parsed: true}
		}
	}
	if len(parts) == 2 && looksLikeVenueCode(parts[1]) {
		return BrokerTickerParts{
			Symbol:       parts[0],
			ExchangeCode: parts[1],
			Parsed:       true,
			Uncertain:    true,
			Reason:       "missing_broker_asset_code",
		}
	}
	return BrokerTickerParts{Symbol: trimmed, Uncertain: true, Reason: "unrecognised_broker_ticker"}
}

func compactAssetSuffix(value string) (string, string, bool) {
	for _, assetCode := range brokerAssetCodes() {
		if len(value) <= len(assetCode) || !strings.HasSuffix(strings.ToUpper(value), assetCode) {
			continue
		}
		prefix := value[:len(value)-len(assetCode)]
		if !hasTrailingLowercaseASCII(prefix) {
			continue
		}
		symbol := compactBrokerSymbol(prefix)
		if symbol == "" {
			continue
		}
		return symbol, assetCode, true
	}
	return "", "", false
}

func compactBrokerSymbol(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 1 {
		last := rune(value[len(value)-1])
		if last >= 'a' && last <= 'z' {
			return value[:len(value)-1]
		}
	}
	return value
}

func ClassifyInstrumentCategory(raw trading212.Instrument) (string, []string) {
	typ := strings.ToUpper(strings.TrimSpace(raw.Type))
	joined := normaliseIdentityText(raw.Ticker, raw.Name, raw.ShortName, raw.Type)
	nameText := normaliseIdentityText(raw.Name, raw.ShortName)
	switch typ {
	case "STOCK", "SHARE", "SHARES", "EQUITY", "EQUITIES":
		if strings.Contains(nameText, " INVESTMENT TRUST ") || strings.Contains(nameText, " INV TRUST ") {
			return CategoryInvestmentTrust, []string{"category_from_investment_trust_name"}
		}
		return CategoryStock, []string{"category_from_trading212_type"}
	case "ETF", "ETFS", "EXCHANGE_TRADED_FUND", "EXCHANGE TRADED FUND":
		return CategoryETF, []string{"category_from_trading212_type"}
	case "FUND", "MUTUAL_FUND", "OPEN_END_FUND", "OEIC":
		return CategoryFund, []string{"category_from_trading212_type"}
	case "INVESTMENT_TRUST", "INVESTMENT TRUST", "TRUST":
		return CategoryInvestmentTrust, []string{"category_from_trading212_type"}
	case "WARRANT", "WARRANTS", "RIGHT", "RIGHTS":
		return CategoryWarrant, []string{"category_from_trading212_type"}
	case "CRYPTO", "CRYPTOCURRENCY":
		return CategoryCrypto, []string{"category_from_trading212_type"}
	case "FOREX", "FX", "CURRENCY":
		return CategoryForex, []string{"category_from_trading212_type"}
	case "BOND", "BONDS", "DEBT", "NOTE", "NOTES":
		return CategoryBond, []string{"category_from_trading212_type"}
	case "COMMODITY", "COMMODITIES", "ETC":
		return CategoryCommodity, []string{"category_from_trading212_type"}
	}
	switch {
	case containsAnyToken(joined, "BITCOIN", "ETHEREUM", "CRYPTOCURRENCY"):
		return CategoryCrypto, []string{"category_from_clear_name_marker"}
	case containsAnyToken(joined, "FOREX") || strings.Contains(joined, " FX "):
		return CategoryForex, []string{"category_from_clear_name_marker"}
	case containsAnyToken(joined, "BOND", "BONDS"):
		return CategoryBond, []string{"category_from_clear_name_marker"}
	case containsAnyToken(joined, "COMMODITY", "COMMODITIES"):
		return CategoryCommodity, []string{"category_from_clear_name_marker"}
	default:
		return CategoryOther, []string{"unknown_trading212_type"}
	}
}

func DetectStructureFlags(category string, values ...string) []string {
	joined := normaliseIdentityText(values...)
	fullNameText := joined
	if len(values) > 1 {
		fullNameText = normaliseIdentityText(values[1])
	}
	upperRaw := strings.ToUpper(strings.Join(values, " "))
	parts := ParseBrokerTicker(firstNonEmpty(values...))
	symbol := strings.ToUpper(parts.Symbol)
	flags := []string{}

	if containsAnyToken(joined, "SHORT") || hasAnyPrefix(symbol, "1S", "2S", "3S", "X1S", "X2S", "X3S") {
		flags = appendUnique(flags, "short")
	}
	if containsAnyToken(joined, "INVERSE") || strings.Contains(upperRaw, "-1X") || strings.Contains(upperRaw, "-2X") || strings.Contains(upperRaw, "-3X") || hasAnyPrefix(symbol, "1S", "2S", "3S", "X1S", "X2S", "X3S") {
		flags = appendUnique(flags, "inverse")
	}
	if containsAnyToken(joined, "LEVERAGED", "LEVERAGE", "2X", "3X", "X2", "X3") || strings.Contains(upperRaw, "-2X") || strings.Contains(upperRaw, "-3X") || hasAnyPrefix(symbol, "2L", "3L", "2S", "3S", "X2S", "X3S") {
		flags = appendUnique(flags, "leveraged")
	}
	if containsAnyToken(joined, "SYNTHETIC", "SWAP") {
		flags = appendUnique(flags, "synthetic")
	}
	if containsAnyToken(joined, "HEDGED", "HEDGE") {
		flags = appendUnique(flags, "hedged")
	}
	if containsAnyToken(joined, "ACC", "ACCUMULATING") {
		flags = appendUnique(flags, "accumulating")
	}
	if containsAnyToken(joined, "DIST", "DISTRIBUTING", "DISTRIBUTION") {
		flags = appendUnique(flags, "distributing")
	}
	if category == CategoryStock && (containsAnyToken(fullNameText, "ADR") || strings.Contains(fullNameText, " AMERICAN DEPOSITARY ") || strings.Contains(fullNameText, " AMERICAN DEPOSITORY ")) {
		flags = appendUnique(flags, "adr")
	}
	if category == CategoryStock && (containsAnyToken(fullNameText, "GDR") || strings.Contains(fullNameText, " GLOBAL DEPOSITARY ") || strings.Contains(fullNameText, " GLOBAL DEPOSITORY ")) {
		flags = appendUnique(flags, "gdr")
	}
	if category == CategoryETF || category == CategoryFund || category == CategoryInvestmentTrust {
		flags = appendUnique(flags, "fund_like")
	}
	return orderedStructureFlags(flags)
}

func DetectDirectionality(values ...string) string {
	flags := DetectStructureFlags("", values...)
	if containsString(flags, "short") || containsString(flags, "inverse") {
		return "inverse_or_short"
	}
	if containsString(flags, "leveraged") {
		return "leveraged_long"
	}
	return "long_or_unlevered"
}

func orderedStructureFlags(values []string) []string {
	out := []string{}
	for _, flag := range structureFlagOrder {
		if containsString(values, flag) {
			out = append(out, flag)
		}
	}
	return out
}

func normaliseIdentityText(values ...string) string {
	var b strings.Builder
	b.WriteRune(' ')
	for _, value := range values {
		for _, r := range strings.ToUpper(value) {
			switch {
			case unicode.IsLetter(r) || unicode.IsDigit(r):
				b.WriteRune(r)
			default:
				b.WriteRune(' ')
			}
		}
		b.WriteRune(' ')
	}
	fields := strings.Join(strings.Fields(b.String()), " ")
	if fields == "" {
		return " "
	}
	return " " + fields + " "
}

func containsAnyToken(text string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(text, " "+strings.ToUpper(token)+" ") {
			return true
		}
	}
	return false
}

func containsString(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}

func hasAnyPrefix(value string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func looksLikeVenueCode(value string) bool {
	if value == "" || len(value) > 12 {
		return false
	}
	for _, r := range value {
		if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func looksLikeAssetCode(value string) bool {
	upper := strings.ToUpper(value)
	for _, assetCode := range brokerAssetCodes() {
		if upper == assetCode {
			return true
		}
	}
	return false
}

func brokerAssetCodes() []string {
	return []string{"COMMODITY", "WARRANT", "CRYPTO", "FOREX", "RIGHT", "FUND", "BOND", "ETF", "ETC", "ETN", "WAR", "EQ", "FX"}
}

func looksLikeStandaloneBrokerSymbol(value string) bool {
	if value == "" || len(value) > 12 {
		return false
	}
	hasSymbolChar := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			hasSymbolChar = true
		case r >= '0' && r <= '9':
			hasSymbolChar = true
		case r == '.':
		default:
			return false
		}
	}
	return hasSymbolChar
}

func hasTrailingLowercaseASCII(value string) bool {
	if value == "" {
		return false
	}
	last := value[len(value)-1]
	return last >= 'a' && last <= 'z'
}

func mergeFlags(existing []string, additions ...[]string) []string {
	out := append([]string(nil), existing...)
	for _, values := range additions {
		for _, value := range values {
			out = appendUnique(out, value)
		}
	}
	return orderedStructureFlags(out)
}

func sortedStringSet(values []string) []string {
	out := unique(values)
	sort.Strings(out)
	return out
}
