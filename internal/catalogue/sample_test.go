package catalogue

import (
	"testing"
	"time"
)

func TestSampleEnrichmentProfilesHaveFreshness(t *testing.T) {
	_, _, profiles := SampleData()
	if len(profiles) == 0 {
		t.Fatal("sample profiles are empty")
	}
	for ticker, profile := range profiles {
		if profile.Source == "" {
			t.Fatalf("%s sample profile has empty source", ticker)
		}
		if profile.RetrievedAt == "" {
			t.Fatalf("%s sample profile has empty retrievedAt", ticker)
		}
		if _, err := time.Parse(time.RFC3339, profile.RetrievedAt); err != nil {
			t.Fatalf("%s sample retrievedAt = %q: %v", ticker, profile.RetrievedAt, err)
		}
	}
}
