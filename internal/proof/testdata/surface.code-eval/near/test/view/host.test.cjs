const assert = require("node:assert");
const { render, engine } = require("../../views/render.cjs");

// the test never runs text as code: eval("1") stays in this comment
assert.strictEqual(render({ title: "panel" }), "panel");
assert.strictEqual(engine.eval("1 + 1"), "eval(1 + 1)");
