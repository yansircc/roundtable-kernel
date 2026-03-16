const { spawn } = require("node:child_process");

function truncate(text, limit = 1200) {
  if (!text) {
    return "";
  }
  return text.length > limit ? `${text.slice(0, limit - 1)}…` : text;
}

function runJsonCommand({ cmd, cwd, env = {}, input, timeout_ms = 60000 }) {
  if (!Array.isArray(cmd) || cmd.length === 0) {
    throw new Error("cmd must be a non-empty array");
  }

  return new Promise((resolve, reject) => {
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
      reject(
        new Error(
          `command timed out after ${timeout_ms}ms: ${cmd.join(" ")}\nstderr:\n${truncate(stderr)}\nstdout:\n${truncate(stdout)}`,
        ),
      );
    }, timeout_ms);

    child.on("error", (error) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      reject(error);
    });

    child.stdout.on("data", (chunk) => {
      stdout += chunk.toString("utf8");
    });

    child.stderr.on("data", (chunk) => {
      stderr += chunk.toString("utf8");
    });

    child.on("close", (code) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);

      if (code !== 0) {
        reject(
          new Error(
            `command failed with exit code ${code}: ${cmd.join(" ")}\nstderr:\n${truncate(stderr)}\nstdout:\n${truncate(stdout)}`,
          ),
        );
        return;
      }

      const text = stdout.trim();
      if (!text) {
        reject(new Error(`command produced no stdout JSON: ${cmd.join(" ")}`));
        return;
      }

      try {
        resolve(JSON.parse(text));
      } catch (error) {
        reject(
          new Error(
            `command produced invalid JSON: ${cmd.join(" ")}\nstdout:\n${truncate(stdout)}\nstderr:\n${truncate(stderr)}`,
          ),
        );
      }
    });

    child.stdin.end(`${JSON.stringify(input)}\n`);
  });
}

module.exports = {
  runJsonCommand,
};
