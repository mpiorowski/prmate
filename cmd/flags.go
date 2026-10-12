package cmd

import "github.com/urfave/cli/v2"

// Model and reasoning defaults live here and nowhere else. Every provider
// call names its model, so a run reviews with the same models on every
// machine regardless of what the provider CLI or account calls "default".
// Bump these when a new model ships.
const (
	defaultThinkingLevel = "high"
	defaultCodexModel    = "gpt-6-astra"
	defaultClaudeModel   = "claude-opus-5-5"
	defaultDescribeLLM   = "claude:" + defaultClaudeModel
	defaultReviewLLMs    = "codex:" + defaultCodexModel + ",claude:" + defaultClaudeModel
	thinkingFlagUsage    = "Optional. Thinking/reasoning level. Provider support: codex none|minimal|low|medium|high|xhigh; claude low|medium|high|xhigh|max; opencode provider variant. Defaults to high for supported providers; Gemini only errors if this flag is explicitly set."
)

func ThinkingFlag() cli.Flag {
	return &cli.StringFlag{
		Name:    "think",
		Aliases: []string{"thinking"},
		Usage:   thinkingFlagUsage,
		Value:   defaultThinkingLevel,
	}
}

func ThinkingFromContext(c *cli.Context) (string, bool) {
	for _, ctx := range c.Lineage() {
		if ctx.IsSet("think") || ctx.IsSet("thinking") {
			return ctx.String("think"), true
		}
	}

	return defaultThinkingLevel, false
}
