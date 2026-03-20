//go:build integration

package rtk

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeIntegrationSpec(t *testing.T, root string, critics []string) string {
	t.Helper()

	specPath := filepath.Join(root, "spec.json")
	spec := map[string]any{
		"topic":      "integration test topic",
		"chair":      "chair",
		"critics":    critics,
		"max_rounds": 1,
		"agent": map[string]any{
			"cmd": []string{"true"},
		},
	}
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(specPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return specPath
}

func testPaths(t *testing.T) Paths {
	t.Helper()
	return ResolvePaths(t.TempDir())
}

func writeSlowRunSpec(t *testing.T, root string) string {
	t.Helper()

	specPath := filepath.Join(root, "slow-spec.json")
	spec := map[string]any{
		"topic":      "slow integration topic",
		"chair":      "chair",
		"critics":    []string{},
		"max_rounds": 1,
		"agent": map[string]any{
			"cmd": []string{"sh", "-c", "sleep 10"},
		},
	}
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(specPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return specPath
}

func TestLiveFlowConvergesWithoutCritics(t *testing.T) {
	paths := testPaths(t)
	specPath := writeIntegrationSpec(t, paths.Root, nil)

	session, _, err := InitSession(paths, specPath, "live-converges", true)
	if err != nil {
		t.Fatalf("InitSession: %v", err)
	}

	session, next, err := NextStep(paths, session.ID, "chair")
	if err != nil {
		t.Fatalf("NextStep explore: %v", err)
	}
	if next.Step == nil || next.Step.Phase != "explore" {
		t.Fatalf("unexpected explore step: %#v", next.Step)
	}

	session, err = ApplyStep(paths, session.ID, ApplyInput{
		StartedAt: next.Step.StartedAt,
		Result: map[string]any{
			"items": []map[string]any{
				{
					"source":    "repo/file.go:1",
					"kind":      "reference",
					"statement": "fact",
					"excerpt":   "fact excerpt",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ApplyStep explore: %v", err)
	}

	session, next, err = NextStep(paths, session.ID, "chair")
	if err != nil {
		t.Fatalf("NextStep propose: %v", err)
	}
	if next.Step == nil || next.Step.Phase != "propose" {
		t.Fatalf("unexpected propose step: %#v", next.Step)
	}

	_, err = ApplyStep(paths, session.ID, ApplyInput{
		StartedAt: next.Step.StartedAt,
		Result: map[string]any{
			"proposal": map[string]any{
				"summary":    "proposal",
				"claims":     []string{"claim"},
				"acceptance": []string{"acceptance"},
			},
		},
	})
	if err != nil {
		t.Fatalf("ApplyStep propose: %v", err)
	}

	finalSession, result, err := WaitForSession(paths, session.ID, "", "terminal", "", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForSession terminal: %v", err)
	}
	if !result.Terminal {
		t.Fatalf("expected terminal result: %#v", result)
	}
	if finalSession.Status.State != "converged" || !finalSession.Status.Converged {
		t.Fatalf("unexpected final status: %#v", finalSession.Status)
	}
	if finalSession.AdjudicatedProposal == nil || finalSession.AdjudicatedProposal.Summary != "proposal" {
		t.Fatalf("unexpected adjudicated proposal: %#v", finalSession.AdjudicatedProposal)
	}
}

func TestWaitForSessionReturnsAfterDurableChange(t *testing.T) {
	paths := testPaths(t)
	specPath := writeIntegrationSpec(t, paths.Root, nil)

	session, _, err := InitSession(paths, specPath, "live-wait-change", true)
	if err != nil {
		t.Fatalf("InitSession: %v", err)
	}
	initialSummary := DeriveSessionSummary(session)

	resultCh := make(chan *NextResult, 1)
	errCh := make(chan error, 1)
	go func() {
		_, result, waitErr := WaitForSession(paths, session.ID, initialSummary.UpdatedAt, "change", "", 2*time.Second)
		if waitErr != nil {
			errCh <- waitErr
			return
		}
		resultCh <- result
	}()

	time.Sleep(100 * time.Millisecond)

	_, _, err = NextStep(paths, session.ID, "chair")
	if err != nil {
		t.Fatalf("NextStep: %v", err)
	}

	select {
	case waitErr := <-errCh:
		t.Fatalf("WaitForSession: %v", waitErr)
	case result := <-resultCh:
		if !result.Ready || result.Step == nil {
			t.Fatalf("unexpected wait result: %#v", result)
		}
		if result.Step.Phase != "explore" {
			t.Fatalf("unexpected phase: %#v", result.Step)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for WaitForSession to return")
	}
}

func TestStopSessionTerminatesLiveFlow(t *testing.T) {
	paths := testPaths(t)
	specPath := writeIntegrationSpec(t, paths.Root, nil)

	session, _, err := InitSession(paths, specPath, "live-stop", true)
	if err != nil {
		t.Fatalf("InitSession: %v", err)
	}
	session, next, err := NextStep(paths, session.ID, "chair")
	if err != nil {
		t.Fatalf("NextStep: %v", err)
	}
	if next.Step == nil || next.Step.Phase != "explore" {
		t.Fatalf("unexpected step: %#v", next.Step)
	}

	session, err = StopSession(paths, session.ID)
	if err != nil {
		t.Fatalf("StopSession: %v", err)
	}
	if session.Status.State != "stopped" {
		t.Fatalf("unexpected stopped state: %#v", session.Status)
	}
	if session.OpenRound == nil || len(session.OpenRound.PhaseHistory) == 0 {
		t.Fatalf("missing phase history: %#v", session.OpenRound)
	}
	if status := session.OpenRound.PhaseHistory[len(session.OpenRound.PhaseHistory)-1].Status; status != "stopped" {
		t.Fatalf("unexpected phase status after stop: %s", status)
	}

	finalSession, result, err := WaitForSession(paths, session.ID, "", "terminal", "", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForSession terminal: %v", err)
	}
	if !result.Terminal || result.Reason != "stopped" {
		t.Fatalf("unexpected terminal result: %#v", result)
	}
	if finalSession.Status.State != "stopped" {
		t.Fatalf("unexpected final state: %#v", finalSession.Status)
	}

	_, err = ApplyStep(paths, session.ID, ApplyInput{
		StartedAt: next.Step.StartedAt,
		Result: map[string]any{
			"items": []map[string]any{},
		},
	})
	if !errors.Is(err, ErrSessionStopped) {
		t.Fatalf("expected ApplyStep to reject stopped session, got %v", err)
	}
}

func TestRunSessionReturnsStoppedWhenExternallyStopped(t *testing.T) {
	paths := testPaths(t)
	specPath := writeSlowRunSpec(t, paths.Root)

	type runResult struct {
		session *Session
		err     error
	}
	resultCh := make(chan runResult, 1)
	go func() {
		session, err := RunSession(context.Background(), RunSessionOptions{
			Paths:     paths,
			SpecPath:  specPath,
			SessionID: "run-stop",
			Force:     true,
		})
		resultCh <- runResult{session: session, err: err}
	}()

	deadline := time.Now().Add(3 * time.Second)
	for {
		session, err := LoadSession(paths, "run-stop")
		if err == nil && session.Status.State == "running" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for autonomous run to enter running state")
		}
		time.Sleep(25 * time.Millisecond)
	}

	stoppedSession, err := StopSession(paths, "run-stop")
	if err != nil {
		t.Fatalf("StopSession: %v", err)
	}
	if stoppedSession.Status.State != "stopped" {
		t.Fatalf("unexpected stopped state: %#v", stoppedSession.Status)
	}

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("RunSession: %v", result.err)
		}
		if result.session == nil || result.session.Status.State != "stopped" {
			t.Fatalf("unexpected run result: %#v", result.session)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for autonomous run to stop")
	}

	finalSession, err := LoadSession(paths, "run-stop")
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if finalSession.Status.State != "stopped" {
		t.Fatalf("unexpected final state: %#v", finalSession.Status)
	}
	if finalSession.OpenRound == nil || len(finalSession.OpenRound.PhaseHistory) == 0 {
		t.Fatalf("missing open round after stop: %#v", finalSession.OpenRound)
	}
	if status := finalSession.OpenRound.PhaseHistory[len(finalSession.OpenRound.PhaseHistory)-1].Status; status != "stopped" {
		t.Fatalf("unexpected phase status after external stop: %s", status)
	}
}
