package rtk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
)

var ErrSessionStopped = errors.New("session stopped")

func sessionStopped(session *Session) bool {
	return session != nil && session.Status.State == "stopped"
}

func runningPhaseIndex(session *Session) int {
	if session == nil || session.OpenRound == nil {
		return -1
	}
	for index := len(session.OpenRound.PhaseHistory) - 1; index >= 0; index-- {
		if session.OpenRound.PhaseHistory[index].Status == "running" {
			return index
		}
	}
	return -1
}

func MarkSessionStopped(session *Session) {
	if session == nil || sessionStopped(session) {
		return
	}
	at := nowISO()
	if session.Status.Round == 0 && session.OpenRound != nil {
		session.Status.Round = session.OpenRound.Index
	}
	session.Status.State = "stopped"
	session.Status.Converged = false
	session.Status.ActiveActor = nil
	session.Status.ActivePhase = nil
	session.Status.Error = nil
	if session.OpenRound == nil {
		return
	}
	if index := runningPhaseIndex(session); index >= 0 {
		entry := session.OpenRound.PhaseHistory[index]
		completedAt := at
		durationMS := phaseDurationMS(entry.StartedAt)
		session.OpenRound.PhaseHistory[index] = PhaseRecord{
			Actor:         entry.Actor,
			Phase:         entry.Phase,
			Status:        "stopped",
			InputSummary:  entry.InputSummary,
			OutputSummary: entry.OutputSummary,
			Artifact:      entry.Artifact,
			Usage:         entry.Usage,
			StartedAt:     entry.StartedAt,
			CompletedAt:   &completedAt,
			DurationMS:    int64Ptr(durationMS),
			Error:         nil,
		}
	}
	session.OpenRound.Error = nil
	session.OpenRound.UpdatedAt = at
}

func StopSession(paths Paths, sessionID string) (*Session, error) {
	session, err := LoadSession(paths, sessionID)
	if err != nil {
		return nil, err
	}
	if sessionStopped(session) {
		return session, nil
	}
	if terminalState(session.Status.State) {
		return nil, fmt.Errorf("session is already terminal: %s", session.Status.State)
	}
	MarkSessionStopped(session)
	if err := SaveSession(paths, session); err != nil {
		return nil, err
	}
	_ = AppendTelemetryEvent(TelemetryPath(paths, session.ID), map[string]any{
		"type":       "session_stopped",
		"session_id": session.ID,
		"adapter":    session.Adapter,
		"round":      session.Status.Round,
	})
	return session, nil
}

func syncStoppedSession(paths Paths, target *Session) error {
	if target == nil {
		return ErrSessionStopped
	}
	latest, err := LoadSession(paths, target.ID)
	if err != nil {
		if os.IsNotExist(err) {
			MarkSessionStopped(target)
			return nil
		}
		return err
	}
	*target = *latest
	if !sessionStopped(target) {
		MarkSessionStopped(target)
	}
	return nil
}

func ContextWithSessionStop(parent context.Context, paths Paths, sessionID string) (context.Context, func(), error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancelCause(parent)
	if err := ensureDir(paths.SessionsRoot); err != nil {
		cancel(context.Canceled)
		return nil, nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		cancel(context.Canceled)
		return nil, nil, err
	}
	watchTargets := []string{paths.SessionsRoot, SessionPath(paths, sessionID)}
	for _, target := range watchTargets {
		if err := watcher.Add(target); err != nil && !os.IsNotExist(err) {
			watcher.Close()
			cancel(context.Canceled)
			return nil, nil, err
		}
	}
	stopIfNeeded := func() bool {
		session, err := LoadSession(paths, sessionID)
		if err != nil {
			return false
		}
		if sessionStopped(session) {
			cancel(ErrSessionStopped)
			return true
		}
		return false
	}
	if stopIfNeeded() {
		_ = watcher.Close()
		return ctx, func() { cancel(context.Canceled) }, nil
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case <-watcher.Events:
				if stopIfNeeded() {
					return
				}
			case <-watcher.Errors:
			}
		}
	}()
	cleanup := func() {
		cancel(context.Canceled)
		select {
		case <-done:
		case <-time.After(250 * time.Millisecond):
		}
	}
	return ctx, cleanup, nil
}
