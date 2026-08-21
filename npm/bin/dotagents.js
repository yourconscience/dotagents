#!/usr/bin/env node
"use strict";

const fs = require("fs");
const path = require("path");
const { spawnSync } = require("child_process");

const binary = path.join(__dirname, "dotagents");

if (!fs.existsSync(binary)) {
  console.error("The dotagents binary was not downloaded during install.");
  console.error(`Run: node ${path.join(path.dirname(__dirname), "install.js")}`);
  console.error("Or reinstall with install scripts enabled: npm rebuild -g dotagents");
  process.exit(1);
}

const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);
