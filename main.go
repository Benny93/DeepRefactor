package main

import (
	"deeprefactor/cmd"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

func main() {
	var cli cmd.CLI
	ctx := kong.Parse(&cli,
		kong.Name("golint-fixer"),
		kong.Description("AI-powered Go lint fixer"),
		kong.UsageOnError(),
	)

	if err := ctx.Run(&cli); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := cli.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
