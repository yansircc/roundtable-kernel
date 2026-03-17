package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"roundtable-kernel/internal/rtk"
)

func parseClaudeStructuredOutput(stdout string) (any, error) {
	payload := []map[string]any{}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return nil, err
	}
	for index := len(payload) - 1; index >= 0; index-- {
		if payload[index]["type"] == "result" {
			value, ok := payload[index]["structured_output"]
			if ok {
				return value, nil
			}
		}
	}
	return nil, fmt.Errorf("claude wrapper could not find structured_output in result event")
}

func main() {
	args := rtk.ParseArgs(os.Args[1:])
	if args.Has("help") {
		fmt.Fprint(os.Stdout, "usage: go run ./cmd/claude-agent --workspace /abs/path [--bin claude|ccc] [--model sonnet|opus|haiku] [--settings file] [--permission-mode bypassPermissions] [--tools Read,Grep,Glob]\n")
		return
	}

	request := rtk.AgentRequest{}
	if err := rtk.ReadJSONStdin(&request); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	workspace := args.Value("workspace")
	if workspace == "" {
		workspace = os.Getenv("ROUNDTABLE_WORKSPACE")
	}
	if err := rtk.Ensure(workspace, "claude-agent requires --workspace or ROUNDTABLE_WORKSPACE"); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	bin := args.Value("bin")
	if bin == "" {
		bin = os.Getenv("CLAUDE_BIN")
	}
	if bin == "" {
		bin = "claude"
	}
	model := args.Value("model")
	if model == "" {
		model = os.Getenv("CLAUDE_MODEL")
	}
	settings := args.Value("settings")
	if settings == "" {
		settings = os.Getenv("CLAUDE_SETTINGS")
	}
	permissionMode := args.Value("permission-mode")
	if permissionMode == "" {
		permissionMode = os.Getenv("CLAUDE_PERMISSION_MODE")
	}
	if permissionMode == "" {
		permissionMode = "bypassPermissions"
	}
	tools := args.Value("tools")
	if tools == "" {
		tools = os.Getenv("CLAUDE_TOOLS")
	}
	if tools == "" {
		tools = "Read,Grep,Glob"
	}
	settingSources := os.Getenv("CLAUDE_SETTING_SOURCES")
	if args.Has("setting-sources") {
		settingSources = args.Value("setting-sources")
	}
	mcpConfig := os.Getenv("CLAUDE_MCP_CONFIG")
	if mcpConfig == "" {
		mcpConfig = `{"mcpServers":{}}`
	}
	if args.Has("mcp-config") {
		mcpConfig = args.Value("mcp-config")
	}
	strictMCP := true
	if value := os.Getenv("CLAUDE_STRICT_MCP_CONFIG"); value == "0" {
		strictMCP = false
	}
	if args.Has("strict-mcp-config") {
		strictMCP = true
	}
	timeoutMS := rtk.DefaultTimeoutMS
	if value := os.Getenv("CLAUDE_TIMEOUT_MS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			timeoutMS = parsed
		}
	}
	if value := args.Value("timeout"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			timeoutMS = parsed
		}
	}
	telemetryFile := os.Getenv("ROUNDTABLE_TELEMETRY_FILE")
	schemaJSON, _ := json.Marshal(rtk.OutputSchemaForPhase(request.Phase))
	cmd := []string{bin, "-p", "--output-format", "json", "--json-schema", string(schemaJSON), "--setting-sources", settingSources}
	if strictMCP {
		cmd = append(cmd, "--strict-mcp-config")
	}
	if mcpConfig != "" {
		cmd = append(cmd, "--mcp-config", mcpConfig)
	}
	if model != "" {
		cmd = append(cmd, "--model", model)
	}
	if settings != "" {
		cmd = append(cmd, "--settings", settings)
	}
	if permissionMode != "" {
		cmd = append(cmd, "--permission-mode", permissionMode)
	}
	if tools != "" {
		cmd = append(cmd, "--tools", tools)
	}
	cmd = append(cmd, "--disable-slash-commands", "--no-session-persistence", rtk.PromptForRequest(request))

	stdout, _, err := rtk.RunCommand(rtk.CommandOptions{
		Cmd:     cmd,
		Cwd:     workspace,
		Timeout: time.Duration(timeoutMS) * time.Millisecond,
		Telemetry: &rtk.CommandTelemetry{
			File: telemetryFile,
			Context: map[string]any{
				"session_id":        request.Session.ID,
				"round":             request.Round,
				"actor":             request.Actor,
				"phase":             request.Phase,
				"adapter":           getenvDefault("ROUNDTABLE_ADAPTER_KIND", "exec"),
				"source":            "claude_wrapper",
				"provider":          "claude",
				"cli_bin":           bin,
				"model":             model,
				"settings":          nullString(settings),
				"setting_sources":   settingSources,
				"strict_mcp_config": strictMCP,
				"permission_mode":   permissionMode,
				"tools":             tools,
			},
		},
		OnStdoutChunk: func(chunk rtk.Chunk) {
			_ = rtk.AppendTelemetryEvent(telemetryFile, map[string]any{
				"type":         "wrapper_stream",
				"session_id":   request.Session.ID,
				"round":        request.Round,
				"actor":        request.Actor,
				"phase":        request.Phase,
				"adapter":      getenvDefault("ROUNDTABLE_ADAPTER_KIND", "exec"),
				"source":       "claude_wrapper",
				"provider":     "claude",
				"cli_bin":      bin,
				"model":        model,
				"channel":      chunk.Channel,
				"byte_length":  chunk.ByteLength,
				"elapsed_ms":   chunk.ElapsedMS,
				"text_excerpt": rtk.ClipText(chunk.Text, 400),
			})
		},
		OnStderrChunk: func(chunk rtk.Chunk) {
			_ = rtk.AppendTelemetryEvent(telemetryFile, map[string]any{
				"type":         "wrapper_stream",
				"session_id":   request.Session.ID,
				"round":        request.Round,
				"actor":        request.Actor,
				"phase":        request.Phase,
				"adapter":      getenvDefault("ROUNDTABLE_ADAPTER_KIND", "exec"),
				"source":       "claude_wrapper",
				"provider":     "claude",
				"cli_bin":      bin,
				"model":        model,
				"channel":      chunk.Channel,
				"byte_length":  chunk.ByteLength,
				"elapsed_ms":   chunk.ElapsedMS,
				"text_excerpt": rtk.ClipText(chunk.Text, 400),
			})
		},
	})
	if err != nil {
		_ = rtk.AppendTelemetryEvent(telemetryFile, map[string]any{
			"type":       "wrapper_failed",
			"session_id": request.Session.ID,
			"round":      request.Round,
			"actor":      request.Actor,
			"phase":      request.Phase,
			"adapter":    getenvDefault("ROUNDTABLE_ADAPTER_KIND", "exec"),
			"source":     "claude_wrapper",
			"provider":   "claude",
			"error":      rtk.SanitizeError(err),
		})
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	value, err := parseClaudeStructuredOutput(stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if err := rtk.PrintJSON(value); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func getenvDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
