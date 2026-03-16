package rtk

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type TelemetryPage struct {
	Events     []map[string]any `json:"events"`
	Offset     int              `json:"offset"`
	NextOffset int              `json:"next_offset"`
	Total      int              `json:"total"`
}

func TelemetryPath(paths Paths, sessionID string) string {
	return filepath.Join(paths.TelemetryRoot, sessionID+".jsonl")
}

func ResetTelemetryFile(paths Paths, sessionID string) (string, error) {
	if err := ensureDir(paths.TelemetryRoot); err != nil {
		return "", err
	}
	file := TelemetryPath(paths, sessionID)
	return file, os.WriteFile(file, []byte{}, 0o644)
}

func ClipText(text string, limit int) string {
	if text == "" {
		return ""
	}
	if limit <= 0 || len(text) <= limit {
		return text
	}
	if limit == 1 {
		return "…"
	}
	return text[:limit-1] + "…"
}

func SanitizeError(err error) map[string]any {
	if err == nil {
		return nil
	}
	return map[string]any{
		"message": err.Error(),
		"stack":   ClipText(err.Error(), 4000),
	}
}

func SanitizeCommand(cmd []string, cwd string, env map[string]string) map[string]any {
	argv := make([]string, 0, len(cmd))
	for _, arg := range cmd {
		argv = append(argv, ClipText(arg, 160))
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return map[string]any{
		"argv":     argv,
		"cwd":      cwd,
		"env_keys": keys,
	}
}

func AppendTelemetryEvent(file string, event map[string]any) error {
	if file == "" {
		return nil
	}
	if err := ensureDir(filepath.Dir(file)); err != nil {
		return err
	}
	record := map[string]any{
		"ts": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
	}
	for key, value := range event {
		record[key] = value
	}
	fh, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer fh.Close()
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = fh.Write(append(data, '\n'))
	return err
}

func LoadTelemetry(paths Paths, sessionID string, since int) (*TelemetryPage, error) {
	file := TelemetryPath(paths, sessionID)
	if _, err := os.Stat(file); err != nil {
		if os.IsNotExist(err) {
			return &TelemetryPage{Events: []map[string]any{}, Offset: 0, NextOffset: 0, Total: 0}, nil
		}
		return nil, err
	}
	fh, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	events := []map[string]any{}
	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		event := map[string]any{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	offset := 0
	if since > 0 && since < len(events) {
		offset = since
	} else if since >= len(events) {
		offset = len(events)
	}
	return &TelemetryPage{
		Events:     append([]map[string]any{}, events[offset:]...),
		Offset:     offset,
		NextOffset: len(events),
		Total:      len(events),
	}, nil
}
