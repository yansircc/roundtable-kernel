package main

import (
	"encoding/json"
	"fmt"
	"os"

	"roundtable-kernel/internal/rtk"
)

func printJSON(value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Println(string(data))
}

func failCommand(command string, text bool, err error) {
	if err == nil {
		os.Exit(1)
	}
	if text {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	data, _ := json.MarshalIndent(map[string]any{
		"error":   command + "_failed",
		"detail":  err.Error(),
		"command": command,
	}, "", "  ")
	fmt.Fprintln(os.Stderr, string(data))
	os.Exit(1)
}

func sessionEnvelope(paths rtk.Paths, session *rtk.Session) map[string]any {
	return map[string]any{
		"session":        session,
		"summary":        rtk.DeriveSessionSummary(session),
		"session_path":   rtk.SessionPath(paths, session.ID),
		"telemetry_path": rtk.TelemetryPath(paths, session.ID),
	}
}

func nextEnvelope(paths rtk.Paths, session *rtk.Session, result *rtk.NextResult) map[string]any {
	envelope := map[string]any{
		"ready":          result.Ready,
		"terminal":       result.Terminal,
		"reason":         result.Reason,
		"summary":        result.Summary,
		"session_path":   rtk.SessionPath(paths, session.ID),
		"telemetry_path": rtk.TelemetryPath(paths, session.ID),
	}
	if session != nil {
		envelope["session"] = session
	}
	if result.Step != nil {
		envelope["step"] = result.Step
	}
	return envelope
}
