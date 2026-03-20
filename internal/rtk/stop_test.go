package rtk

import "testing"

func TestPreviewNextStepTreatsStoppedAsTerminal(t *testing.T) {
	t.Parallel()

	session, err := NewSession("stopped-session", "topic", "chair", nil, nil, "exec")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	session.Status.State = "stopped"

	step, reason, terminal, err := previewNextStep(session)
	if err != nil {
		t.Fatalf("previewNextStep: %v", err)
	}
	if !terminal || reason != "stopped" || step != nil {
		t.Fatalf("unexpected stopped preview result: step=%#v reason=%s terminal=%v", step, reason, terminal)
	}
}
