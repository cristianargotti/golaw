const fs = require("fs");

module.exports = { read: (file) => fs.readFileSync(file, "utf8") };
