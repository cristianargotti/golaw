const assert = require("node:assert");
const vm = require("node:vm");

const render = new Function("view", "return view.title;");
assert.strictEqual(render({ title: "panel" }), "panel");
assert.strictEqual(vm.runInThisContext("1 + 1"), 2);
