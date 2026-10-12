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

// ArchFinding is one architecture comment. It carries no severity and no
// verdict: the architecture track never gates a merge, so the shape cannot
// express one.
type ArchFinding struct {
	Location     string `json:"location"`
	DivergesFrom string `json:"diverges_from"`
	Description  string `json:"description"`
	Suggestion   string `json:"suggestion"`
}

type ArchReport struct {
	Summary   string        `json:"summary"`
	Findings  []ArchFinding `json:"findings"`
	Positives []string      `json:"positives"`
}

// reviewTrack is one independent pass over the PR. Each track has its own
// guidelines, its own report shape, and its own cross-check and synthesis;
// the tracks only meet when the reports are printed.
type reviewTrack int

const (
	trackBugs reviewTrack = iota
	trackArchitecture
)

func (t reviewTrack) label() string {
	switch t {
	case trackBugs:
		return "bugs"
	case trackArchitecture:
		return "arch"
	}
	panic(fmt.Sprintf("unknown review track %d", t))
}

type trackGuidelines struct {
	review     string
	crossCheck string
	synthesis  string
}

func loadTrackGuidelines(t reviewTrack) (trackGuidelines, error) {
	var names [3]string
	switch t {
	case trackBugs:
		names = [3]string{"review", "review-cross-check", "review-synthesis"}
	case trackArchitecture:
		names = [3]string{"review-arch", "review-arch-cross-check", "review-arch-synthesis"}
	default:
		panic(fmt.Sprintf("unknown review track %d", t))
	}

	var loaded [3]string
	for i, name := range names {
		content, err := prompts.Load(name)
		if err != nil {
			return trackGuidelines{}, fmt.Errorf("failed to load %s guidelines: %w", t.label(), err)
		}
		loaded[i] = content
	}
	return trackGuidelines{review: loaded[0], crossCheck: loaded[1], synthesis: loaded[2]}, nil
}

// checkTrackReport confirms a round-1 response parses to the track's report
// shape, so a malformed review fails before it is fed to the cross-check.
func checkTrackReport(t reviewTrack, response ai.Response) error {
	switch t {
	case trackBugs:
		_, err := parseReviewReport(response)
		return err
	case trackArchitecture:
		_, err := parseArchReport(response)
		return err
	}
	panic(fmt.Sprintf("unknown review track %d", t))
}

func renderTrackReport(t reviewTrack, response ai.Response) (string, error) {
	switch t {
	case trackBugs:
		report, err := parseReviewReport(response)
		if err != nil {
			return "", err
		}
		return formatMarkdownReport(report), nil
	case trackArchitecture:
		report, err := parseArchReport(response)
		if err != nil {
			return "", err
		}
		return formatArchMarkdownReport(report), nil
	}
	panic(fmt.Sprintf("unknown review track %d", t))
}

type reviewRunResult struct {
	spec     ai.Spec
	report   *ReviewReport
	err      error
	response ai.Response
}

type promptRun struct {
	spec   ai.Spec
	track  reviewTrack
	prompt string
}

type textRunResult struct {
	spec     ai.Spec
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
				Usage:   "LLM(s) to use. One reviewer runs a standard review; two comma-separated reviewers cross-check and merge findings. Providers: claude, gemini, codex, opencode. Append :<model> to pick a model, e.g. claude:opus,claude:sonnet",
				Value:   "codex,claude",
				Aliases: []string{"l"},
			},
			&cli.BoolFlag{
				Name:  "no-arch",
				Usage: "Skip the architecture track and run the bug review only. Only applies with two reviewers",
			},
			ThinkingFlag(),
		},
		Action: func(c *cli.Context) error {
			thinking, thinkingExplicit := ThinkingFromContext(c)
			return runReviewWithThinking(c.String("pr"), c.String("llm"), tracksForReview(c.Bool("no-arch")), thinking, thinkingExplicit)
		},
	}
}

func tracksForReview(noArch bool) []reviewTrack {
	if noArch {
		return []reviewTrack{trackBugs}
	}
	return []reviewTrack{trackBugs, trackArchitecture}
}

func runReviewWithThinking(prValue string, llmValue string, tracks []reviewTrack, thinking string, thinkingExplicit bool) error {
	specs, err := parseLLMSpecs(llmValue)
	if err != nil {
		return err
	}
	if err := validateThinkingForSpecs(specs, thinking, thinkingExplicit); err != nil {
		return err
	}

	prRef, ok := git.ParsePullRequestRef(prValue)
	if !ok {
		return fmt.Errorf("--pr must be a GitHub PR number or PR URL")
	}

	guide, err := requireProjectGuide()
	if err != nil {
		return err
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

	contextPath, err := writePullRequestContextFile(tempDir, guide, pr, diff)
	if err != nil {
		return err
	}

	if len(specs) == 1 {
		if len(tracks) > 1 {
			ui.Note("Architecture track needs two reviewers to cross-check; running the bug review only")
		}
		return runSingleReview(specs[0], tempDir, pr, contextPath, thinking, thinkingExplicit)
	}
	return runPairedReview(specs, tempDir, pr, contextPath, tracks, thinking, thinkingExplicit)
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

func formatArchMarkdownReport(report *ArchReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Architecture Review Summary\n\n**Summary:** %s\n\n", report.Summary))

	if len(report.Findings) > 0 {
		sb.WriteString("### Findings\n\n")
		for _, finding := range report.Findings {
			sb.WriteString(fmt.Sprintf("- **`%s`** diverges from `%s`\n", finding.Location, finding.DivergesFrom))
			sb.WriteString(fmt.Sprintf("  - %s\n", finding.Description))
			if finding.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("  - *Suggestion:* %s\n", finding.Suggestion))
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

Use %q for additional context. That file contains the project guide, project context, the PR title, PR description, and the local diff against the PR base branch. Read the project guide first and review against it; then read the project context and follow relevant domain-context pointers.

Treat the PR title and description as intent/context, but prioritize the actual code and tests in the worktree if they conflict.

Follow these review guidelines:
%s`, pr.Number, contextPath, guidelines)
}

func buildCrossCheckPrompt(pr git.PullRequest, contextPath string, reviewer string, ownReview string, peer string, peerReview string, guidelines string) string {
	return fmt.Sprintf(`Cross-check a peer review for pull request #%d.

Use the checked out PR branch in this worktree as the source of truth for the code under review.

Use %q for additional context. That file contains the project guide, project context, the PR title, PR description, and the local diff against the PR base branch. Read the project guide first and cross-check against it; then read the project context and follow relevant domain-context pointers.

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

func buildSynthesisPrompt(pr git.PullRequest, contextPath string, initialReviews []textRunResult, crossChecks []textRunResult, guidelines string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`Merge the reviewed findings for pull request #%d.

Use the checked out PR branch in this worktree as the source of truth for the code under review.

Use %q for additional context. That file contains the project guide, project context, the PR title, PR description, and the local diff against the PR base branch. Read the project guide first and synthesize against it; then read the project context and follow relevant domain-context pointers.

The reviewers first produced independent reviews, then each reviewer cross-checked the other review.

`, pr.Number, contextPath))

	for _, result := range initialReviews {
		sb.WriteString(fmt.Sprintf("<initial_review provider=\"%s\">\n%s\n</initial_review>\n\n", result.spec.String(), result.output))
	}

	for _, result := range crossChecks {
		sb.WriteString(fmt.Sprintf("<cross_check provider=\"%s\">\n%s\n</cross_check>\n\n", result.spec.String(), result.output))
	}

	sb.WriteString("Follow these synthesis guidelines:\n")
	sb.WriteString(guidelines)
	return sb.String()
}

// parseLLMSpecs parses a comma-separated reviewer list. Each entry is a
// provider ("claude") or a provider with a model override ("claude:opus"), so
// the same provider can review twice under two different models.
func parseLLMSpecs(value string) ([]ai.Spec, error) {
	seen := map[string]struct{}{}
	specs := []ai.Spec{}

	for _, raw := range strings.Split(value, ",") {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		spec, err := ai.ParseSpec(entry)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[spec.String()]; ok {
			continue
		}
		seen[spec.String()] = struct{}{}
		specs = append(specs, spec)
	}

	if len(specs) == 0 {
		return nil, fmt.Errorf("at least one llm is required")
	}
	if len(specs) > 2 {
		return nil, fmt.Errorf("at most 2 llms are supported")
	}

	return specs, nil
}

func validateThinkingForSpecs(specs []ai.Spec, thinking string, thinkingExplicit bool) error {
	for _, spec := range specs {
		if err := ai.ValidateThinking(spec.Provider, thinking, thinkingExplicit); err != nil {
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

func parseArchReport(response ai.Response) (*ArchReport, error) {
	jsonStr := extractJSON(response.FinalMessage)
	var report ArchReport
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		return nil, fmt.Errorf("parse architecture review JSON: %w", err)
	}
	return &report, nil
}

func runSingleReview(spec ai.Spec, tempDir string, pr git.PullRequest, contextPath string, thinking string, thinkingExplicit bool) error {
	guidelines, err := prompts.Load("review")
	if err != nil {
		return fmt.Errorf("failed to load review guidelines: %w", err)
	}

	prompt := buildReviewPrompt(pr, contextPath, guidelines)
	ui.Step("Running review with %s", ui.Emphasize(spec.String()))

	results := runReviews([]ai.Spec{spec}, tempDir, prompt, true, thinking, thinkingExplicit)
	result := results[0]
	if result.err != nil {
		return reportReviewFailure(result)
	}

	fmt.Printf("\n%s\n", formatMarkdownReport(result.report))
	ui.Success("Review finished")
	return nil
}

func runPairedReview(specs []ai.Spec, tempDir string, pr git.PullRequest, contextPath string, tracks []reviewTrack, thinking string, thinkingExplicit bool) error {
	guidelines := map[reviewTrack]trackGuidelines{}
	for _, track := range tracks {
		loaded, err := loadTrackGuidelines(track)
		if err != nil {
			return err
		}
		guidelines[track] = loaded
	}

	ui.Step("Round 1/3: running independent %s reviews with %s and %s", ui.Emphasize(trackLabels(tracks)), ui.Emphasize(specs[0].String()), ui.Emphasize(specs[1].String()))
	var prompts []promptRun
	for _, track := range tracks {
		prompt := buildReviewPrompt(pr, contextPath, guidelines[track].review)
		for _, spec := range specs {
			prompts = append(prompts, promptRun{spec: spec, track: track, prompt: prompt})
		}
	}
	initialReviews := map[reviewTrack][]textRunResult{}
	for i, result := range runTextPrompts(tempDir, "review", prompts, false, thinking, thinkingExplicit) {
		track := prompts[i].track
		if result.err != nil {
			return reportTextFailure(track.label()+" review", result)
		}
		if err := checkTrackReport(track, result.response); err != nil {
			return reportTextFailure(track.label()+" review", textRunResult{spec: result.spec, err: err, response: result.response})
		}
		initialReviews[track] = append(initialReviews[track], result)
	}

	ui.Step("Round 2/3: cross-checking findings both ways")
	prompts = nil
	for _, track := range tracks {
		reviews := initialReviews[track]
		for i := range specs {
			own := reviews[i]
			peer := reviews[1-i]
			prompts = append(prompts, promptRun{
				spec:   own.spec,
				track:  track,
				prompt: buildCrossCheckPrompt(pr, contextPath, own.spec.String(), own.output, peer.spec.String(), peer.output, guidelines[track].crossCheck),
			})
		}
	}
	crossChecks := map[reviewTrack][]textRunResult{}
	for i, result := range runTextPrompts(tempDir, "cross-check", prompts, false, thinking, thinkingExplicit) {
		track := prompts[i].track
		if result.err != nil {
			return reportTextFailure(track.label()+" cross-check", result)
		}
		crossChecks[track] = append(crossChecks[track], result)
	}

	ui.Step("Round 3/3: merging validated findings with %s", ui.Emphasize(specs[0].String()))
	prompts = nil
	for _, track := range tracks {
		prompts = append(prompts, promptRun{
			spec:   specs[0],
			track:  track,
			prompt: buildSynthesisPrompt(pr, contextPath, initialReviews[track], crossChecks[track], guidelines[track].synthesis),
		})
	}
	var rendered []string
	for i, result := range runTextPrompts(tempDir, "synthesis", prompts, false, thinking, thinkingExplicit) {
		track := prompts[i].track
		if result.err != nil {
			return reportTextFailure(track.label()+" synthesis", result)
		}
		markdown, err := renderTrackReport(track, result.response)
		if err != nil {
			if strings.TrimSpace(result.output) != "" {
				ui.Note("%s final message:\n%s", result.spec.String(), result.output)
			}
			return fmt.Errorf("failed to parse synthesized %s review: %w", track.label(), err)
		}
		rendered = append(rendered, markdown)
	}

	fmt.Printf("\n%s\n", strings.Join(rendered, "\n"))
	ui.Success("Paired review finished")
	return nil
}

func trackLabels(tracks []reviewTrack) string {
	labels := make([]string, len(tracks))
	for i, track := range tracks {
		labels[i] = track.label()
	}
	return strings.Join(labels, "+")
}

func runReviews(specs []ai.Spec, tempDir string, prompt string, verbose bool, thinking string, thinkingExplicit bool) []reviewRunResult {
	results := make([]reviewRunResult, len(specs))
	prompts := make([]promptRun, len(specs))
	for i, spec := range specs {
		prompts[i] = promptRun{spec: spec, track: trackBugs, prompt: prompt}
	}

	textResults := runTextPrompts(tempDir, "review", prompts, verbose, thinking, thinkingExplicit)
	for i, textResult := range textResults {
		result := reviewRunResult{spec: textResult.spec, err: textResult.err, response: textResult.response}
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
						parts[j] = fmt.Sprintf("(%s/%s/%s)=%s", prompt.spec.String(), prompt.track.label(), ai.ReadOnly.ColoredLabel(), statuses[j])
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

			response, err := ai.RunWithOptions(tempDir, run.prompt, run.spec, ai.ReadOnly, ai.RunOptions{ShowProgress: false, Verbose: verbose, Thinking: thinking, ThinkingExplicit: thinkingExplicit})
			result := textRunResult{spec: run.spec, response: response, err: err}
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
	ui.Error("%s review failed: %v", result.spec.String(), result.err)
	if strings.TrimSpace(result.response.FinalMessage) != "" {
		ui.Note("%s final message:\n%s", result.spec.String(), result.response.FinalMessage)
	}
	return fmt.Errorf("review failed")
}

func reportTextFailure(label string, result textRunResult) error {
	ui.Error("%s %s failed: %v", result.spec.String(), label, result.err)
	if strings.TrimSpace(result.response.FinalMessage) != "" {
		ui.Note("%s final message:\n%s", result.spec.String(), result.response.FinalMessage)
	}
	return fmt.Errorf("%s failed", label)
}
