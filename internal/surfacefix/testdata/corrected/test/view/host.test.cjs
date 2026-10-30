const assert = require("node:assert");
const { test } = require("node:test");
const { render, sum } = require("../../views/render.cjs");

test("render", () => {
  assert.strictEqual(render({ title: "panel" }), "panel");
  assert.strictEqual(sum(1, 1), 2);
});
