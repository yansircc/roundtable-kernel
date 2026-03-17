package rtk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewExecAdapterDefaultsToUnboundedRoundsAndDefaultTimeout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	specPath := filepath.Join(root, "spec.json")
	spec := map[string]any{
		"topic": "topic",
		"chair": "chair",
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

	adapter, err := newExecAdapter(specPath, "")
	if err != nil {
		t.Fatalf("newExecAdapter: %v", err)
	}

	if adapter.Metadata().MaxRounds != nil {
		t.Fatalf("expected unbounded rounds, got %#v", adapter.Metadata().MaxRounds)
	}
	if timeout := adapter.resolveAgent("chair").TimeoutMS; timeout != DefaultTimeoutMS {
		t.Fatalf("unexpected timeout: %d", timeout)
	}
}

func TestPreviewNextStepDoesNotExhaustWithoutRoundLimit(t *testing.T) {
	t.Parallel()

	session, err := NewSession("test-unbounded", "topic", "chair", nil, nil, "exec")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	session.Status.Round = 3

	step, reason, terminal, err := previewNextStep(session)
	if err != nil {
		t.Fatalf("previewNextStep: %v", err)
	}
	if terminal {
		t.Fatalf("expected non-terminal result, got reason %s", reason)
	}
	if step == nil || step.Round != 4 || step.Phase != "explore" {
		t.Fatalf("unexpected next step: %#v", step)
	}
}

func TestPreviewNextStepExhaustsAtBoundedRoundLimit(t *testing.T) {
	t.Parallel()

	session, err := NewSession("test-bounded", "topic", "chair", nil, intPtr(3), "exec")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	session.Status.Round = 3

	step, reason, terminal, err := previewNextStep(session)
	if err != nil {
		t.Fatalf("previewNextStep: %v", err)
	}
	if !terminal || reason != "exhausted" || step != nil {
		t.Fatalf("unexpected exhaustion result: step=%#v reason=%s terminal=%v", step, reason, terminal)
	}
}
