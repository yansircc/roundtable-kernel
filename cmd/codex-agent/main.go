package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"roundtable-kernel/internal/rtk"
)

func main() {
	args := rtk.ParseArgs(os.Args[1:])
	if args.Has("help") {
		fmt.Fprint(os.Stdout, "usage: go run ./cmd/codex-agent --workspace /abs/path [--model gpt-5] [--profile profile] [--sandbox read-only]\n")
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
	if err := rtk.Ensure(workspace, "codex-agent requires --workspace or ROUNDTABLE_WORKSPACE"); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	model := args.Value("model")
	if model == "" {
		model = os.Getenv("CODEX_MODEL")
	}
	profile := args.Value("profile")
	if profile == "" {
		profile = os.Getenv("CODEX_PROFILE")
	}
	sandbox := args.Value("sandbox")
	if sandbox == "" {
		sandbox = os.Getenv("CODEX_SANDBOX")
	}
	if sandbox == "" {
		sandbox = "read-only"
	}
	timeoutMS := rtk.DefaultTimeoutMS
	if value := os.Getenv("CODEX_TIMEOUT_MS"); value != "" {
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
	schemaHandle, err := rtk.WriteTempSchema(rtk.OutputSchemaForPhase(request.Phase))
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer schemaHandle.Cleanup()
	outputDir, err := os.MkdirTemp("", "roundtable-codex-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer os.RemoveAll(outputDir)
	outputFile := filepath.Join(outputDir, "last-message.json")
	cmd := []string{
		"codex",
		"exec",
		"--skip-git-repo-check",
		"--sandbox", sandbox,
		"--output-schema", schemaHandle.File,
		"--output-last-message", outputFile,
		"-",
	}
	insert := 2
	if model != "" {
		cmd = append(cmd[:insert], append([]string{"--model", model}, cmd[insert:]...)...)
		insert += 2
	}
	if profile != "" {
		cmd = append(cmd[:insert], append([]string{"--profile", profile}, cmd[insert:]...)...)
	}
	_, _, err = rtk.RunCommand(rtk.CommandOptions{
		Cmd:     cmd,
		Cwd:     workspace,
		Input:   rtk.PromptForRequest(request),
		Timeout: time.Duration(timeoutMS) * time.Millisecond,
		Telemetry: &rtk.CommandTelemetry{
			File: telemetryFile,
			Context: map[string]any{
				"session_id": request.Session.ID,
				"round":      request.Round,
				"actor":      request.Actor,
				"phase":      request.Phase,
				"adapter":    getenvDefault("ROUNDTABLE_ADAPTER_KIND", "exec"),
				"source":     "codex_wrapper",
				"provider":   "codex",
				"model":      model,
				"profile":    nullString(profile),
				"sandbox":    sandbox,
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
				"source":       "codex_wrapper",
				"provider":     "codex",
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
				"source":       "codex_wrapper",
				"provider":     "codex",
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
			"type":     "wrapper_failed",
			"source":   "codex_wrapper",
			"provider": "codex",
			"error":    rtk.SanitizeError(err),
		})
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	data, err := os.ReadFile(outputFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		fmt.Fprintln(os.Stderr, "codex-agent did not receive output-last-message content")
		os.Exit(1)
	}
	value := map[string]any{}
	if err := json.Unmarshal([]byte(text), &value); err != nil {
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
