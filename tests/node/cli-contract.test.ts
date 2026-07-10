import test from "node:test";
import assert from "node:assert/strict";
import { runCli, runScenario, validateScenario } from "../helpers/cli.ts";

test("CLI lists deterministic scenarios", () => {
  const scenarios = JSON.parse(runCli(["list", "--json"]));
  assert.deepEqual(
    scenarios.map((scenario: any) => scenario.name),
    ["balanced-reputation", "capacity-limits", "penalty-cycle", "route-withdrawal"],
  );
});

test("validation command accepts built-in scenarios", () => {
  for (const scenario of ["balanced-reputation", "capacity-limits", "penalty-cycle", "route-withdrawal"]) {
    assert.equal(validateScenario(scenario), `ok ${scenario}`);
  }
});

test("JSON report has stable public contract", () => {
  const report = runScenario("balanced-reputation", ["--events"]);
  assert.equal(report.protocol, "ZenithDTL");
  assert.equal(report.scenario, "balanced-reputation");
  assert.ok(Array.isArray(report.accounts));
  assert.ok(Array.isArray(report.vaults));
  assert.ok(Array.isArray(report.operators));
  assert.ok(Array.isArray(report.channels));
  assert.ok(Array.isArray(report.routes));
  assert.ok(Array.isArray(report.obligations));
  assert.ok(Array.isArray(report.withdrawals));
  assert.ok(Array.isArray(report.events));
  assert.equal(report.summary.conservationOk, true);
});

