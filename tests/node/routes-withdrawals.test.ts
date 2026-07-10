import test from "node:test";
import assert from "node:assert/strict";
import { accountBalance, byId, runScenario } from "../helpers/cli.ts";

test("multi-hop route exposes stable path and fee policy", () => {
  const report = runScenario("route-withdrawal");
  const route = byId(report.routes, "rt-alpha-gamma-alpha-gamma-mesh");
  assert.deepEqual(route.path, ["alpha", "beta", "gamma"]);
  assert.equal(route.hops, 2);
  assert.ok(route.feeBps <= 95);
});

test("withdrawals complete after the epoch hold", () => {
  const report = runScenario("route-withdrawal");
  assert.equal(report.withdrawals.length, 2);
  for (const withdrawal of report.withdrawals) {
    assert.equal(withdrawal.status, "completed");
    assert.ok(withdrawal.amount > 0);
  }
  const atlas = accountBalance(report, "op-atlas");
  const boreal = accountBalance(report, "op-boreal");
  assert.ok(atlas.free >= 42000);
  assert.ok(boreal.free >= 35000);
  assert.equal(report.summary.pendingWithdrawals, 0);
  assert.ok(report.summary.completedWithdrawals >= 77000);
});

