package catalogue

import (
	"statos/internal/enrichment"
	"statos/internal/taxonomy"
	"statos/internal/trading212"
)

const DataContractVersion = 1
const DataContractSchemaVersion = 1

type Catalogue struct {
	DataContractVersion int                     `json:"dataContractVersion"`
	SchemaVersion       int                     `json:"schemaVersion"`
	GeneratedAt         string                  `json:"generatedAt,omitempty"`
	Manifest            BuildManifest           `json:"manifest"`
	Tickers             []Ticker                `json:"tickers"`
	Securities          []Security              `json:"securities"`
	Listings            []Listing               `json:"listings"`
	Companies           []Company               `json:"companies"`
	Sectors             []GroupCount            `json:"sectors"`
	Industries          []GroupCount            `json:"industries"`
	Themes              []taxonomy.Theme        `json:"themes"`
	SupplyChains        []taxonomy.SupplyChain  `json:"supplyChains"`
	Exposures           []taxonomy.Exposure     `json:"exposures"`
	Relationships       []taxonomy.Relationship `json:"relationships"`
	Notes               []taxonomy.Note         `json:"notes"`
	Unclassified        []UnclassifiedRow       `json:"unclassified"`
	IdentityIssues      []IdentityIssue         `json:"identityIssues,omitempty"`
	EnrichmentFailures  []EnrichmentFailure     `json:"-"`
}

type BuildManifest struct {
	DataContractVersion          int                               `json:"dataContractVersion"`
	SchemaVersion                int                               `json:"schemaVersion"`
	BuiltAt                      string                            `json:"builtAt"`
	SourceMode                   string                            `json:"sourceMode,omitempty"`
	Trading212Environment        string                            `json:"trading212Environment,omitempty"`
	Trading212BaseURL            string                            `json:"trading212BaseUrl,omitempty"`
	Trading212FetchAt            string                            `json:"trading212FetchAt,omitempty"`
	InstrumentCount              int                               `json:"instrumentCount"`
	ExchangeCount                int                               `json:"exchangeCount"`
	SecurityCount                int                               `json:"securityCount"`
	CompanyCount                 int                               `json:"companyCount"`
	ListingCount                 int                               `json:"listingCount"`
	ThemeCount                   int                               `json:"themeCount"`
	ExposureCount                int                               `json:"exposureCount"`
	RelationshipCount            int                               `json:"relationshipCount"`
	EnrichmentAttempted          int                               `json:"enrichmentAttempted"`
	EnrichmentSucceeded          int                               `json:"enrichmentSucceeded"`
	EnrichmentFailed             int                               `json:"enrichmentFailed"`
	EnrichmentCacheSchemaVersion int                               `json:"enrichmentCacheSchemaVersion"`
	EnrichmentProvider           string                            `json:"enrichmentProvider"`
	EnrichmentCacheHitCount      int                               `json:"enrichmentCacheHitCount"`
	EnrichmentCacheMissCount     int                               `json:"enrichmentCacheMissCount"`
	EnrichmentCacheStaleCount    int                               `json:"enrichmentCacheStaleCount"`
	EnrichmentAmbiguousCount     int                               `json:"enrichmentAmbiguousCount"`
	EnrichmentFailureCount       int                               `json:"enrichmentFailureCount"`
	EnrichmentFailureCSV         string                            `json:"enrichmentFailureCSV"`
	EnrichmentOldestRetrievedAt  string                            `json:"enrichmentOldestRetrievedAt"`
	EnrichmentNewestRetrievedAt  string                            `json:"enrichmentNewestRetrievedAt"`
	UnclassifiedCount            int                               `json:"unclassifiedCount"`
	EmptyTickerCount             int                               `json:"emptyTickerCount"`
	DuplicateTickerCount         int                               `json:"duplicateTickerCount"`
	DuplicateISINCount           int                               `json:"duplicateISINCount"`
	MissingISINCount             int                               `json:"missingISINCount"`
	IdentityCollisionCount       int                               `json:"identityCollisionCount"`
	IdentityOverrideCount        int                               `json:"identityOverrideCount"`
	IdentityIssueCount           int                               `json:"identityIssueCount"`
	InstrumentCategoryCounts     map[string]int                    `json:"instrumentCategoryCounts,omitempty"`
	StructureFlagCounts          map[string]int                    `json:"structureFlagCounts,omitempty"`
	RawSnapshotAt                string                            `json:"rawSnapshotAt,omitempty"`
	RawSnapshots                 RawSnapshotSummary                `json:"rawSnapshots,omitempty"`
	Trading212HTTPDiagnostics    []trading212.EndpointDiagnostic   `json:"trading212HttpDiagnostics,omitempty"`
	Trading212RateLimits         []trading212.RateLimitObservation `json:"trading212RateLimits,omitempty"`
	DataFreshness                string                            `json:"dataFreshness,omitempty"`
	GeneratedFiles               []GeneratedFile                   `json:"generatedFiles,omitempty"`
}

type GeneratedFile struct {
	Path          string `json:"path"`
	Format        string `json:"format"`
	SchemaVersion int    `json:"schemaVersion"`
	SHA256        string `json:"sha256"`
	Bytes         int64  `json:"bytes"`
	ChecksumMode  string `json:"checksumMode,omitempty"`
}

type EnrichmentDiagnostics struct {
	CacheSchemaVersion int
	Provider           string
	CacheHitCount      int
	CacheMissCount     int
	CacheStaleCount    int
	AmbiguousCount     int
	FailureCount       int
	FailureCSV         string
	OldestRetrievedAt  string
	NewestRetrievedAt  string
}

type EnrichmentFailure = enrichment.Failure

type RawSnapshotSummary struct {
	Timestamp           string `json:"timestamp,omitempty"`
	Directory           string `json:"directory,omitempty"`
	InstrumentsPath     string `json:"instrumentsPath,omitempty"`
	ExchangesPath       string `json:"exchangesPath,omitempty"`
	InstrumentsLatest   string `json:"instrumentsLatest,omitempty"`
	ExchangesLatest     string `json:"exchangesLatest,omitempty"`
	InstrumentFileCount int    `json:"instrumentFileCount,omitempty"`
	ExchangeFileCount   int    `json:"exchangeFileCount,omitempty"`
}

type Ticker struct {
	Ticker             string   `json:"ticker"`
	BrokerSymbol       string   `json:"brokerSymbol,omitempty"`
	BrokerAssetCode    string   `json:"brokerAssetCode,omitempty"`
	Name               string   `json:"name"`
	ShortName          string   `json:"shortName,omitempty"`
	Type               string   `json:"type,omitempty"`
	InstrumentCategory string   `json:"instrumentCategory,omitempty"`
	StructureFlags     []string `json:"structureFlags,omitempty"`
	CurrencyCode       string   `json:"currencyCode,omitempty"`
	ISIN               string   `json:"isin,omitempty"`
	ExchangeCode       string   `json:"exchangeCode,omitempty"`
	ExchangeName       string   `json:"exchangeName,omitempty"`
	WorkingScheduleID  int64    `json:"workingScheduleId,omitempty"`
	MaxOpenQuantity    float64  `json:"maxOpenQuantity,omitempty"`
	ExtendedHours      bool     `json:"extendedHours,omitempty"`
	SecurityID         string   `json:"securityId"`
	CompanyID          string   `json:"companyId"`
	ListingID          string   `json:"listingId"`
	Directionality     string   `json:"directionality"`
	YahooSymbol        string   `json:"yahooSymbol,omitempty"`
	Sector             string   `json:"sector,omitempty"`
	Industry           string   `json:"industry,omitempty"`
	Country            string   `json:"country,omitempty"`
	MarketCap          int64    `json:"marketCap,omitempty"`
	ThemeIDs           []string `json:"themeIds,omitempty"`
	LayerIDs           []string `json:"layerIds,omitempty"`
	RelatedTickers     []string `json:"relatedTickers,omitempty"`
	Sources            []Source `json:"sources,omitempty"`
	IdentityConfidence string   `json:"identityConfidence,omitempty"`
	IdentityReasons    []string `json:"identityReasons,omitempty"`
	LastReviewed       string   `json:"lastReviewed,omitempty"`
	LastRefreshed      string   `json:"lastRefreshed,omitempty"`
	Unclassified       bool     `json:"unclassified"`
}

type Security struct {
	ID                 string   `json:"id"`
	ISIN               string   `json:"isin,omitempty"`
	Name               string   `json:"name"`
	Type               string   `json:"type,omitempty"`
	InstrumentCategory string   `json:"instrumentCategory,omitempty"`
	StructureFlags     []string `json:"structureFlags,omitempty"`
	CompanyID          string   `json:"companyId"`
	ListingIDs         []string `json:"listingIds"`
	TickerIDs          []string `json:"tickerIds"`
	CurrencySet        []string `json:"currencySet,omitempty"`
	IdentityConfidence string   `json:"identityConfidence,omitempty"`
	IdentityReasons    []string `json:"identityReasons,omitempty"`
}

type Listing struct {
	ID           string `json:"id"`
	Ticker       string `json:"ticker"`
	SecurityID   string `json:"securityId"`
	CompanyID    string `json:"companyId"`
	ExchangeCode string `json:"exchangeCode,omitempty"`
	ExchangeName string `json:"exchangeName,omitempty"`
	CurrencyCode string `json:"currencyCode,omitempty"`
}

type Company struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	PrimaryTicker      string   `json:"primaryTicker,omitempty"`
	Sector             string   `json:"sector,omitempty"`
	Industry           string   `json:"industry,omitempty"`
	Country            string   `json:"country,omitempty"`
	YahooSymbol        string   `json:"yahooSymbol,omitempty"`
	MarketCap          int64    `json:"marketCap,omitempty"`
	SecurityIDs        []string `json:"securityIds"`
	ListingIDs         []string `json:"listingIds"`
	TickerIDs          []string `json:"tickerIds"`
	ThemeIDs           []string `json:"themeIds,omitempty"`
	LayerIDs           []string `json:"layerIds,omitempty"`
	RelatedTickers     []string `json:"relatedTickers,omitempty"`
	Sources            []Source `json:"sources,omitempty"`
	IdentityConfidence string   `json:"identityConfidence,omitempty"`
	IdentityReasons    []string `json:"identityReasons,omitempty"`
	LastReviewed       string   `json:"lastReviewed,omitempty"`
	LastRefreshed      string   `json:"lastRefreshed,omitempty"`
}

type Source struct {
	Kind         string `json:"kind"`
	URL          string `json:"url,omitempty"`
	Label        string `json:"label,omitempty"`
	LastReviewed string `json:"lastReviewed,omitempty"`
}

type GroupCount struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Count   int      `json:"count"`
	Tickers []string `json:"tickers,omitempty"`
}

type UnclassifiedRow struct {
	Ticker    string `json:"ticker"`
	CompanyID string `json:"companyId,omitempty"`
	Name      string `json:"name"`
	ISIN      string `json:"isin,omitempty"`
	Reason    string `json:"reason"`
}

type IdentityIssue struct {
	IssueCode       string `json:"issueCode"`
	Ticker          string `json:"ticker,omitempty"`
	ISIN            string `json:"isin,omitempty"`
	SecurityID      string `json:"securityId,omitempty"`
	CompanyID       string `json:"companyId,omitempty"`
	Name            string `json:"name,omitempty"`
	Reason          string `json:"reason"`
	SuggestedAction string `json:"suggestedAction,omitempty"`
}
