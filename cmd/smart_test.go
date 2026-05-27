package cmd

import (
	"errors"
	"strings"
	"testing"

	"pr/pkg/git"
)

func TestRunSmartDescribesWhenNoOpenPRExists(t *testing.T) {
	t.Parallel()

	var describeCalled bool
	var reviewCalled bool

	err := runSmartWithOps(smartOps{
		currentBranch: func() (string, error) {
			return "feature/test", nil
		},
		currentPullRequest: func() (git.PullRequest, bool, error) {
			return git.PullRequest{}, false, nil
		},
		describe: func(branch string, prRef string, llm string) error {
			describeCalled = true
			if branch != "" || prRef != "" || llm != smartDescribeLLM {
				t.Fatalf("describe args = (%q, %q, %q)", branch, prRef, llm)
			}
			return nil
		},
		review: func(prRef string, llms string) error {
			reviewCalled = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runSmartWithOps returned error: %v", err)
	}
	if !describeCalled {
		t.Fatal("expected describe to be called")
	}
	if reviewCalled {
		t.Fatal("did not expect review to be called")
	}
}

func TestRunSmartReviewsWhenOpenPRExists(t *testing.T) {
	t.Parallel()

	var describeCalled bool
	var reviewCalled bool

	err := runSmartWithOps(smartOps{
		currentBranch: func() (string, error) {
			return "feature/test", nil
		},
		currentPullRequest: func() (git.PullRequest, bool, error) {
			return git.PullRequest{Number: 123}, true, nil
		},
		describe: func(branch string, prRef string, llm string) error {
			describeCalled = true
			return nil
		},
		review: func(prRef string, llms string) error {
			reviewCalled = true
			if prRef != "123" || llms != smartReviewLLMs {
				t.Fatalf("review args = (%q, %q)", prRef, llms)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runSmartWithOps returned error: %v", err)
	}
	if describeCalled {
		t.Fatal("did not expect describe to be called")
	}
	if !reviewCalled {
		t.Fatal("expected review to be called")
	}
}

func TestRunSmartRejectsDetachedHead(t *testing.T) {
	t.Parallel()

	err := runSmartWithOps(smartOps{
		currentBranch: func() (string, error) {
			return "", nil
		},
		currentPullRequest: func() (git.PullRequest, bool, error) {
			t.Fatal("currentPullRequest should not be called")
			return git.PullRequest{}, false, nil
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not currently on any branch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSmartWrapsPullRequestInspectionError(t *testing.T) {
	t.Parallel()

	err := runSmartWithOps(smartOps{
		currentBranch: func() (string, error) {
			return "feature/test", nil
		},
		currentPullRequest: func() (git.PullRequest, bool, error) {
			return git.PullRequest{}, false, errors.New("gh failed")
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to inspect pull requests") {
		t.Fatalf("unexpected error: %v", err)
	}
}
