package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pr/pkg/git"
)

func writePullRequestContextFileAs(dir string, name string, pr git.PullRequest, diff string) (string, error) {
	filename := filepath.Join(dir, name)
	content := fmt.Sprintf(
		"# Pull Request Context\n\n"+
			"- Number: %d\n"+
			"- URL: %s\n"+
			"- State: %s\n"+
			"- Base branch: %s\n"+
			"- Head branch: %s\n"+
			"- Merged at: %s\n\n"+
			"## Title\n\n"+
			"%s\n\n"+
			"## Body\n\n"+
			"%s\n\n"+
			"## Diff\n\n"+
			"```diff\n%s\n```\n",
		pr.Number,
		pr.URL,
		pr.State,
		pr.BaseRefName,
		pr.HeadRefName,
		pr.MergedAt,
		pr.Title,
		strings.TrimSpace(pr.Body),
		diff,
	)

	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write PR context file: %w", err)
	}

	return filepath.Base(filename), nil
}

func writePullRequestContextFile(dir string, pr git.PullRequest, diff string) (string, error) {
	filename := filepath.Join(dir, ".pr-context.md")
	content := fmt.Sprintf(
		"# Pull Request Context\n\n"+
			"- Number: %d\n"+
			"- URL: %s\n"+
			"- State: %s\n"+
			"- Base branch: %s\n"+
			"- Head branch: %s\n"+
			"- Merged at: %s\n\n"+
			"## Title\n\n"+
			"%s\n\n"+
			"## Body\n\n"+
			"%s\n\n"+
			"## Diff\n\n"+
			"```diff\n%s\n```\n",
		pr.Number,
		pr.URL,
		pr.State,
		pr.BaseRefName,
		pr.HeadRefName,
		pr.MergedAt,
		pr.Title,
		strings.TrimSpace(pr.Body),
		diff,
	)

	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write PR context file: %w", err)
	}

	return filepath.Base(filename), nil
}

func writePullRequestContextFileWithoutDiff(dir string, pr git.PullRequest) (string, error) {
	filename := filepath.Join(dir, ".pr-context.md")
	content := fmt.Sprintf(
		"# Pull Request Context\n\n"+
			"- Number: %d\n"+
			"- URL: %s\n"+
			"- State: %s\n"+
			"- Base branch: %s\n"+
			"- Head branch: %s\n"+
			"- Merged at: %s\n\n"+
			"## Title\n\n"+
			"%s\n\n"+
			"## Body\n\n"+
			"%s\n",
		pr.Number,
		pr.URL,
		pr.State,
		pr.BaseRefName,
		pr.HeadRefName,
		pr.MergedAt,
		pr.Title,
		strings.TrimSpace(pr.Body),
	)

	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write PR context file: %w", err)
	}

	return filepath.Base(filename), nil
}
