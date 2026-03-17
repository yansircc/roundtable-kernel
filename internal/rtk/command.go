package rtk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type CommandTelemetry struct {
	File    string
	Context map[string]any
}

type Chunk struct {
	Channel    string
	Text       string
	ByteLength int
	ElapsedMS  int64
}

type CommandOptions struct {
	Cmd           []string
	Context       context.Context
	Cwd           string
	Env           map[string]string
	Input         any
	Timeout       time.Duration
	Telemetry     *CommandTelemetry
	OnStdoutChunk func(Chunk)
	OnStderrChunk func(Chunk)
}

func commandEvent(telemetry *CommandTelemetry, eventType string, payload map[string]any) {
	if telemetry == nil || telemetry.File == "" {
		return
	}
	record := map[string]any{"type": eventType}
	for key, value := range telemetry.Context {
		record[key] = value
	}
	for key, value := range payload {
		record[key] = value
	}
	_ = AppendTelemetryEvent(telemetry.File, record)
}

func emitChunk(callback func(Chunk), payload Chunk) {
	if callback == nil {
		return
	}
	defer func() { _ = recover() }()
	callback(payload)
}

func mergeEnv(overrides map[string]string) []string {
	env := os.Environ()
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
}

func writeInput(stdin io.WriteCloser, input any) {
	defer stdin.Close()
	if input == nil {
		return
	}
	switch value := input.(type) {
	case string:
		_, _ = io.WriteString(stdin, value)
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return
		}
		_, _ = stdin.Write(append(data, '\n'))
	}
}

func readPipe(pipe io.Reader, channel string, startedAt time.Time, dst *bytes.Buffer, callback func(Chunk)) {
	buffer := make([]byte, 32*1024)
	for {
		count, err := pipe.Read(buffer)
		if count > 0 {
			text := string(buffer[:count])
			dst.WriteString(text)
			emitChunk(callback, Chunk{
				Channel:    channel,
				Text:       text,
				ByteLength: len(text),
				ElapsedMS:  time.Since(startedAt).Milliseconds(),
			})
		}
		if err != nil {
			return
		}
	}
}

func RunCommand(options CommandOptions) (string, string, error) {
	if len(options.Cmd) == 0 {
		return "", "", fmt.Errorf("cmd must be a non-empty array")
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	startedAt := time.Now()
	commandEvent(options.Telemetry, "command_started", map[string]any{
		"command":    SanitizeCommand(options.Cmd, options.Cwd, options.Env),
		"timeout_ms": timeout.Milliseconds(),
	})

	baseCtx := options.Context
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, options.Cmd[0], options.Cmd[1:]...)
	cmd.Dir = options.Cwd
	cmd.Env = mergeEnv(options.Env)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", "", err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", "", err
	}
	if err := cmd.Start(); err != nil {
		commandEvent(options.Telemetry, "command_failed", map[string]any{
			"command":     SanitizeCommand(options.Cmd, options.Cwd, options.Env),
			"duration_ms": time.Since(startedAt).Milliseconds(),
			"error":       SanitizeError(err),
		})
		return "", "", err
	}

	go writeInput(stdin, options.Input)

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		readPipe(stdoutPipe, "stdout", startedAt, &stdoutBuf, options.OnStdoutChunk)
	}()
	go func() {
		defer wg.Done()
		readPipe(stderrPipe, "stderr", startedAt, &stderrBuf, options.OnStderrChunk)
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()
	durationMS := time.Since(startedAt).Milliseconds()

	if ctx.Err() == context.DeadlineExceeded {
		err := fmt.Errorf("command timed out after %dms: %s\nstderr:\n%s\nstdout:\n%s", timeout.Milliseconds(), stringsJoin(options.Cmd), ClipText(stderr, 1200), ClipText(stdout, 1200))
		commandEvent(options.Telemetry, "command_failed", map[string]any{
			"command":        SanitizeCommand(options.Cmd, options.Cwd, options.Env),
			"duration_ms":    durationMS,
			"timeout_ms":     timeout.Milliseconds(),
			"stdout_excerpt": ClipText(stdout, 1200),
			"stderr_excerpt": ClipText(stderr, 1200),
			"error": map[string]any{
				"message": fmt.Sprintf("command timed out after %dms", timeout.Milliseconds()),
			},
		})
		return stdout, stderr, err
	}
	if waitErr != nil {
		exitCode := 1
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		commandEvent(options.Telemetry, "command_failed", map[string]any{
			"command":        SanitizeCommand(options.Cmd, options.Cwd, options.Env),
			"duration_ms":    durationMS,
			"exit_code":      exitCode,
			"stdout_excerpt": ClipText(stdout, 1200),
			"stderr_excerpt": ClipText(stderr, 1200),
			"error": map[string]any{
				"message": fmt.Sprintf("command failed with exit code %d", exitCode),
			},
		})
		return stdout, stderr, fmt.Errorf("command failed with exit code %d: %s\nstderr:\n%s\nstdout:\n%s", exitCode, stringsJoin(options.Cmd), ClipText(stderr, 1200), ClipText(stdout, 1200))
	}

	commandEvent(options.Telemetry, "command_finished", map[string]any{
		"command":        SanitizeCommand(options.Cmd, options.Cwd, options.Env),
		"duration_ms":    durationMS,
		"exit_code":      0,
		"stdout_excerpt": ClipText(stdout, 1200),
		"stderr_excerpt": ClipText(stderr, 1200),
	})
	return stdout, stderr, nil
}

func RunJSONCommand(options CommandOptions) (map[string]any, error) {
	stdout, stderr, err := RunCommand(options)
	if err != nil {
		return nil, err
	}
	text := bytes.TrimSpace([]byte(stdout))
	if len(text) == 0 {
		return nil, fmt.Errorf("command produced no stdout JSON: %s", stringsJoin(options.Cmd))
	}
	value := map[string]any{}
	if err := json.Unmarshal(text, &value); err != nil {
		return nil, fmt.Errorf("command produced invalid JSON: %s\nstdout:\n%s\nstderr:\n%s", stringsJoin(options.Cmd), ClipText(stdout, 1200), ClipText(stderr, 1200))
	}
	return value, nil
}

func stringsJoin(values []string) string {
	return strings.Join(values, " ")
}
