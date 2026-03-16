const fs = require("node:fs");
const path = require("node:path");

const projectRoot = path.resolve(__dirname, "..");
const telemetryRoot = path.join(projectRoot, "telemetry");

function ensureDir(dir) {
  fs.mkdirSync(dir, { recursive: true });
}

function telemetryPath(sessionId) {
  return path.join(telemetryRoot, `${sessionId}.jsonl`);
}

function resetTelemetryFile(sessionId) {
  ensureDir(telemetryRoot);
  const file = telemetryPath(sessionId);
  fs.writeFileSync(file, "");
  return file;
}

function clipText(text, limit = 1200) {
  if (!text) {
    return "";
  }
  return text.length > limit ? `${text.slice(0, limit - 1)}…` : text;
}

function sanitizeError(error) {
  if (!error) {
    return null;
  }
  return {
    message: error.message || String(error),
    stack: clipText(error.stack || error.message || String(error), 4000),
  };
}

function sanitizeCommand(cmd, cwd, env) {
  return {
    argv: Array.isArray(cmd) ? cmd.map((arg) => clipText(String(arg), 160)) : [],
    cwd,
    env_keys: Object.keys(env || {}).sort(),
  };
}

function appendTelemetryEvent(file, event) {
  if (!file) {
    return;
  }
  ensureDir(path.dirname(file));
  const record = {
    ts: new Date().toISOString(),
    ...event,
  };
  fs.appendFileSync(file, `${JSON.stringify(record)}\n`);
}

function loadTelemetry(sessionId, { since = 0 } = {}) {
  const file = telemetryPath(sessionId);
  if (!fs.existsSync(file)) {
    return { events: [], offset: 0, next_offset: 0, total: 0 };
  }
  const events = fs
    .readFileSync(file, "utf8")
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => JSON.parse(line));
  const offset = Number.isInteger(since) && since > 0 ? since : 0;
  return {
    events: events.slice(offset),
    offset,
    next_offset: events.length,
    total: events.length,
  };
}

module.exports = {
  appendTelemetryEvent,
  clipText,
  loadTelemetry,
  resetTelemetryFile,
  sanitizeCommand,
  sanitizeError,
  telemetryPath,
  telemetryRoot,
};
