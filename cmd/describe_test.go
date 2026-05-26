package cmd

import (
	"strings"
	"testing"
)

func TestRunDescribeRejectsInvalidPRRef(t *testing.T) {
	t.Parallel()

	err := runDescribe("", "not-a-pr", "claude")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--pr must be a valid GitHub PR number or PR URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}
