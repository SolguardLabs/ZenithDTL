import test from "node:test";
import assert from "node:assert/strict";
import { byId, runScenario } from "../helpers/cli.ts";

test("penalty cycle moves operator into watchlist band", () => {
  const report = runScenario("penalty-cycle");
  const cypher = byId(report.operators, "cypher");
  assert.equal(cypher.status, "watchlist");
  assert.ok(cypher.score < 645);
  assert.ok(cypher.pendingNegative >= 0);
});

test("expired obligations are visible as closed settlement records", () => {
  const report = runScenario("penalty-cycle", ["--events"]);
  const expired = report.obligations.filter((row: any) => row.status === "expired");
  const reputationEvents = report.events.filter((event: any) => event.kind === "reputation.updated");
  assert.equal(expired.length, 1);
  assert.ok(reputationEvents.length >= 2);
  assert.equal(report.summary.settlement.cancelled, 1);
});

