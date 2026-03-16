const { spawn } = require("node:child_process");
const { appendTelemetryEvent, clipText, sanitizeCommand, sanitizeError } = require("./telemetry");

function commandEvent(telemetry, type, payload = {}) {
  if (!telemetry?.file) {
    return;
  }
  appendTelemetryEvent(telemetry.file, {
    type,
    ...telemetry.context,
    ...payload,
  });
}

function emitChunk(callback, payload) {
  if (typeof callback !== "function") {
    return;
  }
  try {
    callback(payload);
  } catch {}
}

function runCommand({
  cmd,
  cwd,
  env = {},
  input,
  timeout_ms = 60000,
  telemetry = null,
  onStdoutChunk = null,
  onStderrChunk = null,
}) {
  if (!Array.isArray(cmd) || cmd.length === 0) {
    throw new Error("cmd must be a non-empty array");
  }

  return new Promise((resolve, reject) => {
    const startedAt = Date.now();
    commandEvent(telemetry, "command_started", {
      command: sanitizeCommand(cmd, cwd, env),
      timeout_ms,
    });

    const child = spawn(cmd[0], cmd.slice(1), {
      cwd,
      env: { ...process.env, ...env },
      stdio: ["pipe", "pipe", "pipe"],
    });

    let stdout = "";
    let stderr = "";
    let settled = false;

    const timer = setTimeout(() => {
      if (settled) {
        return;
      }
      settled = true;
      child.kill("SIGTERM");
      commandEvent(telemetry, "command_failed", {
        command: sanitizeCommand(cmd, cwd, env),
        duration_ms: Date.now() - startedAt,
        timeout_ms,
        stdout_excerpt: clipText(stdout),
        stderr_excerpt: clipText(stderr),
        error: {
          message: `command timed out after ${timeout_ms}ms`,
        },
      });
      reject(
        new Error(
          `command timed out after ${timeout_ms}ms: ${cmd.join(" ")}\nstderr:\n${clipText(stderr)}\nstdout:\n${clipText(stdout)}`,
        ),
      );
    }, timeout_ms);

    child.on("error", (error) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      commandEvent(telemetry, "command_failed", {
        command: sanitizeCommand(cmd, cwd, env),
        duration_ms: Date.now() - startedAt,
        stdout_excerpt: clipText(stdout),
        stderr_excerpt: clipText(stderr),
        error: sanitizeError(error),
      });
      reject(error);
    });

    child.stdout.on("data", (chunk) => {
      const text = chunk.toString("utf8");
      stdout += text;
      emitChunk(onStdoutChunk, {
        channel: "stdout",
        text,
        byte_length: Buffer.byteLength(text),
        elapsed_ms: Date.now() - startedAt,
      });
    });

    child.stderr.on("data", (chunk) => {
      const text = chunk.toString("utf8");
      stderr += text;
      emitChunk(onStderrChunk, {
        channel: "stderr",
        text,
        byte_length: Buffer.byteLength(text),
        elapsed_ms: Date.now() - startedAt,
      });
    });

    child.on("close", (code) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);

      if (code !== 0) {
        commandEvent(telemetry, "command_failed", {
          command: sanitizeCommand(cmd, cwd, env),
          duration_ms: Date.now() - startedAt,
          exit_code: code,
          stdout_excerpt: clipText(stdout),
          stderr_excerpt: clipText(stderr),
          error: {
            message: `command failed with exit code ${code}`,
          },
        });
        reject(
          new Error(
            `command failed with exit code ${code}: ${cmd.join(" ")}\nstderr:\n${clipText(stderr)}\nstdout:\n${clipText(stdout)}`,
          ),
        );
        return;
      }
      commandEvent(telemetry, "command_finished", {
        command: sanitizeCommand(cmd, cwd, env),
        duration_ms: Date.now() - startedAt,
        exit_code: code,
        stdout_excerpt: clipText(stdout),
        stderr_excerpt: clipText(stderr),
      });
      resolve({ stdout, stderr });
    });

    if (input === undefined) {
      child.stdin.end();
      return;
    }
    if (typeof input === "string") {
      child.stdin.end(input);
      return;
    }
    child.stdin.end(`${JSON.stringify(input)}\n`);
  });
}

async function runJsonCommand(options) {
  const { stdout, stderr } = await runCommand(options);
  const text = stdout.trim();
  if (!text) {
    throw new Error(`command produced no stdout JSON: ${options.cmd.join(" ")}`);
  }

  try {
    return JSON.parse(text);
  } catch (error) {
    throw new Error(
      `command produced invalid JSON: ${options.cmd.join(" ")}\nstdout:\n${clipText(stdout)}\nstderr:\n${clipText(stderr)}`,
    );
  }
}

module.exports = {
  runCommand,
  runJsonCommand,
};
