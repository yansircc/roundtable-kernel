package rtk

func bootstrapExecSession(paths Paths, specPath string, sessionID string, force bool) (*Session, *execAdapter, string, error) {
	bootstrap, err := newExecAdapter(specPath, "")
	if err != nil {
		return nil, nil, "", err
	}
	metadata := bootstrap.Metadata()
	session, err := NewSession(sessionID, metadata.Topic, metadata.Chair, metadata.Critics, metadata.MaxRounds, "exec")
	if err != nil {
		return nil, nil, "", err
	}
	if err := CreateSessionFile(paths, session, force); err != nil {
		return nil, nil, "", err
	}
	telemetryFile, err := ResetTelemetryFile(paths, session.ID)
	if err != nil {
		return nil, nil, "", err
	}
	adapter, err := newExecAdapter(specPath, telemetryFile)
	if err != nil {
		return nil, nil, "", err
	}
	return session, adapter, telemetryFile, nil
}
