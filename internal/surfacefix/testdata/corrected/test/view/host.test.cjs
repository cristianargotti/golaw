const assert = require("node:assert");
const { render, sum } = require("../../views/render.cjs");

assert.strictEqual(render({ title: "panel" }), "panel");
assert.strictEqual(sum(1, 1), 2);
