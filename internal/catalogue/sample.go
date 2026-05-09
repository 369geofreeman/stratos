package catalogue

import (
	"statos/internal/enrichment"
	"statos/internal/trading212"
)

const SampleEnrichmentRetrievedAt = "2026-05-09T12:00:00Z"

func SampleData() ([]trading212.Instrument, []trading212.Exchange, map[string]enrichment.Profile) {
	exchanges := []trading212.Exchange{
		{ID: 1, Name: "NASDAQ"},
		{ID: 2, Name: "NYSE"},
	}
	instruments := []trading212.Instrument{
		{Ticker: "NVDA_US_EQ", Name: "NVIDIA Corporation", ShortName: "NVIDIA", ISIN: "US67066G1040", Type: "STOCK", CurrencyCode: "USD", WorkingScheduleID: 1, MaxOpenQuantity: 100000, ExtendedHours: true},
		{Ticker: "MSFT_US_EQ", Name: "Microsoft Corporation", ShortName: "Microsoft", ISIN: "US5949181045", Type: "STOCK", CurrencyCode: "USD", WorkingScheduleID: 1, MaxOpenQuantity: 100000, ExtendedHours: true},
		{Ticker: "ASML_US_EQ", Name: "ASML Holding N.V.", ShortName: "ASML", ISIN: "USN070592100", Type: "STOCK", CurrencyCode: "USD", WorkingScheduleID: 1, MaxOpenQuantity: 100000, ExtendedHours: true},
		{Ticker: "VRT_US_EQ", Name: "Vertiv Holdings Co", ShortName: "Vertiv", ISIN: "US92537N1081", Type: "STOCK", CurrencyCode: "USD", WorkingScheduleID: 2, MaxOpenQuantity: 100000, ExtendedHours: true},
		{Ticker: "CEG_US_EQ", Name: "Constellation Energy Corporation", ShortName: "Constellation Energy", ISIN: "US21037T1097", Type: "STOCK", CurrencyCode: "USD", WorkingScheduleID: 1, MaxOpenQuantity: 100000, ExtendedHours: true},
		{Ticker: "AMD_US_EQ", Name: "Advanced Micro Devices Inc", ShortName: "AMD", ISIN: "US0079031078", Type: "STOCK", CurrencyCode: "USD", WorkingScheduleID: 1, MaxOpenQuantity: 100000, ExtendedHours: true},
		{Ticker: "AVGO_US_EQ", Name: "Broadcom Inc", ShortName: "Broadcom", ISIN: "US11135F1012", Type: "STOCK", CurrencyCode: "USD", WorkingScheduleID: 1, MaxOpenQuantity: 100000, ExtendedHours: true},
		{Ticker: "MU_US_EQ", Name: "Micron Technology Inc", ShortName: "Micron", ISIN: "US5951121038", Type: "STOCK", CurrencyCode: "USD", WorkingScheduleID: 1, MaxOpenQuantity: 100000, ExtendedHours: true},
		{Ticker: "SMCI_US_EQ", Name: "Super Micro Computer Inc", ShortName: "Supermicro", ISIN: "US86800U1043", Type: "STOCK", CurrencyCode: "USD", WorkingScheduleID: 1, MaxOpenQuantity: 100000, ExtendedHours: true},
		{Ticker: "VST_US_EQ", Name: "Vistra Corp", ShortName: "Vistra", ISIN: "US92840M1027", Type: "STOCK", CurrencyCode: "USD", WorkingScheduleID: 2, MaxOpenQuantity: 100000, ExtendedHours: true},
		{Ticker: "TSM_US_EQ", Name: "Taiwan Semiconductor Manufacturing Company Limited", ShortName: "TSMC", ISIN: "US8740391003", Type: "STOCK", CurrencyCode: "USD", WorkingScheduleID: 2, MaxOpenQuantity: 100000, ExtendedHours: true},
		{Ticker: "AAPL_US_EQ", Name: "Apple Inc", ShortName: "Apple", ISIN: "US0378331005", Type: "STOCK", CurrencyCode: "USD", WorkingScheduleID: 1, MaxOpenQuantity: 100000, ExtendedHours: true},
		{Ticker: "3SQQQ_US_EQ", Name: "UltraPro Short QQQ -3X Daily ETF", ShortName: "Short QQQ 3x", ISIN: "US74347G4322", Type: "ETF", CurrencyCode: "USD", WorkingScheduleID: 1, MaxOpenQuantity: 100000, ExtendedHours: true},
	}
	profiles := map[string]enrichment.Profile{
		"NVDA_US_EQ": {Symbol: "NVDA", Name: "NVIDIA Corporation", Sector: "Technology", Industry: "Semiconductors", Country: "United States", MarketCap: 3000000000000, Source: "sample"},
		"MSFT_US_EQ": {Symbol: "MSFT", Name: "Microsoft Corporation", Sector: "Technology", Industry: "Software - Infrastructure", Country: "United States", MarketCap: 3100000000000, Source: "sample"},
		"ASML_US_EQ": {Symbol: "ASML", Name: "ASML Holding N.V.", Sector: "Technology", Industry: "Semiconductor Equipment", Country: "Netherlands", MarketCap: 350000000000, Source: "sample"},
		"VRT_US_EQ":  {Symbol: "VRT", Name: "Vertiv Holdings Co", Sector: "Industrials", Industry: "Electrical Equipment", Country: "United States", MarketCap: 45000000000, Source: "sample"},
		"CEG_US_EQ":  {Symbol: "CEG", Name: "Constellation Energy Corporation", Sector: "Utilities", Industry: "Independent Power Producers", Country: "United States", MarketCap: 90000000000, Source: "sample"},
		"AMD_US_EQ":  {Symbol: "AMD", Name: "Advanced Micro Devices Inc", Sector: "Technology", Industry: "Semiconductors", Country: "United States", MarketCap: 250000000000, Source: "sample"},
		"AVGO_US_EQ": {Symbol: "AVGO", Name: "Broadcom Inc", Sector: "Technology", Industry: "Semiconductors", Country: "United States", MarketCap: 700000000000, Source: "sample"},
		"MU_US_EQ":   {Symbol: "MU", Name: "Micron Technology Inc", Sector: "Technology", Industry: "Semiconductors", Country: "United States", MarketCap: 130000000000, Source: "sample"},
		"SMCI_US_EQ": {Symbol: "SMCI", Name: "Super Micro Computer Inc", Sector: "Technology", Industry: "Computer Hardware", Country: "United States", MarketCap: 30000000000, Source: "sample"},
		"VST_US_EQ":  {Symbol: "VST", Name: "Vistra Corp", Sector: "Utilities", Industry: "Independent Power Producers", Country: "United States", MarketCap: 55000000000, Source: "sample"},
		"TSM_US_EQ":  {Symbol: "TSM", Name: "Taiwan Semiconductor Manufacturing Company Limited", Sector: "Technology", Industry: "Semiconductors", Country: "Taiwan", MarketCap: 800000000000, Source: "sample"},
		"AAPL_US_EQ": {Symbol: "AAPL", Name: "Apple Inc", Sector: "Technology", Industry: "Consumer Electronics", Country: "United States", MarketCap: 2900000000000, Source: "sample"},
	}
	for ticker, profile := range profiles {
		profile.RetrievedAt = SampleEnrichmentRetrievedAt
		profiles[ticker] = profile
	}
	return instruments, exchanges, profiles
}
