package main

import (
	"os"
	"strings"

	"github.com/urfave/cli/v2"
	"pr/cmd"
	"pr/pkg/ui"
)

func main() {
	cli.AppHelpTemplate = strings.Replace(
		cli.AppHelpTemplate,
		`{{template "visibleCommandCategoryTemplate" .}}`,
		`{{range .VisibleCategories}}{{if .Name}}
   {{.Name}}:{{"\t"}}{{range .VisibleCommands}}
     {{join .Names ", "}}{{"\t"}}{{.Usage}}{{end}}{{else}}{{range .VisibleCommands}}
   {{join .Names ", "}}{{"\t"}}{{.Usage}}{{end}}{{end}}{{end}}`,
		1,
	)

	app := &cli.App{
		Name:  "pr",
		Usage: "Personal pull request helper",
		Commands: []*cli.Command{
			cmd.DescribeCommand(),
			cmd.ReviewCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
}
