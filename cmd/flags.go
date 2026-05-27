package cmd

import "github.com/urfave/cli/v2"

const (
	defaultThinkingLevel = "high"
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
