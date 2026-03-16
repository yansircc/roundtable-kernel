const { createFixtureAdapter } = require("./fixture");
const { createExecAdapter } = require("./exec");

function createAdapter(kind, config) {
  if (kind === "fixture") {
    return createFixtureAdapter(config);
  }
  if (kind === "exec") {
    return createExecAdapter(config);
  }
  throw new Error(`unknown adapter kind: ${kind}`);
}

module.exports = {
  createAdapter,
};
