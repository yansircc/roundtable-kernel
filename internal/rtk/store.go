package rtk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Paths struct {
	Root          string
	SessionsRoot  string
	TelemetryRoot string
	UIRoot        string
}

func ResolvePaths(root string) Paths {
	return Paths{
		Root:          root,
		SessionsRoot:  filepath.Join(root, "sessions"),
		TelemetryRoot: filepath.Join(root, "telemetry"),
		UIRoot:        filepath.Join(root, "ui", "dist"),
	}
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

func SessionPath(paths Paths, id string) string {
	return filepath.Join(paths.SessionsRoot, id+".json")
}

func SaveSession(paths Paths, session *Session) error {
	if err := ensureDir(paths.SessionsRoot); err != nil {
		return err
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(SessionPath(paths, session.ID), data, 0o644)
}

func CreateSessionFile(paths Paths, session *Session, force bool) error {
	file := SessionPath(paths, session.ID)
	if !force {
		if _, err := os.Stat(file); err == nil {
			return fmt.Errorf("session already exists: %s", session.ID)
		}
	}
	return SaveSession(paths, session)
}

func LoadSession(paths Paths, id string) (*Session, error) {
	data, err := os.ReadFile(SessionPath(paths, id))
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func ListSessions(paths Paths) ([]string, error) {
	if _, err := os.Stat(paths.SessionsRoot); err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	entries, err := os.ReadDir(paths.SessionsRoot)
	if err != nil {
		return nil, err
	}
	sessions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		sessions = append(sessions, name[:len(name)-len(filepath.Ext(name))])
	}
	sort.Strings(sessions)
	return sessions, nil
}
