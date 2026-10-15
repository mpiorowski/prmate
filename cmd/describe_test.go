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

func TestDescribeTarget(t *testing.T) {
	for _, tt := range []struct {
		name, prRef, base, want, wantPR, wantErr string
		openPR                                   int
		lookupErr, findErr, fetchErr             error
	}{
		{name: "new PR", want: "origin/main"},
		{name: "stacked PR", prRef: "25624", base: "mateu/account_opening_dynamic_fields", want: "origin/mateu/account_opening_dynamic_fields", wantPR: "25624"},
		{name: "URL", prRef: "https://github.com/example/repo/pull/25624", base: "develop", want: "origin/develop", wantPR: "https://github.com/example/repo/pull/25624"},
		{name: "branch with open stacked PR", openPR: 29530, base: "mateu/ao_schwab_user_feedback", want: "origin/mateu/ao_schwab_user_feedback", wantPR: "29530"},
		{name: "metadata error", prRef: "25624", lookupErr: errors.New("unavailable"), wantErr: "failed to get PR metadata"},
		{name: "open PR lookup error", findErr: errors.New("unavailable"), wantErr: "failed to look up open PR for feature"},
		{name: "missing base", prRef: "25624", wantErr: "has no base branch"},
		{name: "fetch error", prRef: "25624", base: "stack/parent", fetchErr: errors.New("unavailable"), wantErr: "failed to fetch PR base branch"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fetched := false
			getPR := func(ref string) (git.PullRequest, error) {
				if ref == "" || ref != tt.prRef {
					t.Fatalf("unexpected PR lookup %q", ref)
				}
				return git.PullRequest{Number: 25624, BaseRefName: tt.base}, tt.lookupErr
			}
			findPR := func(branch string) (git.PullRequest, bool, error) {
				if tt.prRef != "" || branch != "feature" {
					t.Fatalf("unexpected open PR lookup %q", branch)
				}
				return git.PullRequest{Number: tt.openPR, BaseRefName: tt.base}, tt.openPR != 0, tt.findErr
			}
			fetch := func(base string) error {
				if base == "" || base != tt.base {
					t.Fatalf("unexpected fetch %q", base)
				}
				fetched = true
				return tt.fetchErr
			}
			got, gotPR, err := describeTarget(tt.prRef, "feature", getPR, findPR, fetch)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want || gotPR != tt.wantPR {
				t.Fatalf("got %q, %q, %v; want %q, %q", got, gotPR, err, tt.want, tt.wantPR)
			}
			if fetched != (tt.wantPR != "") {
				t.Fatalf("unexpected fetch state: %v", fetched)
			}
		})
	}
}
