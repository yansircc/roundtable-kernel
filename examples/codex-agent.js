#!/usr/bin/env node
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { runCommand } = require("../src/command");
const { appendTelemetryEvent, clipText, sanitizeError } = require("../src/telemetry");
const {
  ensure,
  outputSchemaForPhase,
  parseArgs,
  printJson,
  promptForRequest,
  readJsonStdin,
  writeTempSchema,
} = require("./roundtable-agent-lib");

async function main() {
  const args = parseArgs(process.argv.slice(2));
  if (args.help) {
    process.stdout.write(
      "usage: node examples/codex-agent.js --workspace /abs/path [--model gpt-5] [--profile profile] [--sandbox read-only]\n",
    );
    return;
  }
  const request = await readJsonStdin();
  const workspace = args.workspace || process.env.ROUNDTABLE_WORKSPACE;
  ensure(workspace, "codex-agent requires --workspace or ROUNDTABLE_WORKSPACE");

  const model = args.model || process.env.CODEX_MODEL;
  const profile = args.profile || process.env.CODEX_PROFILE;
  const sandbox = args.sandbox || process.env.CODEX_SANDBOX || "read-only";
  const telemetryFile = process.env.ROUNDTABLE_TELEMETRY_FILE || null;

  const schemaHandle = writeTempSchema(outputSchemaForPhase(request.phase));
  const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), "roundtable-codex-"));
  const outputFile = path.join(outputDir, "last-message.json");
  const prompt = promptForRequest(request);
  const streamContext = {
    session_id: request.session?.id,
    round: request.round,
    actor: request.actor,
    phase: request.phase,
    adapter: process.env.ROUNDTABLE_ADAPTER_KIND || "exec",
    source: "codex_wrapper",
    provider: "codex",
    model: model || null,
  };

  try {
    const cmd = [
      "codex",
      "exec",
      "--skip-git-repo-check",
      "--sandbox",
      sandbox,
      "--output-schema",
      schemaHandle.file,
      "--output-last-message",
      outputFile,
      "-",
    ];

    if (model) {
      cmd.splice(2, 0, "--model", model);
    }
    if (profile) {
      cmd.splice(2, 0, "--profile", profile);
    }

    await runCommand({
      cmd,
      cwd: workspace,
      input: prompt,
      timeout_ms: Number.parseInt(args.timeout || process.env.CODEX_TIMEOUT_MS || "180000", 10),
      telemetry: {
        file: telemetryFile,
        context: {
          ...streamContext,
          profile: profile || null,
          sandbox,
        },
      },
      onStdoutChunk: (chunk) => {
        appendTelemetryEvent(telemetryFile, {
          type: "wrapper_stream",
          ...streamContext,
          channel: chunk.channel,
          byte_length: chunk.byte_length,
          elapsed_ms: chunk.elapsed_ms,
          text_excerpt: clipText(chunk.text, 400),
        });
      },
      onStderrChunk: (chunk) => {
        appendTelemetryEvent(telemetryFile, {
          type: "wrapper_stream",
          ...streamContext,
          channel: chunk.channel,
          byte_length: chunk.byte_length,
          elapsed_ms: chunk.elapsed_ms,
          text_excerpt: clipText(chunk.text, 400),
        });
      },
    });

    const text = fs.readFileSync(outputFile, "utf8").trim();
    if (!text) {
      throw new Error("codex-agent did not receive output-last-message content");
    }
    printJson(JSON.parse(text));
  } finally {
    schemaHandle.cleanup();
    try {
      fs.unlinkSync(outputFile);
    } catch {}
    try {
      fs.rmdirSync(outputDir);
    } catch {}
  }
}

main().catch((error) => {
  appendTelemetryEvent(process.env.ROUNDTABLE_TELEMETRY_FILE || null, {
    type: "wrapper_failed",
    source: "codex_wrapper",
    provider: "codex",
    error: sanitizeError(error),
  });
  process.stderr.write(`${error.stack || error.message}\n`);
  process.exit(1);
});
