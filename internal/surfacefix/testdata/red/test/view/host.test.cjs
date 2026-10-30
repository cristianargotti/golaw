const assert = require("node:assert");
const vm = require("node:vm");

const render = new Function("view", "return view.title;");
const sum = "1 + 1";
assert.strictEqual(render({ title: "panel" }), "panel");
assert.strictEqual(vm.runInThisContext(sum), 2);
