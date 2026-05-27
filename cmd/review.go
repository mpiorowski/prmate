package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/urfave/cli/v2"
	"pr/pkg/ai"
	"pr/pkg/git"
	"pr/pkg/prompts"
	"pr/pkg/ui"
)

type ReviewIssue struct {
	Severity    string `json:"severity"`
	Type        string `json:"type"`
	Location    string `json:"location"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

type ReviewReport struct {
	Summary   string        `json:"summary"`
	Verdict   string        `json:"verdict"`
	Issues    []ReviewIssue `json:"issues"`
	Positives []string      `json:"positives"`
}

type reviewRunResult struct {
	llm      string
	report   *ReviewReport
	err      error
	response ai.Response
}

type promptRun struct {
	llm    string
	prompt string
}

type textRunResult struct {
	llm      string
	output   string
	err      error
	response ai.Response
}

func ReviewCommand() *cli.Command {
	return &cli.Command{
		Name:     "review",
		Usage:    ui.FormatReadUsage("Review a GitHub PR and generate actionable feedback"),
		Category: ui.FormatCategory("Code & PRs"),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "pr",
				Usage:    "Required. GitHub PR number or PR URL to review",
				Aliases:  []string{"p"},
				Required: true,
			},
			&cli.StringFlag{
				Name:    "llm",
				Usage:   "LLM(s) to use. One provider runs a standard review; two comma-separated providers cross-check and merge findings. Options: claude, gemini, codex, opencode",
				Value:   "claude,codex",
				Aliases: []string{"l"},
			},
			ThinkingFlag(),
		},
		Action: func(c *cli.Context) error {
			thinking, thinkingExplicit := ThinkingFromContext(c)
			return runReviewWithThinking(c.String("pr"), c.String("llm"), thinking, thinkingExplicit)
		},
	}
}

func runReview(prValue string, llmValue string) error {
	return runReviewWithThinking(prValue, llmValue, defaultThinkingLevel, false)
}

func runReviewWithThinking(prValue string, llmValue string, thinking string, thinkingExplicit bool) error {
	llms, err := parseLLMList(llmValue)
	if err != nil {
		return err
	}
	if err := validateThinkingForLLMs(llms, thinking, thinkingExplicit); err != nil {
		return err
	}

	prRef, ok := git.ParsePullRequestRef(prValue)
	if !ok {
		return fmt.Errorf("--pr must be a GitHub PR number or PR URL")
	}

	ui.Step("Reviewing pull request %s", ui.Emphasize(prRef))

	ui.Step("Fetching PR metadata")
	pr, err := git.GetPullRequest(prRef)
	if err != nil {
		return err
	}

	ui.Step("Creating temporary worktree from PR head branch %s", ui.Emphasize(pr.HeadRefName))
	tempDir, fetchedRef, err := addWorktreeFromPullRequestHead(pr)
	if err != nil {
		return err
	}
	ui.Note("Worktree: %s", tempDir)
	defer cleanupWorktree(tempDir, fetchedRef)

	diff, err := git.DiffAgainstBase(tempDir, pr.BaseRefName)
	if err != nil {
		return err
	}

	contextPath, err := writePullRequestContextFile(tempDir, pr, diff)
	if err != nil {
		return err
	}

	if len(llms) == 1 {
		return runSingleReview(llms[0], tempDir, pr, contextPath, thinking, thinkingExplicit)
	}
	return runPairedReview(llms, tempDir, pr, contextPath, thinking, thinkingExplicit)
}

func extractJSON(input string) string {
	source := input
	re := regexp.MustCompile("(?s)```json\\s*")
	if loc := re.FindStringIndex(input); loc != nil {
		source = input[loc[1]:]
	}

	if extracted, ok := extractBalancedJSONObject(source); ok {
		return extracted
	}

	return input
}

func extractBalancedJSONObject(input string) (string, bool) {
	start := strings.Index(input, "{")
	if start == -1 {
		return "", false
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(input); i++ {
		ch := input[i]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return input[start : i+1], true
			}
		}
	}

	return "", false
}

func formatMarkdownReport(report *ReviewReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Code Review Summary\n\n**Verdict:** %s\n\n**Summary:** %s\n\n", report.Verdict, report.Summary))

	if len(report.Issues) > 0 {
		sb.WriteString("### Issues Found\n\n")
		for _, issue := range report.Issues {
			sb.WriteString(fmt.Sprintf("- **[%s] %s** at `%s`\n", strings.ToUpper(issue.Severity), issue.Type, issue.Location))
			sb.WriteString(fmt.Sprintf("  - %s\n", issue.Description))
			if issue.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("  - *Suggestion:* %s\n", issue.Suggestion))
			}
		}
		sb.WriteString("\n")
	}

	if len(report.Positives) > 0 {
		sb.WriteString("### Positives\n\n")
		for _, positive := range report.Positives {
			sb.WriteString(fmt.Sprintf("- %s\n", positive))
		}
	}

	return sb.String()
}

func buildReviewPrompt(pr git.PullRequest, contextPath string, guidelines string) string {
	return fmt.Sprintf(`Review pull request #%d.

Use the checked out PR branch in this worktree as the source of truth for the code under review.

Use %q for additional context. That file contains project context, the PR title, PR description, and the local diff against the PR base branch. Read the project context and follow relevant domain-context pointers before reviewing.

Treat the PR title and description as intent/context, but prioritize the actual code and tests in the worktree if they conflict.

Follow these review guidelines:
%s`, pr.Number, contextPath, guidelines)
}

func buildCrossCheckPrompt(pr git.PullRequest, contextPath string, reviewer string, ownReview string, peer string, peerReview string, guidelines string) string {
	return fmt.Sprintf(`Cross-check a peer review for pull request #%d.

Use the checked out PR branch in this worktree as the source of truth for the code under review.

Use %q for additional context. That file contains project context, the PR title, PR description, and the local diff against the PR base branch. Read the project context and follow relevant domain-context pointers before cross-checking.

You are %s. You already produced this independent review:

<your_review provider="%s">
%s
</your_review>

%s produced this independent review:

<peer_review provider="%s">
%s
</peer_review>

Follow these cross-check guidelines:
%s`, pr.Number, contextPath, reviewer, reviewer, ownReview, peer, peer, peerReview, guidelines)
}

func buildSynthesisPrompt(pr git.PullRequest, contextPath string, initialReviews []reviewRunResult, crossChecks []textRunResult, guidelines string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`Merge the reviewed findings for pull request #%d.

Use the checked out PR branch in this worktree as the source of truth for the code under review.

Use %q for additional context. That file contains project context, the PR title, PR description, and the local diff against the PR base branch. Read the project context and follow relevant domain-context pointers before synthesizing.

The reviewers first produced independent reviews, then each reviewer cross-checked the other review.

`, pr.Number, contextPath))

	for _, result := range initialReviews {
		sb.WriteString(fmt.Sprintf("<initial_review provider=\"%s\">\n%s\n</initial_review>\n\n", result.llm, result.response.FinalMessage))
	}

	for _, result := range crossChecks {
		sb.WriteString(fmt.Sprintf("<cross_check provider=\"%s\">\n%s\n</cross_check>\n\n", result.llm, result.output))
	}

	sb.WriteString("Follow these synthesis guidelines:\n")
	sb.WriteString(guidelines)
	return sb.String()
}

func parseLLMList(value string) ([]string, error) {
	seen := map[string]struct{}{}
	llms := []string{}

	for _, raw := range strings.Split(value, ",") {
		llm := strings.TrimSpace(raw)
		if llm == "" {
			continue
		}
		switch llm {
		case "claude", "codex", "gemini", "opencode":
		default:
			return nil, fmt.Errorf("unsupported llm %q", llm)
		}
		if _, ok := seen[llm]; ok {
			continue
		}
		seen[llm] = struct{}{}
		llms = append(llms, llm)
	}

	if len(llms) == 0 {
		return nil, fmt.Errorf("at least one llm is required")
	}
	if len(llms) > 2 {
		return nil, fmt.Errorf("at most 2 llms are supported")
	}

	return llms, nil
}

func validateThinkingForLLMs(llms []string, thinking string, thinkingExplicit bool) error {
	for _, llm := range llms {
		if err := ai.ValidateThinking(llm, thinking, thinkingExplicit); err != nil {
			return err
		}
	}
	return nil
}

func parseReviewReport(response ai.Response) (*ReviewReport, error) {
	jsonStr := extractJSON(response.FinalMessage)
	var report ReviewReport
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		return nil, fmt.Errorf("parse review JSON: %w", err)
	}
	return &report, nil
}

func runSingleReview(llm string, tempDir string, pr git.PullRequest, contextPath string, thinking string, thinkingExplicit bool) error {
	guidelines, err := prompts.Load("review")
	if err != nil {
		return fmt.Errorf("failed to load review guidelines: %w", err)
	}

	prompt := buildReviewPrompt(pr, contextPath, guidelines)
	ui.Step("Running review with %s", ui.Emphasize(llm))

	results := runReviews([]string{llm}, tempDir, prompt, true, thinking, thinkingExplicit)
	result := results[0]
	if result.err != nil {
		return reportReviewFailure(result)
	}

	fmt.Printf("\n%s\n", formatMarkdownReport(result.report))
	ui.Success("Review finished")
	return nil
}

func runPairedReview(llms []string, tempDir string, pr git.PullRequest, contextPath string, thinking string, thinkingExplicit bool) error {
	reviewGuidelines, err := prompts.Load("review")
	if err != nil {
		return fmt.Errorf("failed to load review guidelines: %w", err)
	}
	crossCheckGuidelines, err := prompts.Load("review-cross-check")
	if err != nil {
		return fmt.Errorf("failed to load cross-check guidelines: %w", err)
	}
	synthesisGuidelines, err := prompts.Load("review-synthesis")
	if err != nil {
		return fmt.Errorf("failed to load synthesis guidelines: %w", err)
	}

	ui.Step("Round 1/3: running independent reviews with %s and %s", ui.Emphasize(llms[0]), ui.Emphasize(llms[1]))
	initialPrompt := buildReviewPrompt(pr, contextPath, reviewGuidelines)
	initialReviews := runReviews(llms, tempDir, initialPrompt, false, thinking, thinkingExplicit)
	for _, result := range initialReviews {
		if result.err != nil {
			return reportReviewFailure(result)
		}
	}

	ui.Step("Round 2/3: cross-checking findings both ways")
	crossCheckPrompts := []promptRun{
		{
			llm: llms[0],
			prompt: buildCrossCheckPrompt(
				pr,
				contextPath,
				llms[0],
				initialReviews[0].response.FinalMessage,
				llms[1],
				initialReviews[1].response.FinalMessage,
				crossCheckGuidelines,
			),
		},
		{
			llm: llms[1],
			prompt: buildCrossCheckPrompt(
				pr,
				contextPath,
				llms[1],
				initialReviews[1].response.FinalMessage,
				llms[0],
				initialReviews[0].response.FinalMessage,
				crossCheckGuidelines,
			),
		},
	}
	crossChecks := runTextPrompts(tempDir, "cross-check", crossCheckPrompts, false, thinking, thinkingExplicit)
	for _, result := range crossChecks {
		if result.err != nil {
			return reportTextFailure("cross-check", result)
		}
	}

	ui.Step("Round 3/3: merging validated findings with %s", ui.Emphasize(llms[0]))
	synthesisPrompt := buildSynthesisPrompt(pr, contextPath, initialReviews, crossChecks, synthesisGuidelines)
	response, err := ai.RunWithOptions(tempDir, synthesisPrompt, llms[0], ai.ReadOnly, ai.RunOptions{ShowProgress: true, Verbose: false, Thinking: thinking, ThinkingExplicit: thinkingExplicit})
	if err != nil {
		return fmt.Errorf("synthesis failed: %w", err)
	}

	report, err := parseReviewReport(response)
	if err != nil {
		if strings.TrimSpace(response.FinalMessage) != "" {
			ui.Note("%s final message:\n%s", llms[0], response.FinalMessage)
		}
		return fmt.Errorf("failed to parse synthesized review: %w", err)
	}

	fmt.Printf("\n%s\n", formatMarkdownReport(report))
	ui.Success("Paired review finished")
	return nil
}

func runReviews(llms []string, tempDir string, prompt string, verbose bool, thinking string, thinkingExplicit bool) []reviewRunResult {
	results := make([]reviewRunResult, len(llms))
	prompts := make([]promptRun, len(llms))
	for i, llm := range llms {
		prompts[i] = promptRun{llm: llm, prompt: prompt}
	}

	textResults := runTextPrompts(tempDir, "review", prompts, verbose, thinking, thinkingExplicit)
	for i, textResult := range textResults {
		result := reviewRunResult{llm: textResult.llm, err: textResult.err, response: textResult.response}
		if result.err == nil {
			report, parseErr := parseReviewReport(textResult.response)
			if parseErr != nil {
				result.err = parseErr
			} else {
				result.report = report
			}
		}
		results[i] = result
	}

	return results
}

func runTextPrompts(tempDir string, label string, prompts []promptRun, verbose bool, thinking string, thinkingExplicit bool) []textRunResult {
	results := make([]textRunResult, len(prompts))
	statuses := make([]string, len(prompts))
	for i := range statuses {
		statuses[i] = "queued"
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	stop := make(chan struct{})
	done := make(chan struct{})
	if !verbose {
		go func() {
			spinner := []string{"|", "/", "-", "\\"}
			idx := 0
			ticker := time.NewTicker(120 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					fmt.Fprint(os.Stderr, "\r\033[K")
					close(done)
					return
				case <-ticker.C:
					mu.Lock()
					parts := make([]string, len(prompts))
					for j, prompt := range prompts {
						parts[j] = fmt.Sprintf("(%s/%s)=%s", prompt.llm, ai.ReadOnly.ColoredLabel(), statuses[j])
					}
					mu.Unlock()
					fmt.Fprintf(os.Stderr, "\r\033[K%s AI %s: %s", spinner[idx], label, strings.Join(parts, " | "))
					idx = (idx + 1) % len(spinner)
				}
			}
		}()
	}

	for i, prompt := range prompts {
		wg.Add(1)
		go func(idx int, run promptRun) {
			defer wg.Done()
			mu.Lock()
			statuses[idx] = "running"
			mu.Unlock()

			response, err := ai.RunWithOptions(tempDir, run.prompt, run.llm, ai.ReadOnly, ai.RunOptions{ShowProgress: false, Verbose: verbose, Thinking: thinking, ThinkingExplicit: thinkingExplicit})
			result := textRunResult{llm: run.llm, response: response, err: err}
			if err == nil {
				result.output = response.FinalMessage
			}

			mu.Lock()
			if result.err != nil {
				statuses[idx] = "failed"
			} else {
				statuses[idx] = "done"
			}
			results[idx] = result
			mu.Unlock()
		}(i, prompt)
	}

	wg.Wait()
	if !verbose {
		close(stop)
		<-done
	}

	return results
}

func reportReviewFailure(result reviewRunResult) error {
	ui.Error("%s review failed: %v", result.llm, result.err)
	if strings.TrimSpace(result.response.FinalMessage) != "" {
		ui.Note("%s final message:\n%s", result.llm, result.response.FinalMessage)
	}
	return fmt.Errorf("review failed")
}

func reportTextFailure(label string, result textRunResult) error {
	ui.Error("%s %s failed: %v", result.llm, label, result.err)
	if strings.TrimSpace(result.response.FinalMessage) != "" {
		ui.Note("%s final message:\n%s", result.llm, result.response.FinalMessage)
	}
	return fmt.Errorf("%s failed", label)
}
