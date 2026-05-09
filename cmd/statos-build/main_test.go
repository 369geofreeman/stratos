package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"statos/internal/catalogue"
)

func TestReadRawSnapshotsMissingLatestFailsClearly(t *testing.T) {
	_, _, _, err := readRawSnapshots(t.TempDir())
	if err == nil {
		t.Fatal("expected missing raw replay error")
	}
	if !strings.Contains(err.Error(), "raw replay requested") || !strings.Contains(err.Error(), "instruments_latest.json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadRawSnapshotsFromLatestAliases(t *testing.T) {
	dir := t.TempDir()
	instruments, exchanges, _ := catalogue.SampleData()
	builtAt := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	written, err := writeRawSnapshots(dir, builtAt, instruments, exchanges)
	if err != nil {
		t.Fatal(err)
	}

	gotInstruments, gotExchanges, replayed, err := readRawSnapshots(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotInstruments) != len(instruments) || len(gotExchanges) != len(exchanges) {
		t.Fatalf("replayed counts = %d instruments/%d exchanges, want %d/%d", len(gotInstruments), len(gotExchanges), len(instruments), len(exchanges))
	}
	if replayed.Timestamp != written.Timestamp {
		t.Fatalf("replayed timestamp = %q, want %q", replayed.Timestamp, written.Timestamp)
	}
	if replayed.InstrumentsLatest == "" || replayed.ExchangesLatest == "" {
		t.Fatalf("latest aliases missing from replay summary: %#v", replayed)
	}
}

func TestRunNoFetchUsesRawSnapshotTimestamp(t *testing.T) {
	rawDir := t.TempDir()
	siteDataDir := t.TempDir()
	cacheDir := t.TempDir()
	instruments, exchanges, _ := catalogue.SampleData()
	builtAt := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	if _, err := writeRawSnapshots(rawDir, builtAt, instruments, exchanges); err != nil {
		t.Fatal(err)
	}

	err := run([]string{
		"refresh",
		"--no-fetch",
		"--input-raw-dir", rawDir,
		"--raw-dir", rawDir,
		"--site-data-dir", siteDataDir,
		"--manual-dir", filepath.Join("..", "..", "data", "manual"),
		"--cache-dir", cacheDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(siteDataDir, "build_manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		BuiltAt    string `json:"builtAt"`
		SourceMode string `json:"sourceMode"`
	}
	if err := json.Unmarshal(b, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.BuiltAt != "2026-05-09T12:00:00Z" || manifest.SourceMode != "raw_replay" {
		t.Fatalf("manifest = %#v, want deterministic raw replay timestamp/source", manifest)
	}
}
