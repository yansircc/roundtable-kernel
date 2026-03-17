package main

import (
	"fmt"
	"os"
	"strings"
)

type commandDoc struct {
	summary string
	usage   []string
	details []string
}

var commandDocs = map[string]commandDoc{
	"init": {
		summary: "Create a session and preview the first step without claiming it.",
		usage: []string{
			"rtk init <session-id> <spec-path> [--force]",
		},
		details: []string{
			"`init` creates durable session state and returns the next ready step.",
			"Use `--force` to overwrite an existing session file with the same id.",
		},
	},
	"run": {
		summary: "Run a session autonomously from a spec.",
		usage: []string{
			"rtk run <session-id> <spec-path> [--force] [--text]",
		},
		details: []string{
			"Default output is JSON.",
			"Use `--text` for a human-oriented summary instead of the JSON envelope.",
		},
	},
	"next": {
		summary: "Claim the next ready step for an actor.",
		usage: []string{
			"rtk next <session-id> [--actor name]",
		},
		details: []string{
			"`next` is the only command that moves a ready phase into `running`.",
			"The returned `started_at` token should be echoed back in the later `apply` call.",
		},
	},
	"apply": {
		summary: "Complete the currently running step with a result payload.",
		usage: []string{
			"rtk apply <session-id> [result.json|-]",
		},
		details: []string{
			"Read the result from a file or from stdin when the path is `-` or omitted.",
			"Include the `started_at` token from the corresponding `next` response to guard against stale writes.",
		},
	},
	"wait": {
		summary: "Block until the session changes, a specific actor turn is ready, or the session becomes terminal.",
		usage: []string{
			"rtk wait <session-id> [--until change|turn|terminal] [--actor name] [--since updated_at] [--timeout-ms 600000]",
		},
		details: []string{
			"Use `--until turn --actor critic` to hand work to a critic window.",
			"Use `--until terminal` to wait for convergence, failure, or exhaustion.",
		},
	},
	"show": {
		summary: "Show the durable session state.",
		usage: []string{
			"rtk show <session-id> [--text]",
		},
		details: []string{
			"Default output is the full JSON session envelope.",
			"Use `--text` for a compact rendered view.",
		},
	},
	"list": {
		summary: "List known sessions.",
		usage: []string{
			"rtk list [--text]",
		},
		details: []string{
			"Default output is JSON with derived summaries.",
			"Use `--text` to print session ids only.",
		},
	},
	"serve": {
		summary: "Serve the local web UI.",
		usage: []string{
			"rtk serve [--port 3133]",
		},
		details: []string{
			"The binary serves API endpoints plus the static UI bundle under `ui/dist`.",
			"Release archives include the built UI; source checkouts should run `npm --prefix ui run build` first.",
		},
	},
	"version": {
		summary: "Print build metadata for the current binary.",
		usage: []string{
			"rtk version",
			"rtk --version",
			"rtk -v",
		},
		details: []string{
			"Version metadata is filled by release builds and defaults to `dev` locally.",
		},
	},
}

var commandOrder = []string{"init", "run", "next", "apply", "wait", "show", "list", "serve", "version"}

func helpFlag(arg string) bool {
	return arg == "-h" || arg == "--help"
}

func globalHelp() string {
	lines := []string{
		"rtk is the Roundtable Kernel CLI.",
		"",
		"Usage:",
		"  rtk <command> [options]",
		"",
		"Commands:",
	}
	for _, name := range commandOrder {
		doc := commandDocs[name]
		lines = append(lines, fmt.Sprintf("  %-8s %s", name, doc.summary))
	}
	lines = append(lines,
		"",
		"Examples:",
		"  rtk init my-session ./spec.json --force",
		"  rtk next my-session --actor chair",
		"  rtk apply my-session result.json",
		"  rtk wait my-session --until turn --actor critic",
		"  rtk serve --port 3133",
		"",
		"Use `rtk help <command>` or `rtk <command> -h` for command-specific help.",
	)
	return strings.Join(lines, "\n")
}

func commandHelp(name string) (string, bool) {
	doc, ok := commandDocs[name]
	if !ok {
		return "", false
	}
	lines := []string{
		fmt.Sprintf("rtk %s", name),
		"",
		doc.summary,
		"",
		"Usage:",
	}
	for _, usage := range doc.usage {
		lines = append(lines, "  "+usage)
	}
	if len(doc.details) > 0 {
		lines = append(lines, "", "Notes:")
		for _, detail := range doc.details {
			lines = append(lines, "  - "+detail)
		}
	}
	return strings.Join(lines, "\n"), true
}

func printHelpAndExit(name string, code int) {
	if name == "" {
		fmt.Println(globalHelp())
		os.Exit(code)
	}
	text, ok := commandHelp(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", name)
		fmt.Println(globalHelp())
		os.Exit(1)
	}
	fmt.Println(text)
	os.Exit(code)
}

func maybeHandleMeta(args []string) bool {
	if len(args) == 0 {
		fmt.Println(globalHelp())
		return true
	}
	switch args[0] {
	case "help":
		if len(args) > 1 {
			printHelpAndExit(args[1], 0)
		}
		printHelpAndExit("", 0)
	case "-h", "--help":
		printHelpAndExit("", 0)
	case "version", "-v", "--version":
		printVersion()
		return true
	}
	if len(args) > 1 && helpFlag(args[1]) {
		printHelpAndExit(args[0], 0)
	}
	return false
}
