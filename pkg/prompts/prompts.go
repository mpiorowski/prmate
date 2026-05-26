package prompts

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed guidelines/*.md
var guidelinesFS embed.FS

func Load(name string) (string, error) {
	content, err := guidelinesFS.ReadFile("guidelines/" + name + ".md")
	if err != nil {
		return "", fmt.Errorf("could not find guideline file %q: %w", name+".md", err)
	}

	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return "", fmt.Errorf("guideline file %q is empty", name+".md")
	}

	return trimmed, nil
}
