package cmd

import (
	"errors"
	"strings"
	"testing"

	"pr/pkg/git"
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

func TestDescribeBaseRef(t *testing.T) {
	for _, tt := range []struct {
		name, prRef, base, want, wantErr string
		lookupErr, fetchErr              error
	}{
		{name: "new PR", want: "origin/main"},
		{name: "stacked PR", prRef: "25624", base: "mateu/account_opening_dynamic_fields", want: "origin/mateu/account_opening_dynamic_fields"},
		{name: "URL", prRef: "https://github.com/example/repo/pull/25624", base: "develop", want: "origin/develop"},
		{name: "metadata error", prRef: "25624", lookupErr: errors.New("unavailable"), wantErr: "failed to get PR metadata"},
		{name: "missing base", prRef: "25624", wantErr: "has no base branch"},
		{name: "fetch error", prRef: "25624", base: "stack/parent", fetchErr: errors.New("unavailable"), wantErr: "failed to fetch PR base branch"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fetched := false
			got, err := describeBaseRef(tt.prRef, func(ref string) (git.PullRequest, error) {
				if ref == "" || ref != tt.prRef {
					t.Fatalf("unexpected PR lookup %q", ref)
				}
				return git.PullRequest{BaseRefName: tt.base}, tt.lookupErr
			}, func(base string) error {
				if base == "" || base != tt.base {
					t.Fatalf("unexpected fetch %q", base)
				}
				fetched = true
				return tt.fetchErr
			})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
			if fetched != (tt.prRef != "") {
				t.Fatalf("unexpected fetch state: %v", fetched)
			}
		})
	}
}
