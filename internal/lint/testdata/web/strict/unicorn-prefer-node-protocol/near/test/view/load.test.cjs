const fs = require("node:fs");

module.exports = { read: (file) => fs.readFileSync(file, "utf8") };
