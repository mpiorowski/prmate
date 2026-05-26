package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"pr/pkg/ai"
	"pr/pkg/git"
)

func TestBuildReviewPrompt(t *testing.T) {
	t.Parallel()

	got := buildReviewPrompt(git.PullRequest{Number: 123}, ".pr-context.md", "Return JSON only.")

	for _, want := range []string{
		"pull request #123",
		"source of truth",
		".pr-context.md",
		"PR title, PR description, and the local diff",
		"Return JSON only.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q: %q", want, got)
		}
	}
}

func TestParseLLMList(t *testing.T) {
	t.Parallel()

	got, err := parseLLMList("claude, codex,claude")
	if err != nil {
		t.Fatalf("parseLLMList returned error: %v", err)
	}

	want := []string{"claude", "codex"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestParseLLMListRejectsUnknownProvider(t *testing.T) {
	t.Parallel()

	if _, err := parseLLMList("claude,unknown"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseLLMListRejectsMoreThanTwo(t *testing.T) {
	t.Parallel()

	_, err := parseLLMList("claude,gemini,codex")
	if err == nil {
		t.Fatal("expected error for 3 LLMs")
	}
	if !strings.Contains(err.Error(), "at most") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestParseLLMListAcceptsOne(t *testing.T) {
	t.Parallel()

	got, err := parseLLMList("claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "claude" {
		t.Fatalf("got %v, want [claude]", got)
	}
}

func TestParseLLMListAcceptsTwo(t *testing.T) {
	t.Parallel()

	got, err := parseLLMList("claude,gemini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "claude" || got[1] != "gemini" {
		t.Fatalf("got %v, want [claude gemini]", got)
	}
}

func TestExtractJSONFromFencedBlockWithNestedCodeFenceText(t *testing.T) {
	t.Parallel()

	input := "done\n\n```json\n{\n  \"summary\": \"ok\",\n  \"verdict\": \"NEEDS_CHANGES\",\n  \"issues\": [\n    {\n      \"severity\": \"high\",\n      \"type\": \"correctness\",\n      \"location\": \"file.go:10\",\n      \"description\": \"desc\",\n      \"suggestion\": \"Use this snippet:\\n```go\\nvalue := 1\\n```\"\n    }\n  ],\n  \"positives\": []\n}\n```\n"

	got := extractJSON(input)

	var report ReviewReport
	if err := json.Unmarshal([]byte(got), &report); err != nil {
		t.Fatalf("extractJSON returned invalid JSON: %v\n%s", err, got)
	}
	if report.Summary != "ok" {
		t.Fatalf("got summary %q", report.Summary)
	}
}

func TestExtractJSONFallsBackToWholeInputWhenNoObjectFound(t *testing.T) {
	t.Parallel()

	input := "not json"
	if got := extractJSON(input); got != input {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func TestBuildCrossCheckPrompt(t *testing.T) {
	t.Parallel()

	pr := git.PullRequest{Number: 42}
	ownReview := `{"summary":"looks ok","verdict":"APPROVE","issues":[],"positives":["clean"]}`
	peerReview := `{"summary":"found issue","verdict":"NEEDS_CHANGES","issues":[],"positives":[]}`
	got := buildCrossCheckPrompt(pr, ".pr-context.md", "claude", ownReview, "codex", peerReview, "cross-check guidelines here")

	for _, want := range []string{
		"pull request #42",
		".pr-context.md",
		"<your_review provider=\"claude\">",
		ownReview,
		"</your_review>",
		"<peer_review provider=\"codex\">",
		peerReview,
		"</peer_review>",
		"cross-check guidelines here",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("cross-check prompt missing %q", want)
		}
	}
}

func TestBuildSynthesisPrompt(t *testing.T) {
	t.Parallel()

	pr := git.PullRequest{Number: 99}
	initialReviews := []reviewRunResult{
		{llm: "claude", response: ai.Response{FinalMessage: "claude review"}},
		{llm: "codex", response: ai.Response{FinalMessage: "codex review"}},
	}
	crossChecks := []textRunResult{
		{llm: "claude", output: "claude checked codex"},
		{llm: "codex", output: "codex checked claude"},
	}

	got := buildSynthesisPrompt(pr, ".pr-context.md", initialReviews, crossChecks, "synthesis guidelines")

	for _, want := range []string{
		"pull request #99",
		".pr-context.md",
		"<initial_review provider=\"claude\">",
		"claude review",
		"<initial_review provider=\"codex\">",
		"codex review",
		"<cross_check provider=\"claude\">",
		"claude checked codex",
		"<cross_check provider=\"codex\">",
		"codex checked claude",
		"synthesis guidelines",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("synthesis prompt missing %q", want)
		}
	}
}
