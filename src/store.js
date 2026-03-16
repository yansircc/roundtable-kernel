const fs = require("node:fs");
const path = require("node:path");

const projectRoot = path.resolve(__dirname, "..");
const sessionsRoot = path.join(projectRoot, "sessions");

function ensureDir(dir) {
  fs.mkdirSync(dir, { recursive: true });
}

function sessionPath(id) {
  return path.join(sessionsRoot, `${id}.json`);
}

function saveSession(session) {
  ensureDir(sessionsRoot);
  fs.writeFileSync(sessionPath(session.id), JSON.stringify(session, null, 2) + "\n");
}

function createSessionFile(session, { force = false } = {}) {
  const file = sessionPath(session.id);
  if (fs.existsSync(file) && !force) {
    throw new Error(`session already exists: ${session.id}`);
  }
  saveSession(session);
  return file;
}

function loadSession(id) {
  return JSON.parse(fs.readFileSync(sessionPath(id), "utf8"));
}

function listSessions() {
  if (!fs.existsSync(sessionsRoot)) {
    return [];
  }
  return fs
    .readdirSync(sessionsRoot)
    .filter((file) => file.endsWith(".json"))
    .map((file) => path.basename(file, ".json"))
    .sort();
}

module.exports = {
  createSessionFile,
  listSessions,
  loadSession,
  projectRoot,
  saveSession,
  sessionPath,
};
