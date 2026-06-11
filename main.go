package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const defaultHeading = "AI Instructions"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: ayumi <add|inject|version> [commit-message-file]")
		return 2
	}

	if args[0] == "version" {
		if len(args) > 1 {
			printVersionUsage(stderr)
			fmt.Fprintf(stderr, "\ngot extra arguments: %s\n\n", strings.Join(args[1:], " "))
			return 2
		}
		fmt.Fprintln(stdout, currentVersion())
		return 0
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(stderr, "ayumi: %v\n", err)
		return 1
	}

	switch args[0] {
	case "add":
		if err := addPrompt(stdin, cfg); err != nil {
			fmt.Fprintf(stderr, "ayumi add: %v\n", err)
			return 1
		}
		return 0
	case "inject":
		if len(args) < 2 {
			printInjectUsage(stderr)
			return 2
		}
		if len(args) > 2 {
			printInjectUsage(stderr)
			fmt.Fprintf(stderr, "\ngot extra arguments: %s\n\n", strings.Join(args[2:], " "))
			return 2
		}
		if err := injectInstructions(args[1], cfg); err != nil {
			fmt.Fprintf(stderr, "ayumi inject: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 2
	}
}

func printVersionUsage(stderr io.Writer) {
	fmt.Fprintln(stderr, "usage: ayumi version")
}

func printInjectUsage(stderr io.Writer) {
	fmt.Fprintln(stderr, "usage: ayumi inject <commit-message-file>")
}
