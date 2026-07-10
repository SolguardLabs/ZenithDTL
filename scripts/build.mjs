import { existsSync, mkdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const outDir = join(root, "out");
const exeName = process.platform === "win32" ? "zenithdtl.exe" : "zenithdtl";
const output = join(outDir, exeName);

function run(command, args, options = {}) {
  return spawnSync(command, args, {
    cwd: root,
    encoding: "utf8",
    stdio: options.stdio ?? "pipe",
    shell: false,
  });
}

function commandExists(command) {
  if (process.platform === "win32") {
    return run("where.exe", [command]).status === 0;
  }
  return spawnSync("sh", ["-c", `command -v "${command.replaceAll('"', '\\"')}"`], {
    cwd: root,
    encoding: "utf8",
    stdio: "pipe",
  }).status === 0;
}

mkdirSync(outDir, { recursive: true });

if (!commandExists("go")) {
  console.error("Go toolchain not found. Install Go 1.22+ or add go to PATH.");
  process.exit(127);
}

const result = run("go", ["build", "-trimpath", "-o", output, "./cmd/zenithdtl"], {
  stdio: "inherit",
});

if (result.status !== 0) {
  process.exit(result.status ?? 1);
}

if (!existsSync(output)) {
  console.error(`build completed but binary was not written: ${output}`);
  process.exit(1);
}

console.log(output);
