package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"roundtable-kernel/internal/rtk"
)

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

func usage() {
	fmt.Println(`usage:
  rtk init <session-id> <spec-path> [--force]
  rtk run <session-id> <spec-path> [--force] [--text]
  rtk next <session-id> [--actor name]
  rtk apply <session-id> [result.json|-]
  rtk stop <session-id>
  rtk wait <session-id> [--until change|turn|terminal] [--actor name] [--since updated_at] [--timeout-ms 600000]
  rtk show <session-id> [--text]
  rtk list [--text]
  rtk serve [--port 3133]`)
}

func parsePort(args []string, fallback int) int {
	for index := 0; index < len(args); index++ {
		if args[index] == "--port" && index+1 < len(args) {
			if port, err := strconv.Atoi(args[index+1]); err == nil && port > 0 {
				return port
			}
		}
	}
	return fallback
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fail(err.Error())
	}
	paths := rtk.ResolvePaths(root)
	args := os.Args[1:]
	if maybeHandleMeta(args) {
		return
	}

	switch args[0] {
	case "list":
		parsed := rtk.ParseArgs(args[1:])
		summaries, err := rtk.ListSessions(paths)
		if err != nil {
			failCommand("list", parsed.Has("text"), err)
		}
		if parsed.Has("text") {
			if len(summaries) == 0 {
				fmt.Println("no sessions")
				return
			}
			for _, sessionID := range summaries {
				fmt.Println(sessionID)
			}
			return
		}
		detailed := make([]rtk.SessionSummary, 0, len(summaries))
		for _, sessionID := range summaries {
			session, err := rtk.LoadSession(paths, sessionID)
			if err != nil {
				failCommand("list", false, err)
			}
			detailed = append(detailed, rtk.DeriveSessionSummary(session))
		}
		rtk.SortSessionSummaries(detailed)
		printJSON(map[string]any{"sessions": detailed})
	case "show":
		parsed := rtk.ParseArgs(args[1:])
		if len(parsed.Positionals) < 1 {
			failCommand("show", parsed.Has("text"), fmt.Errorf("show requires a session id"))
		}
		session, err := rtk.LoadSession(paths, parsed.Positionals[0])
		if err != nil {
			failCommand("show", parsed.Has("text"), err)
		}
		if parsed.Has("text") {
			fmt.Println(rtk.RenderSession(session))
			return
		}
		printJSON(sessionEnvelope(paths, session))
	case "init":
		parsed := rtk.ParseArgs(args[1:])
		if len(parsed.Positionals) < 2 {
			failCommand("init", false, fmt.Errorf("init requires session id and spec path"))
		}
		session, _, err := rtk.InitSession(paths, filepath.Clean(parsed.Positionals[1]), parsed.Positionals[0], parsed.Has("force"))
		if err != nil {
			failCommand("init", false, err)
		}
		_, nextResult, err := rtk.PeekNextStep(paths, session.ID, "")
		if err != nil {
			failCommand("init", false, err)
		}
		printJSON(nextEnvelope(paths, session, nextResult))
	case "run":
		parsed := rtk.ParseArgs(args[1:])
		if len(parsed.Positionals) < 2 {
			failCommand("run", parsed.Has("text"), fmt.Errorf("run requires session id and spec path"))
		}
		sessionID := parsed.Positionals[0]
		specPath := filepath.Clean(parsed.Positionals[1])
		session, err := rtk.RunSession(context.Background(), rtk.RunSessionOptions{
			Paths:     paths,
			SpecPath:  specPath,
			SessionID: sessionID,
			Force:     parsed.Has("force"),
		})
		if err != nil {
			failCommand("run", parsed.Has("text"), err)
		}
		if parsed.Has("text") {
			fmt.Println(rtk.RenderSession(session))
			return
		}
		printJSON(sessionEnvelope(paths, session))
	case "next":
		parsed := rtk.ParseArgs(args[1:])
		if len(parsed.Positionals) < 1 {
			failCommand("next", false, fmt.Errorf("next requires a session id"))
		}
		session, result, err := rtk.NextStep(paths, parsed.Positionals[0], parsed.Value("actor"))
		if err != nil {
			failCommand("next", false, err)
		}
		printJSON(nextEnvelope(paths, session, result))
	case "apply":
		parsed := rtk.ParseArgs(args[1:])
		if len(parsed.Positionals) < 1 {
			failCommand("apply", false, fmt.Errorf("apply requires a session id"))
		}
		input := rtk.ApplyInput{}
		resultPath := "-"
		if len(parsed.Positionals) >= 2 {
			resultPath = parsed.Positionals[1]
		}
		if resultPath == "-" {
			if err := rtk.ReadJSONStdin(&input); err != nil {
				failCommand("apply", false, err)
			}
		} else {
			data, err := os.ReadFile(filepath.Clean(resultPath))
			if err != nil {
				failCommand("apply", false, err)
			}
			if err := json.Unmarshal(data, &input); err != nil {
				failCommand("apply", false, err)
			}
		}
		session, err := rtk.ApplyStep(paths, parsed.Positionals[0], input)
		if err != nil {
			failCommand("apply", false, err)
		}
		_, nextResult, err := rtk.PeekNextStep(paths, session.ID, "")
		if err != nil {
			failCommand("apply", false, err)
		}
		printJSON(nextEnvelope(paths, session, nextResult))
	case "wait":
		parsed := rtk.ParseArgs(args[1:])
		if len(parsed.Positionals) < 1 {
			failCommand("wait", false, fmt.Errorf("wait requires a session id"))
		}
		sessionID := parsed.Positionals[0]
		until := parsed.Value("until")
		if until == "" {
			until = "change"
		}
		timeout := rtk.DefaultTimeoutMS
		if value := parsed.Value("timeout-ms"); value != "" {
			parsedTimeout, err := strconv.Atoi(value)
			if err != nil || parsedTimeout < 0 {
				failCommand("wait", false, fmt.Errorf("timeout-ms must be a non-negative integer"))
			}
			timeout = parsedTimeout
		}
		since := parsed.Value("since")
		if since == "" && until == "change" {
			session, err := rtk.LoadSession(paths, sessionID)
			if err == nil {
				since = rtk.DeriveSessionSummary(session).UpdatedAt
			}
		}
		session, result, err := rtk.WaitForSession(paths, sessionID, since, until, parsed.Value("actor"), time.Duration(timeout)*time.Millisecond)
		if err != nil {
			failCommand("wait", false, err)
		}
		printJSON(nextEnvelope(paths, session, result))
	case "stop":
		parsed := rtk.ParseArgs(args[1:])
		if len(parsed.Positionals) < 1 {
			failCommand("stop", false, fmt.Errorf("stop requires a session id"))
		}
		session, err := rtk.StopSession(paths, parsed.Positionals[0])
		if err != nil {
			failCommand("stop", false, err)
		}
		_, nextResult, err := rtk.PeekNextStep(paths, session.ID, "")
		if err != nil {
			failCommand("stop", false, err)
		}
		printJSON(nextEnvelope(paths, session, nextResult))
	case "serve":
		port := parsePort(args[1:], 3133)
		if err := rtk.Serve(paths, port); err != nil {
			fail(err.Error())
		}
	default:
		usage()
		os.Exit(1)
	}
}
