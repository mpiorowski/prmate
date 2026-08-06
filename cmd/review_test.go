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
		"the project guide, project context, the PR title, PR description, and the local diff",
		"Read the project guide first",
		"Return JSON only.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q: %q", want, got)
		}
	}
}

func TestParseLLMSpecs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  []ai.Spec
	}{
		{
			name:  "dedupes repeated providers",
			value: "claude, codex,claude",
			want:  []ai.Spec{{Provider: "claude"}, {Provider: "codex"}},
		},
		{
			name:  "accepts one",
			value: "claude",
			want:  []ai.Spec{{Provider: "claude"}},
		},
		{
			name:  "accepts two providers",
			value: "claude,gemini",
			want:  []ai.Spec{{Provider: "claude"}, {Provider: "gemini"}},
		},
		{
			name:  "accepts the same provider with two models",
			value: "claude:opus,claude:sonnet",
			want:  []ai.Spec{{Provider: "claude", Model: "opus"}, {Provider: "claude", Model: "sonnet"}},
		},
		{
			name:  "keeps a bare provider and a model override apart",
			value: "claude, claude:claude-opus-5",
			want:  []ai.Spec{{Provider: "claude"}, {Provider: "claude", Model: "claude-opus-5"}},
		},
		{
			name:  "dedupes repeated provider and model pairs",
			value: "claude:opus,claude:opus",
			want:  []ai.Spec{{Provider: "claude", Model: "opus"}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseLLMSpecs(tt.value)
			if err != nil {
				t.Fatalf("parseLLMSpecs returned error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestParseLLMSpecsRejectsUnknownProvider(t *testing.T) {
	t.Parallel()

	if _, err := parseLLMSpecs("claude,unknown"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := parseLLMSpecs("unknown:opus"); err == nil {
		t.Fatal("expected error for unknown provider with a model")
	}
}

func TestParseLLMSpecsRejectsEmptyModel(t *testing.T) {
	t.Parallel()

	_, err := parseLLMSpecs("claude:")
	if err == nil {
		t.Fatal("expected error for empty model")
	}
	if !strings.Contains(err.Error(), "missing a model") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestParseLLMSpecsRejectsMoreThanTwo(t *testing.T) {
	t.Parallel()

	_, err := parseLLMSpecs("claude,gemini,codex")
	if err == nil {
		t.Fatal("expected error for 3 LLMs")
	}
	if !strings.Contains(err.Error(), "at most") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateThinkingForSpecs(t *testing.T) {
	t.Parallel()

	if err := validateThinkingForSpecs([]ai.Spec{{Provider: "codex"}, {Provider: "claude"}}, "xhigh", true); err != nil {
		t.Fatalf("validateThinkingForSpecs returned error: %v", err)
	}
	if err := validateThinkingForSpecs([]ai.Spec{{Provider: "claude", Model: "opus"}}, "max", true); err != nil {
		t.Fatalf("model override should not change thinking validation, got: %v", err)
	}
	if err := validateThinkingForSpecs([]ai.Spec{{Provider: "gemini"}}, "high", false); err != nil {
		t.Fatalf("implicit gemini default should be ignored, got: %v", err)
	}
	if err := validateThinkingForSpecs([]ai.Spec{{Provider: "gemini"}}, "high", true); err == nil {
		t.Fatal("expected gemini thinking error")
	}
	if err := validateThinkingForSpecs([]ai.Spec{{Provider: "codex"}}, "max", true); err == nil {
		t.Fatal("expected codex max thinking error")
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
		{spec: ai.Spec{Provider: "claude"}, response: ai.Response{FinalMessage: "claude review"}},
		{spec: ai.Spec{Provider: "codex"}, response: ai.Response{FinalMessage: "codex review"}},
	}
	crossChecks := []textRunResult{
		{spec: ai.Spec{Provider: "claude"}, output: "claude checked codex"},
		{spec: ai.Spec{Provider: "codex"}, output: "codex checked claude"},
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

func TestBuildSynthesisPromptLabelsTwoModelsOfOneProvider(t *testing.T) {
	t.Parallel()

	initialReviews := []reviewRunResult{
		{spec: ai.Spec{Provider: "claude", Model: "opus"}, response: ai.Response{FinalMessage: "opus review"}},
		{spec: ai.Spec{Provider: "claude", Model: "sonnet"}, response: ai.Response{FinalMessage: "sonnet review"}},
	}
	crossChecks := []textRunResult{
		{spec: ai.Spec{Provider: "claude", Model: "opus"}, output: "opus checked sonnet"},
		{spec: ai.Spec{Provider: "claude", Model: "sonnet"}, output: "sonnet checked opus"},
	}

	got := buildSynthesisPrompt(git.PullRequest{Number: 7}, ".pr-context.md", initialReviews, crossChecks, "synthesis guidelines")

	for _, want := range []string{
		"<initial_review provider=\"claude:opus\">",
		"<initial_review provider=\"claude:sonnet\">",
		"<cross_check provider=\"claude:opus\">",
		"<cross_check provider=\"claude:sonnet\">",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("synthesis prompt missing %q", want)
		}
	}
}
