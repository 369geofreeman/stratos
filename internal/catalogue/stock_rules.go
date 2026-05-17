package catalogue

import (
	"fmt"
	"sort"
	"strings"

	"statos/internal/taxonomy"
)

const (
	themeConnectivityInfrastructure = "connectivity_infrastructure"
	confidenceStockRule             = "rule_low"
	ruleStockExposureScore          = 2
	ruleRationaleStockBase          = "Rule from classified stock sector/industry"
)

type stockRuleAssignment struct {
	themeID string
	layerID string
	reason  string
}

func buildStockRuleExposures(tickers map[string]*Ticker, manual taxonomy.ManualData) []taxonomy.Exposure {
	validLayers := productRuleLayerSet(manual)
	if len(validLayers) == 0 {
		return nil
	}

	tickerIDs := make([]string, 0, len(tickers))
	for tickerID := range tickers {
		tickerIDs = append(tickerIDs, tickerID)
	}
	sort.Strings(tickerIDs)

	assignmentsByTicker := map[string][]stockRuleAssignment{}
	assignmentKeysByISIN := map[string]map[string]bool{}
	for _, tickerID := range tickerIDs {
		ticker := tickers[tickerID]
		if ticker == nil || !isStockRuleCandidate(*ticker) {
			continue
		}
		assignments := validStockRuleAssignments(*ticker, validLayers)
		assignmentsByTicker[tickerID] = assignments
		if ticker.ISIN != "" {
			if assignmentKeysByISIN[ticker.ISIN] == nil {
				assignmentKeysByISIN[ticker.ISIN] = map[string]bool{}
			}
			assignmentKeysByISIN[ticker.ISIN][stockRuleAssignmentSetKey(assignments)] = true
		}
	}
	conflictedISINs := map[string]bool{}
	for isin, keys := range assignmentKeysByISIN {
		if len(keys) > 1 {
			conflictedISINs[isin] = true
		}
	}

	index := newStockRuleTickerIndex(tickers)
	manualResolved := manualExposureTickerLayerSet(manual.Exposures, index)
	seen := map[string]bool{}
	var out []taxonomy.Exposure
	for _, tickerID := range tickerIDs {
		ticker := tickers[tickerID]
		if ticker == nil || !isStockRuleCandidate(*ticker) {
			continue
		}
		for _, assignment := range assignmentsByTicker[tickerID] {
			scope := stockRuleExposureScope(*ticker, conflictedISINs[ticker.ISIN], index)
			available := unshadowedStockRuleScope(assignment, scope, manualResolved)
			switch {
			case len(available) == 0:
				continue
			case len(available) == len(scope):
				exposure := stockRuleExposure(*ticker, assignment, conflictedISINs[ticker.ISIN])
				if appendUniqueStockRuleExposure(&out, seen, exposure) {
					continue
				}
			default:
				for _, scopedTickerID := range available {
					exposure := stockRuleTickerExposure(scopedTickerID, assignment)
					appendUniqueStockRuleExposure(&out, seen, exposure)
				}
			}
		}
	}
	return out
}

func isStockRuleCandidate(ticker Ticker) bool {
	return ticker.InstrumentCategory == CategoryStock &&
		strings.TrimSpace(ticker.Sector) != "" &&
		strings.TrimSpace(ticker.Industry) != ""
}

func validStockRuleAssignments(ticker Ticker, validLayers map[string]bool) []stockRuleAssignment {
	var out []stockRuleAssignment
	for _, assignment := range stockRuleAssignments(ticker) {
		if validLayers[assignment.themeID+"|"+assignment.layerID] {
			out = append(out, assignment)
		}
	}
	return out
}

func stockRuleAssignmentSetKey(assignments []stockRuleAssignment) string {
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

func stockRuleExposure(ticker Ticker, assignment stockRuleAssignment, forceTickerTarget bool) taxonomy.Exposure {
	exposure := taxonomy.Exposure{
		ThemeID:       assignment.themeID,
		LayerID:       assignment.layerID,
		ExposureScore: ruleStockExposureScore,
		Confidence:    confidenceStockRule,
		Rationale:     fmt.Sprintf("%s: %s.", ruleRationaleStockBase, assignment.reason),
	}
	if ticker.ISIN != "" && !forceTickerTarget {
		exposure.ISIN = ticker.ISIN
	} else {
		exposure.Ticker = ticker.Ticker
	}
	return exposure
}

func stockRuleTickerExposure(tickerID string, assignment stockRuleAssignment) taxonomy.Exposure {
	return taxonomy.Exposure{
		ThemeID:       assignment.themeID,
		LayerID:       assignment.layerID,
		Ticker:        tickerID,
		ExposureScore: ruleStockExposureScore,
		Confidence:    confidenceStockRule,
		Rationale:     fmt.Sprintf("%s: %s.", ruleRationaleStockBase, assignment.reason),
	}
}

func appendUniqueStockRuleExposure(out *[]taxonomy.Exposure, seen map[string]bool, exposure taxonomy.Exposure) bool {
	key := strings.Join([]string{
		exposure.ThemeID,
		exposure.LayerID,
		exposure.Ticker,
		exposure.ISIN,
	}, "\x00")
	if seen[key] {
		return true
	}
	seen[key] = true
	*out = append(*out, exposure)
	return false
}

type stockRuleTickerIndex struct {
	tickersByCompany  map[string][]string
	securityIDsByISIN map[string][]string
	tickersBySecurity map[string][]string
}

func newStockRuleTickerIndex(tickers map[string]*Ticker) stockRuleTickerIndex {
	index := stockRuleTickerIndex{
		tickersByCompany:  map[string][]string{},
		securityIDsByISIN: map[string][]string{},
		tickersBySecurity: map[string][]string{},
	}
	for tickerID, ticker := range tickers {
		if ticker == nil {
			continue
		}
		index.tickersByCompany[ticker.CompanyID] = appendUnique(index.tickersByCompany[ticker.CompanyID], tickerID)
		index.tickersBySecurity[ticker.SecurityID] = appendUnique(index.tickersBySecurity[ticker.SecurityID], tickerID)
		if ticker.ISIN != "" {
			index.securityIDsByISIN[ticker.ISIN] = appendUnique(index.securityIDsByISIN[ticker.ISIN], ticker.SecurityID)
		}
	}
	sortStockRuleIndex(index)
	return index
}

func sortStockRuleIndex(index stockRuleTickerIndex) {
	for _, values := range index.tickersByCompany {
		sort.Strings(values)
	}
	for _, values := range index.securityIDsByISIN {
		sort.Strings(values)
	}
	for _, values := range index.tickersBySecurity {
		sort.Strings(values)
	}
}

func manualExposureTickerLayerSet(exposures []taxonomy.Exposure, index stockRuleTickerIndex) map[string]bool {
	out := map[string]bool{}
	for _, exposure := range exposures {
		if exposure.ThemeID == "" || exposure.LayerID == "" {
			continue
		}
		if exposure.Ticker != "" {
			out[stockRuleResolvedKey(exposure.ThemeID, exposure.LayerID, exposure.Ticker)] = true
		}
		if exposure.CompanyID != "" {
			for _, tickerID := range index.tickersByCompany[exposure.CompanyID] {
				out[stockRuleResolvedKey(exposure.ThemeID, exposure.LayerID, tickerID)] = true
			}
		}
		if exposure.ISIN != "" {
			for _, tickerID := range stockRuleTickersForISIN(exposure.ISIN, index) {
				out[stockRuleResolvedKey(exposure.ThemeID, exposure.LayerID, tickerID)] = true
			}
		}
	}
	return out
}

func stockRuleExposureScope(ticker Ticker, forceTickerTarget bool, index stockRuleTickerIndex) []string {
	if ticker.ISIN == "" || forceTickerTarget {
		return []string{ticker.Ticker}
	}
	scope := stockRuleTickersForISIN(ticker.ISIN, index)
	if len(scope) == 0 {
		return []string{ticker.Ticker}
	}
	return scope
}

func stockRuleTickersForISIN(isin string, index stockRuleTickerIndex) []string {
	out := []string{}
	for _, securityID := range index.securityIDsByISIN[isin] {
		for _, tickerID := range index.tickersBySecurity[securityID] {
			out = appendUnique(out, tickerID)
		}
	}
	sort.Strings(out)
	return out
}

func unshadowedStockRuleScope(assignment stockRuleAssignment, scope []string, manualResolved map[string]bool) []string {
	out := make([]string, 0, len(scope))
	for _, tickerID := range scope {
		if manualResolved[stockRuleResolvedKey(assignment.themeID, assignment.layerID, tickerID)] {
			continue
		}
		out = append(out, tickerID)
	}
	return out
}

func stockRuleResolvedKey(themeID string, layerID string, tickerID string) string {
	return strings.Join([]string{themeID, layerID, tickerID}, "\x00")
}

func stockRuleAssignments(ticker Ticker) []stockRuleAssignment {
	sector := strings.TrimSpace(ticker.Sector)
	industry := strings.TrimSpace(ticker.Industry)
	text := normaliseIdentityText(ticker.Ticker, ticker.Name, ticker.ShortName, ticker.CompanyID)

	if assignments := stockIndustryAssignments(industry, text); len(assignments) > 0 {
		return assignments
	}
	if assignment, ok := stockSectorFallbackAssignment(sector); ok {
		return []stockRuleAssignment{assignment}
	}
	return nil
}

func stockIndustryAssignments(industry string, text string) []stockRuleAssignment {
	switch industry {
	case "Advertising Agencies":
		return stockAssignments("digital_platforms", "digital_advertising", industry)
	case "Aerospace & Defense":
		return stockAssignments("defence", "primes", industry)
	case "Agricultural Inputs":
		return stockAssignments("commodities", "agriculture_inputs", industry, "food_agriculture", "fertiliser_crop_inputs")
	case "Airlines", "Airports & Air Services":
		return stockAssignments("transport_logistics", "airlines_airports", industry)
	case "Aluminum":
		return stockAssignments("commodities", "aluminum", industry)
	case "Apparel Manufacturing", "Footwear & Accessories", "Luxury Goods":
		return stockAssignments("consumer_brands", "luxury_apparel", industry)
	case "Apparel Retail", "Department Stores", "Discount Stores", "Specialty Retail":
		return stockAssignments("consumer_brands", "specialty_retail", industry)
	case "Asset Management", "Financial Conglomerates":
		return stockAssignments("financial_system", "asset_management", industry)
	case "Auto & Truck Dealerships":
		return stockAssignments("mobility_ev", "dealers_retail_services", industry)
	case "Auto Manufacturers":
		return stockAssignments("mobility_ev", "auto_oems", industry)
	case "Auto Parts":
		return stockAssignments("mobility_ev", "auto_components", industry)
	case "Banks - Diversified":
		return stockAssignments("financial_system", "diversified_banks", industry)
	case "Banks - Regional":
		return stockAssignments("financial_system", "regional_banks", industry)
	case "Beverages - Brewers", "Beverages - Non-Alcoholic", "Beverages - Wineries & Distilleries", "Tobacco":
		return stockAssignments("consumer_brands", "beverages_tobacco", industry)
	case "Biotechnology":
		return stockAssignments("healthcare", "biotech", industry)
	case "Broadcasting", "Entertainment", "Publishing":
		return stockAssignments("digital_platforms", "streaming_media", industry)
	case "Building Materials", "Lumber & Wood Production":
		return stockAssignments("building_housing", "building_materials", industry)
	case "Building Products & Equipment":
		return stockAssignments("building_housing", "hvac_building_systems", industry, "industrial_automation", "machinery_equipment")
	case "Business Equipment & Supplies", "Industrial Distribution":
		return stockAssignments("industrial_automation", "machinery_equipment", industry)
	case "Capital Markets", "Shell Companies":
		if hasCryptoRailsMarker(text) {
			return stockAssignments("fintech", "crypto_rails", industry)
		}
		return stockAssignments("financial_system", "capital_markets", industry)
	case "Chemicals", "Specialty Chemicals":
		return stockAssignments("commodities", "specialty_chemicals", industry)
	case "Coking Coal", "Steel":
		return stockAssignments("commodities", "steel_iron_ore", industry)
	case "Communication Equipment":
		return stockAssignments(themeConnectivityInfrastructure, "network_equipment", industry)
	case "Computer Hardware":
		return stockAssignments("ai_infrastructure", "systems", industry, "enterprise_software", "data_observability")
	case "Confectioners", "Packaged Foods":
		return stockAssignments("food_agriculture", "packaged_foods", industry)
	case "Conglomerates":
		return stockAssignments("industrial_automation", "machinery_equipment", industry)
	case "Consulting Services", "Specialty Business Services", "Staffing & Employment Services":
		return stockAssignments("enterprise_software", "it_services_consulting", industry)
	case "Consumer Electronics":
		return stockAssignments("consumer_brands", "consumer_durables", industry)
	case "Copper":
		return stockAssignments("commodities", "copper", industry)
	case "Credit Services":
		return stockAssignments("financial_system", "credit_payments", industry)
	case "Diagnostics & Research":
		return stockAssignments("healthcare", "diagnostics_tools", industry)
	case "Drug Manufacturers - General":
		return stockAssignments("healthcare", "large_pharma", industry)
	case "Drug Manufacturers - Specialty & Generic":
		return stockAssignments("healthcare", "biopharma", industry)
	case "Education & Training Services", "Personal Services":
		return stockAssignments("consumer_brands", "specialty_retail", industry)
	case "Electrical Equipment", "Electrical Equipment & Parts":
		return stockAssignments("industrial_automation", "electrification_power", industry, "energy", "grid_equipment")
	case "Electronic Components":
		return stockAssignments("semiconductors", "analog_power", industry)
	case "Electronic Gaming & Multimedia", "Gambling":
		return stockAssignments("digital_platforms", "gaming_interactive", industry)
	case "Electronics & Computer Distribution":
		return stockAssignments("enterprise_software", "it_services_consulting", industry)
	case "Engineering & Construction", "Infrastructure Operations", "Pollution & Treatment Controls":
		return stockAssignments("industrial_automation", "engineering_services", industry)
	case "Farm & Heavy Construction Machinery":
		return stockAssignments("industrial_automation", "construction_agriculture_equipment", industry, "food_agriculture", "agriculture_machinery")
	case "Farm Products":
		return stockAssignments("food_agriculture", "agriculture_processing", industry)
	case "Financial Data & Stock Exchanges":
		return stockAssignments("financial_system", "exchanges_data", industry, "fintech", "capital_markets")
	case "Food Distribution", "Grocery Stores":
		return stockAssignments("food_agriculture", "grocery_distribution", industry)
	case "Furnishings, Fixtures & Appliances", "Leisure", "Recreational Vehicles":
		return stockAssignments("consumer_brands", "consumer_durables", industry)
	case "Gold", "Other Precious Metals & Mining", "Silver":
		return stockAssignments("commodities", "precious_metals", industry)
	case "Health Information Services":
		return stockAssignments("healthcare", "healthcare_it", industry)
	case "Healthcare Plans":
		return stockAssignments("healthcare", "managed_care", industry)
	case "Home Improvement Retail":
		return stockAssignments("building_housing", "home_improvement_retail", industry)
	case "Household & Personal Products":
		return stockAssignments("consumer_brands", "household_personal_care", industry)
	case "Independent Power Producers", "Utilities - Independent Power Producers":
		return stockAssignments("energy", "power_generation", industry)
	case "Information Technology Services":
		if hasPaymentsMarker(text) {
			return stockAssignments("fintech", "payments", industry, "enterprise_software", "it_services_consulting")
		}
		return stockAssignments("enterprise_software", "it_services_consulting", industry)
	case "Insurance - Diversified", "Insurance - Life", "Insurance - Property & Casualty", "Insurance - Reinsurance", "Insurance - Specialty", "Insurance Brokers":
		if hasInsurtechMarker(text) {
			return stockAssignments("fintech", "insurtech", industry, "financial_system", "insurance")
		}
		return stockAssignments("financial_system", "insurance", industry)
	case "Integrated Freight & Logistics":
		return stockAssignments("transport_logistics", "parcel_logistics", industry)
	case "Internet Content & Information":
		return stockAssignments("digital_platforms", "social_platforms", industry)
	case "Internet Retail":
		return stockAssignments("digital_platforms", "marketplaces_ecommerce", industry)
	case "Lodging", "Travel Services":
		return stockAssignments("digital_platforms", "travel_local_platforms", industry)
	case "Marine Shipping":
		return stockAssignments("transport_logistics", "shipping_ports", industry)
	case "Medical Care Facilities":
		return stockAssignments("healthcare", "care_delivery", industry)
	case "Medical Devices", "Medical Instruments & Supplies":
		return stockAssignments("healthcare", "medical_devices", industry)
	case "Medical Distribution", "Pharmaceutical Retailers":
		return stockAssignments("healthcare", "healthcare_distributors", industry)
	case "Metal Fabrication", "Specialty Industrial Machinery":
		return stockAssignments("industrial_automation", "machinery_equipment", industry)
	case "Mortgage Finance":
		return stockAssignments("financial_system", "credit_payments", industry)
	case "Oil & Gas Drilling", "Oil & Gas Equipment & Services":
		return stockAssignments("energy", "oilfield_services", industry)
	case "Oil & Gas E&P":
		return stockAssignments("energy", "upstream", industry)
	case "Oil & Gas Integrated":
		return stockAssignments("energy", "integrated_oil_gas", industry)
	case "Oil & Gas Midstream":
		return stockAssignments("energy", "midstream_lng", industry)
	case "Oil & Gas Refining & Marketing":
		return stockAssignments("energy", "refining_marketing", industry)
	case "Other Industrial Metals & Mining":
		return stockAssignments("commodities", "diversified_mining", industry)
	case "Packaging & Containers", "Paper & Paper Products":
		return stockAssignments("commodities", "commodity_processors", industry)
	case "Railroads":
		return stockAssignments("transport_logistics", "railroads", industry)
	case "Real Estate - Development", "Real Estate - Diversified", "Real Estate Services":
		return stockAssignments("real_estate_infrastructure", "property_services", industry)
	case "REIT - Diversified", "REIT - Healthcare Facilities", "REIT - Hotel & Motel", "REIT - Mortgage", "REIT - Office", "REIT - Retail":
		return stockAssignments("real_estate_infrastructure", "retail_office_healthcare_reits", industry)
	case "REIT - Industrial":
		return stockAssignments("real_estate_infrastructure", "industrial_logistics_reits", industry)
	case "REIT - Residential":
		return stockAssignments("real_estate_infrastructure", "residential_reits", industry)
	case "REIT - Specialty":
		if hasTowerInfrastructureMarker(text) {
			return stockAssignments("real_estate_infrastructure", "tower_reits", industry, themeConnectivityInfrastructure, "tower_infrastructure")
		}
		return stockAssignments("real_estate_infrastructure", "tower_reits", industry)
	case "Rental & Leasing Services":
		return stockAssignments("industrial_automation", "engineering_services", industry)
	case "Residential Construction":
		return stockAssignments("building_housing", "homebuilders_developers", industry)
	case "Resorts & Casinos", "Restaurants":
		return stockAssignments("consumer_brands", "restaurants_foodservice", industry)
	case "Scientific & Technical Instruments":
		return stockAssignments("industrial_automation", "testing_measurement", industry)
	case "Security & Protection Services":
		return stockAssignments("defence", "defence_services", industry)
	case "Semiconductor Equipment", "Semiconductor Equipment & Materials":
		return stockAssignments("semiconductors", "equipment", industry)
	case "Semiconductors":
		return stockAssignments("semiconductors", "fabless_design", industry)
	case "Software - Application":
		if hasIndustrialSoftwareMarker(text) {
			return stockAssignments("industrial_automation", "industrial_software", industry, "enterprise_software", "erp_crm_workflow")
		}
		return stockAssignments("enterprise_software", "erp_crm_workflow", industry)
	case "Software - Infrastructure":
		if hasCybersecurityMarker(text) {
			return stockAssignments("enterprise_software", "cybersecurity", industry, "defence", "cyber")
		}
		if hasCryptoRailsMarker(text) {
			return stockAssignments("fintech", "crypto_rails", industry)
		}
		return stockAssignments("enterprise_software", "data_observability", industry)
	case "Solar":
		return stockAssignments("energy", "solar_wind_equipment", industry)
	case "Telecom Services":
		return telecomStockAssignments(industry, text)
	case "Textile Manufacturing":
		return stockAssignments("consumer_brands", "luxury_apparel", industry)
	case "Thermal Coal":
		return stockAssignments("commodities", "steel_iron_ore", industry, "energy", "upstream")
	case "Tools & Accessories":
		return stockAssignments("building_housing", "tools_hardware", industry)
	case "Trucking":
		return stockAssignments("transport_logistics", "trucking_freight", industry)
	case "Uranium":
		return stockAssignments("commodities", "uranium", industry, "energy", "uranium_nuclear")
	case "Utilities - Diversified", "Utilities - Regulated Electric", "Utilities - Regulated Gas", "Utilities - Regulated Water":
		return stockAssignments("energy", "regulated_utilities", industry)
	case "Utilities - Renewable":
		return stockAssignments("energy", "renewable_developers", industry, "energy", "renewables")
	case "Waste Management":
		return stockAssignments("industrial_automation", "engineering_services", industry)
	default:
		return nil
	}
}

func stockSectorFallbackAssignment(sector string) (stockRuleAssignment, bool) {
	switch sector {
	case "Basic Materials":
		return stockAssignment("commodities", "commodity_processors", sector), true
	case "Communication Services":
		return stockAssignment("digital_platforms", "platform_infrastructure", sector), true
	case "Consumer Cyclical":
		return stockAssignment("consumer_brands", "consumer_durables", sector), true
	case "Consumer Defensive":
		return stockAssignment("consumer_brands", "household_personal_care", sector), true
	case "Energy":
		return stockAssignment("energy", "upstream", sector), true
	case "Financial Services":
		return stockAssignment("financial_system", "capital_markets", sector), true
	case "Healthcare":
		return stockAssignment("healthcare", "biopharma", sector), true
	case "Industrials":
		return stockAssignment("industrial_automation", "machinery_equipment", sector), true
	case "Real Estate":
		return stockAssignment("real_estate_infrastructure", "property_services", sector), true
	case "Technology":
		return stockAssignment("enterprise_software", "vertical_software", sector), true
	case "Utilities":
		return stockAssignment("energy", "regulated_utilities", sector), true
	default:
		return stockRuleAssignment{}, false
	}
}

func telecomStockAssignments(industry string, text string) []stockRuleAssignment {
	switch {
	case hasTowerInfrastructureMarker(text):
		return stockAssignments(themeConnectivityInfrastructure, "tower_infrastructure", industry)
	case containsAnyToken(text, "SATELLITE", "GLOBALSTAR", "IRIDIUM", "EUTELSAT", "ECHOSTAR"):
		return stockAssignments(themeConnectivityInfrastructure, "satellite_connectivity", industry)
	case hasFiberInterconnectionMarker(text):
		return stockAssignments(themeConnectivityInfrastructure, "fiber_interconnection", industry)
	case containsAnyToken(text, "CABLE", "BROADBAND", "CHARTER", "COMCAST", "COGECO") || strings.Contains(text, " LIBERTY GLOBAL "):
		return stockAssignments(themeConnectivityInfrastructure, "broadband_cable", industry)
	default:
		return stockAssignments(themeConnectivityInfrastructure, "telecom_operators", industry)
	}
}

func stockAssignments(themeID string, layerID string, reason string, additional ...string) []stockRuleAssignment {
	out := []stockRuleAssignment{stockAssignment(themeID, layerID, reason)}
	for i := 0; i+1 < len(additional); i += 2 {
		out = append(out, stockAssignment(additional[i], additional[i+1], reason))
	}
	return out
}

func stockAssignment(themeID string, layerID string, reason string) stockRuleAssignment {
	return stockRuleAssignment{
		themeID: themeID,
		layerID: layerID,
		reason:  fmt.Sprintf("%s maps to %s/%s", reason, themeID, layerID),
	}
}

func hasCybersecurityMarker(text string) bool {
	return containsAnyToken(text, "CYBER", "SECURITY", "CROWDSTRIKE", "FORTINET", "ZSCALER", "SENTINELONE") ||
		strings.Contains(text, " PALO ALTO ") ||
		strings.Contains(text, " CHECK POINT ")
}

func hasCryptoRailsMarker(text string) bool {
	return containsAnyToken(text, "BITCOIN", "BLOCKCHAIN", "CRYPTO", "COINBASE", "RIOT", "MARATHON", "CLEANSPARK")
}

func hasInsurtechMarker(text string) bool {
	return containsAnyToken(text, "LEMONADE", "ROOT", "HIPPO", "OSCAR")
}

func hasPaymentsMarker(text string) bool {
	return containsAnyToken(text, "PAYMENT", "PAYMENTS", "FISERV", "ADYEN", "PAYPAL", "STRIPE", "TOAST", "MARQETA")
}

func hasIndustrialSoftwareMarker(text string) bool {
	return containsAnyToken(text, "AUTODESK", "DASSAULT", "BENTLEY", "ANSYS", "PTC", "TRIMBLE")
}

func hasTowerInfrastructureMarker(text string) bool {
	return containsAnyToken(text, "TOWER", "TOWERS", "TELESITES", "EUROTELESITES", "AMERICAN TOWER", "CROWN CASTLE", "SBA")
}

func hasFiberInterconnectionMarker(text string) bool {
	return containsAnyToken(text, "FIBER", "FIBRE", "COGENT", "SIFY", "GOGO", "GAMMA") ||
		strings.Contains(text, " TELEPHONE AND DATA ")
}
