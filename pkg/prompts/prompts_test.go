package prompts

import (
	"testing"
)

func TestLoadReturnsContent(t *testing.T) {
	t.Parallel()

	got, err := Load("describe")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got == "" {
		t.Fatal("Load returned empty string")
	}
}

func TestLoadReturnsErrorForMissing(t *testing.T) {
	t.Parallel()

	_, err := Load("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing guideline, got nil")
	}
}
