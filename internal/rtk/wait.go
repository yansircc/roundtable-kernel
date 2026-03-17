package rtk

import (
	"fmt"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
)

func terminalState(state string) bool {
	switch state {
	case "converged", "failed", "exhausted":
		return true
	default:
		return false
	}
}

func loadSummary(paths Paths, sessionID string) (*Session, SessionSummary, error) {
	session, err := LoadSession(paths, sessionID)
	if err != nil {
		return nil, SessionSummary{}, err
	}
	return session, DeriveSessionSummary(session), nil
}

func WaitForSession(paths Paths, sessionID string, since string, until string, actor string, timeout time.Duration) (*Session, *NextResult, error) {
	check := func() (*Session, *NextResult, bool, error) {
		session, err := LoadSession(paths, sessionID)
		if err != nil {
			return nil, nil, false, err
		}
		if err := advanceInternal(session, paths); err != nil {
			return nil, nil, false, err
		}
		summary := DeriveSessionSummary(session)
		step, reason, terminal, err := previewNextStep(session)
		if err != nil {
			return nil, nil, false, err
		}
		result := &NextResult{
			Ready:    step != nil,
			Terminal: terminal,
			Reason:   reason,
			Summary:  summary,
			Step:     step,
		}
		switch until {
		case "terminal":
			return session, result, terminalState(summary.State), nil
		case "turn":
			if step == nil {
				return session, result, false, nil
			}
			if actor == "" {
				return session, result, true, nil
			}
			return session, result, step.Actor == actor, nil
		default:
			if since == "" {
				return session, result, false, nil
			}
			return session, result, summary.UpdatedAt != since, nil
		}
	}

	session, result, matched, err := check()
	if err == nil && matched {
		return session, result, nil
	}
	if err != nil {
		return nil, nil, err
	}

	if err := ensureDir(paths.SessionsRoot); err != nil {
		return nil, nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, err
	}
	defer watcher.Close()

	watchTargets := []string{paths.SessionsRoot, SessionPath(paths, sessionID)}
	for _, target := range watchTargets {
		if err := watcher.Add(target); err != nil && !os.IsNotExist(err) {
			return nil, nil, err
		}
	}

	// Re-check after watcher registration so writes in the setup window are not lost.
	session, result, matched, err = check()
	if err == nil && matched {
		return session, result, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var timeoutC <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		timeoutC = timer.C
	}

	for {
		select {
		case <-watcher.Events:
			session, result, matched, err = check()
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, nil, err
			}
			if matched {
				return session, result, nil
			}
		case err := <-watcher.Errors:
			if err != nil {
				return nil, nil, err
			}
		case <-timeoutC:
			return nil, nil, fmt.Errorf("wait timed out")
		}
	}
}
