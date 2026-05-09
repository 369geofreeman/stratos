package enrichment

import (
	"reflect"
	"testing"
)

func TestCandidateSymbols(t *testing.T) {
	got := CandidateSymbols("VOD_L_EQ")
	want := []string{"VOD", "VOD.L", "VOD_L_EQ"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CandidateSymbols() = %#v, want %#v", got, want)
	}
}
