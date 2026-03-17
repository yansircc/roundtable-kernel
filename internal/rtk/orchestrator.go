package rtk

import "context"

type RunSessionOptions struct {
	Paths     Paths
	SpecPath  string
	SessionID string
	Force     bool
}

func failSession(paths Paths, session *Session, telemetryFile string, runErr error) (*Session, error) {
	if session.Status.State != "failed" {
		round := session.Status.Round
		if session.OpenRound != nil {
			round = session.OpenRound.Index
		}
		actor := ""
		if session.Status.ActiveActor != nil {
			actor = *session.Status.ActiveActor
		}
		phase := ""
		if session.Status.ActivePhase != nil {
			phase = *session.Status.ActivePhase
		}
		MarkSessionFailed(session, round, actor, phase, runErr)
		_ = SaveSession(paths, session)
	}
	_ = AppendTelemetryEvent(telemetryFile, map[string]any{
		"type":       "session_failed",
		"session_id": session.ID,
		"adapter":    session.Adapter,
		"round":      session.Status.Round,
		"error":      SanitizeError(runErr),
	})
	return nil, runErr
}

func RunSession(ctx context.Context, options RunSessionOptions) (*Session, error) {
	session, adapter, telemetryFile, err := bootstrapExecSession(options.Paths, options.SpecPath, options.SessionID, options.Force)
	if err != nil {
		return nil, err
	}
	_ = AppendTelemetryEvent(telemetryFile, map[string]any{
		"type":       "session_started",
		"session_id": session.ID,
		"adapter":    session.Adapter,
		"topic":      session.Topic,
		"chair":      session.Chair,
		"critics":    session.Critics,
		"max_rounds": session.MaxRounds,
	})

	evidenceKeyMap := map[string]string{}
	if err := runSeedPhase(ctx, adapter, session, evidenceKeyMap, telemetryFile, options.Paths); err != nil {
		return failSession(options.Paths, session, telemetryFile, err)
	}

	for round := 1; !roundLimitExceeded(session.MaxRounds, round); round++ {
		if err := runRound(ctx, adapter, session, round, evidenceKeyMap, telemetryFile, options.Paths); err != nil {
			return failSession(options.Paths, session, telemetryFile, err)
		}
		if session.Status.Converged {
			break
		}
	}

	if !session.Status.Converged && roundLimitExhausted(session.MaxRounds, session.Status.Round) {
		session.Status.State = "exhausted"
	}
	if err := SaveSession(options.Paths, session); err != nil {
		return failSession(options.Paths, session, telemetryFile, err)
	}
	_ = AppendTelemetryEvent(telemetryFile, map[string]any{
		"type":              "session_finished",
		"session_id":        session.ID,
		"adapter":           session.Adapter,
		"state":             session.Status.State,
		"round":             session.Status.Round,
		"converged":         session.Status.Converged,
		"unresolved_high":   session.Status.UnresolvedHigh,
		"unresolved_medium": session.Status.UnresolvedMedium,
	})
	return session, nil
}
