const fs = require("node:fs");
const path = require("node:path");
const { runJsonCommand } = require("../command");

function readSpec(file) {
  return JSON.parse(fs.readFileSync(file, "utf8"));
}

function asBatch(result, { actor, phase }) {
  if (!result || !Array.isArray(result.items) || result.items.length === 0) {
    return [];
  }
  return [
    {
      items: result.items,
      collected_by: result.collected_by || actor,
      phase,
    },
  ];
}

function validateSpec(spec) {
  if (!spec || typeof spec !== "object") {
    throw new Error("exec spec must be an object");
  }
  if (typeof spec.topic !== "string" || spec.topic.trim().length === 0) {
    throw new Error("exec spec.topic must be a non-empty string");
  }
  if (typeof spec.chair !== "string" || spec.chair.trim().length === 0) {
    throw new Error("exec spec.chair must be a non-empty string");
  }
  if (!Array.isArray(spec.critics)) {
    throw new Error("exec spec.critics must be an array");
  }
  if (!Number.isInteger(spec.max_rounds) || spec.max_rounds <= 0) {
    throw new Error("exec spec.max_rounds must be a positive integer");
  }
  if (!spec.agent || typeof spec.agent !== "object") {
    throw new Error("exec spec.agent must be an object");
  }
}

function createExecAdapter({ specPath }) {
  const resolvedSpecPath = path.resolve(specPath);
  const specDir = path.dirname(resolvedSpecPath);
  const spec = readSpec(resolvedSpecPath);
  validateSpec(spec);

  function resolveAgent(actor) {
    const override = spec.actors?.[actor] || {};
    const merged = { ...spec.agent, ...override };
    if (!Array.isArray(merged.cmd) || merged.cmd.length === 0) {
      throw new Error(`exec agent for ${actor} must define a non-empty cmd array`);
    }
    return {
      cmd: merged.cmd,
      cwd: path.resolve(specDir, merged.cwd || "."),
      env: merged.env || {},
      timeout_ms: Number.isInteger(merged.timeout_ms) ? merged.timeout_ms : 60000,
    };
  }

  async function invoke(actor, payload) {
    const agent = resolveAgent(actor);
    return runJsonCommand({
      cmd: agent.cmd,
      cwd: agent.cwd,
      env: agent.env,
      timeout_ms: agent.timeout_ms,
      input: {
        protocol: "roundtable-kernel.exec.v1",
        actor,
        ...payload,
      },
    });
  }

  return {
    kind: "exec",
    metadata() {
      return {
        topic: spec.topic,
        chair: spec.chair,
        critics: spec.critics,
        max_rounds: spec.max_rounds,
      };
    },
    seedEvidence() {
      if (!spec.seed_batch || !Array.isArray(spec.seed_batch.items) || spec.seed_batch.items.length === 0) {
        return [];
      }
      return [
        {
          items: spec.seed_batch.items,
          collected_by: spec.seed_batch.collected_by || spec.seed_batch.actor || spec.chair,
          phase: "seed",
        },
      ];
    },
    async collectEvidence({ session, round, actor, phase }) {
      const result = await invoke(actor, {
        phase,
        round,
        session,
      });
      return asBatch(result, { actor, phase });
    },
    async propose({ session, round }) {
      const result = await invoke(spec.chair, {
        phase: "propose",
        round,
        session,
      });
      return result.proposal || result;
    },
    async review({ session, round, critic, proposal }) {
      const result = await invoke(critic, {
        phase: "review",
        round,
        session,
        proposal,
      });
      return result.findings || [];
    },
    async adjudicate({ session, round, proposal, findings }) {
      const result = await invoke(spec.chair, {
        phase: "adjudicate",
        round,
        session,
        proposal,
        findings,
      });
      if (!result) {
        return null;
      }
      return result.verdict === undefined ? result : result.verdict;
    },
  };
}

module.exports = {
  createExecAdapter,
};
