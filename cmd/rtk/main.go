package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"roundtable-kernel/internal/rtk"
)

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

func usage() {
	fmt.Println(`usage:
  go run ./cmd/rtk run <session-id> <spec-path> [--force]
  go run ./cmd/rtk show <session-id> [--json]
  go run ./cmd/rtk list
  go run ./cmd/rtk serve [--port 3133]`)
}

func extractFlag(args []string, flag string) ([]string, bool) {
	next := []string{}
	enabled := false
	for _, arg := range args {
		if arg == flag {
			enabled = true
			continue
		}
		next = append(next, arg)
	}
	return next, enabled
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
	if len(args) == 0 {
		usage()
		return
	}

	switch args[0] {
	case "list":
		sessions, err := rtk.ListSessions(paths)
		if err != nil {
			fail(err.Error())
		}
		if len(sessions) == 0 {
			fmt.Println("no sessions")
			return
		}
		for _, session := range sessions {
			fmt.Println(session)
		}
	case "show":
		rest, asJSON := extractFlag(args[1:], "--json")
		if len(rest) < 1 {
			fail("show requires a session id")
		}
		session, err := rtk.LoadSession(paths, rest[0])
		if err != nil {
			fail(err.Error())
		}
		if asJSON {
			data, _ := json.MarshalIndent(session, "", "  ")
			fmt.Println(string(data))
			return
		}
		fmt.Println(rtk.RenderSession(session))
	case "run":
		rest, force := extractFlag(args[1:], "--force")
		if len(rest) < 2 {
			fail("run requires session id and spec path")
		}
		sessionID := rest[0]
		specPath := rest[1]
		session, err := rtk.RunSession(context.Background(), rtk.RunSessionOptions{
			Paths:     paths,
			SpecPath:  specPath,
			SessionID: sessionID,
			Force:     force,
		})
		if err != nil {
			fail(err.Error())
		}
		fmt.Printf("wrote %s\n", rtk.SessionPath(paths, session.ID))
		fmt.Println(rtk.RenderSession(session))
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
