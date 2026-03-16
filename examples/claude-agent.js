#!/usr/bin/env node
const { runCommand } = require("../src/command");
const { appendTelemetryEvent, clipText, sanitizeError } = require("../src/telemetry");
const {
  ensure,
  outputSchemaForPhase,
  parseArgs,
  printJson,
  promptForRequest,
  readJsonStdin,
} = require("./roundtable-agent-lib");

function parseClaudeStructuredOutput(stdout) {
  const payload = JSON.parse(stdout);
  if (!Array.isArray(payload)) {
    throw new Error("claude wrapper expected JSON event array");
  }
  const result = [...payload].reverse().find((item) => item.type === "result");
  if (!result || result.structured_output === undefined) {
    throw new Error("claude wrapper could not find structured_output in result event");
  }
  return result.structured_output;
}

function hasArg(args, key) {
  return Object.prototype.hasOwnProperty.call(args, key);
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  if (args.help) {
    process.stdout.write(
      "usage: node examples/claude-agent.js --workspace /abs/path [--bin claude|ccc] [--model sonnet|opus|haiku] [--settings file] [--permission-mode bypassPermissions] [--tools Read,Grep,Glob]\n",
    );
    return;
  }
  const request = await readJsonStdin();
  const workspace = args.workspace || process.env.ROUNDTABLE_WORKSPACE;
  ensure(workspace, "claude-agent requires --workspace or ROUNDTABLE_WORKSPACE");

  const bin = args.bin || process.env.CLAUDE_BIN || "claude";
  const model = args.model || process.env.CLAUDE_MODEL;
  const settings = args.settings || process.env.CLAUDE_SETTINGS;
  const permissionMode = args["permission-mode"] || process.env.CLAUDE_PERMISSION_MODE || "bypassPermissions";
  const tools = args.tools || process.env.CLAUDE_TOOLS || "Read,Grep,Glob";
  const settingSources = hasArg(args, "setting-sources")
    ? String(args["setting-sources"])
    : (process.env.CLAUDE_SETTING_SOURCES ?? "");
  const mcpConfig = hasArg(args, "mcp-config")
    ? String(args["mcp-config"])
    : (process.env.CLAUDE_MCP_CONFIG || '{"mcpServers":{}}');
  const strictMcpConfig = hasArg(args, "strict-mcp-config")
    ? Boolean(args["strict-mcp-config"])
    : process.env.CLAUDE_STRICT_MCP_CONFIG !== "0";
  const telemetryFile = process.env.ROUNDTABLE_TELEMETRY_FILE || null;

  const schema = outputSchemaForPhase(request.phase);
  const prompt = promptForRequest(request);
  const streamContext = {
    session_id: request.session?.id,
    round: request.round,
    actor: request.actor,
    phase: request.phase,
    adapter: process.env.ROUNDTABLE_ADAPTER_KIND || "exec",
    source: "claude_wrapper",
    provider: "claude",
    cli_bin: bin,
    model: model || null,
  };

  const cmd = [bin, "-p", "--output-format", "json", "--json-schema", JSON.stringify(schema)];
  cmd.push("--setting-sources", settingSources);
  if (strictMcpConfig) {
    cmd.push("--strict-mcp-config");
  }
  if (mcpConfig) {
    cmd.push("--mcp-config", mcpConfig);
  }
  if (model) {
    cmd.push("--model", model);
  }
  if (settings) {
    cmd.push("--settings", settings);
  }
  if (permissionMode) {
    cmd.push("--permission-mode", permissionMode);
  }
  if (tools) {
    cmd.push("--tools", tools);
  }
  cmd.push("--disable-slash-commands", "--no-session-persistence", prompt);

  try {
    const { stdout } = await runCommand({
      cmd,
      cwd: workspace,
      timeout_ms: Number.parseInt(args.timeout || process.env.CLAUDE_TIMEOUT_MS || "180000", 10),
      telemetry: {
        file: telemetryFile,
        context: {
          ...streamContext,
          settings: settings || null,
          setting_sources: settingSources,
          strict_mcp_config: strictMcpConfig,
          permission_mode: permissionMode,
          tools,
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

    printJson(parseClaudeStructuredOutput(stdout));
  } catch (error) {
    appendTelemetryEvent(telemetryFile, {
      type: "wrapper_failed",
      session_id: request.session?.id,
      round: request.round,
      actor: request.actor,
      phase: request.phase,
      adapter: process.env.ROUNDTABLE_ADAPTER_KIND || "exec",
      source: "claude_wrapper",
      provider: "claude",
      error: sanitizeError(error),
    });
    throw error;
  }
}

main().catch((error) => {
  process.stderr.write(`${error.stack || error.message}\n`);
  process.exit(1);
});
