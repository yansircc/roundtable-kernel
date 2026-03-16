#!/usr/bin/env node
const http = require("node:http");
const fs = require("node:fs");
const path = require("node:path");
const { loadSession, listSessions, projectRoot } = require("./store");
const { loadTelemetry } = require("./telemetry");
const { deriveSessionSummary, sortSessionSummaries } = require("./view");

const staticRoot = path.join(projectRoot, "ui", "dist");
const defaultPort = Number.parseInt(process.env.PORT || "3133", 10);

const mimeTypes = {
  ".css": "text/css; charset=utf-8",
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".svg": "image/svg+xml",
};

function sendJson(res, statusCode, payload) {
  const body = JSON.stringify(payload, null, 2) + "\n";
  res.writeHead(statusCode, {
    "Content-Type": "application/json; charset=utf-8",
    "Content-Length": Buffer.byteLength(body),
    "Cache-Control": "no-store",
  });
  res.end(body);
}

function sendText(res, statusCode, body) {
  res.writeHead(statusCode, {
    "Content-Type": "text/plain; charset=utf-8",
    "Content-Length": Buffer.byteLength(body),
    "Cache-Control": "no-store",
  });
  res.end(body);
}

function sessionSummaries() {
  return sortSessionSummaries(
    listSessions().map((id) => deriveSessionSummary(loadSession(id))),
  );
}

function decodeSessionId(urlPath) {
  return decodeURIComponent(urlPath.replace(/^\/api\/session\//, ""));
}

function safeStaticPath(urlPath) {
  const normalized = path.normalize(urlPath.replace(/^\/+/, ""));
  const target = path.join(staticRoot, normalized);
  if (!target.startsWith(staticRoot)) {
    return null;
  }
  return target;
}

function serveStatic(req, res, urlPath) {
  if (!fs.existsSync(staticRoot)) {
    sendText(res, 503, "UI assets missing. Run `npm --prefix ui install && npm --prefix ui run build`.\n");
    return;
  }

  const target =
    urlPath === "/" || urlPath === ""
      ? path.join(staticRoot, "index.html")
      : safeStaticPath(urlPath);

  if (!target) {
    sendText(res, 404, "not found\n");
    return;
  }

  let file = target;
  if (fs.existsSync(file) && fs.statSync(file).isDirectory()) {
    file = path.join(file, "index.html");
  }
  if (!fs.existsSync(file)) {
    file = path.join(staticRoot, "index.html");
  }

  const ext = path.extname(file).toLowerCase();
  const body = fs.readFileSync(file);
  res.writeHead(200, {
    "Content-Type": mimeTypes[ext] || "application/octet-stream",
    "Content-Length": body.byteLength,
    "Cache-Control": ext === ".html" ? "no-store" : "public, max-age=300",
  });
  res.end(body);
}

function createHandler() {
  return (req, res) => {
    const url = new URL(req.url || "/", "http://127.0.0.1");
    const urlPath = url.pathname;

    try {
      if (urlPath === "/api/healthz") {
        sendJson(res, 200, { ok: true, project_root: projectRoot });
        return;
      }

      if (urlPath === "/api/sessions") {
        sendJson(res, 200, {
          project_root: projectRoot,
          generated_at: new Date().toISOString(),
          sessions: sessionSummaries(),
        });
        return;
      }

      if (urlPath.startsWith("/api/session/")) {
        const sessionId = decodeSessionId(urlPath);
        sendJson(res, 200, loadSession(sessionId));
        return;
      }

      if (urlPath.startsWith("/api/telemetry/")) {
        const sessionId = decodeURIComponent(urlPath.replace(/^\/api\/telemetry\//, ""));
        const since = Number.parseInt(url.searchParams.get("since") || "0", 10);
        sendJson(res, 200, {
          session_id: sessionId,
          ...loadTelemetry(sessionId, { since: Number.isInteger(since) && since > 0 ? since : 0 }),
        });
        return;
      }

      serveStatic(req, res, urlPath);
    } catch (error) {
      if (error && error.code === "ENOENT") {
        sendJson(res, 404, { error: "not_found", detail: error.message });
        return;
      }
      sendJson(res, 500, { error: "internal_error", detail: error.message });
    }
  };
}

function parsePort(argv) {
  const index = argv.indexOf("--port");
  if (index >= 0 && argv[index + 1]) {
    const port = Number.parseInt(argv[index + 1], 10);
    if (Number.isInteger(port) && port > 0) {
      return port;
    }
  }
  return defaultPort;
}

function main(argv) {
  const port = parsePort(argv);
  const server = http.createServer(createHandler());
  server.listen(port, "127.0.0.1", () => {
    console.log(`roundtable-kernel ui listening on http://127.0.0.1:${port}`);
  });
}

main(process.argv.slice(2));
