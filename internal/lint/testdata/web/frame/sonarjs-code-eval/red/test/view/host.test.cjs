const vm = require("node:vm");

const source = "globalThis.ready = true;";
vm.runInThisContext(source);
const body = "return value * 2;";
const doubled = new Function("value", body);

module.exports = { doubled };
