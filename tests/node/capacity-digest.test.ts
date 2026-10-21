import test from "node:test";
import assert from "node:assert/strict";
import { runScenario } from "../helpers/cli.ts";

test("capacity digest reconciles assigned limit and open exposure", () => {
  const report = runScenario("capacity-limits");
  assert.equal(report.capacity.length, report.operators.length);
  const atlas = report.capacity.find((row: any) => row.operator === "atlas");
  assert.ok(atlas);
  assert.ok(atlas.openExposure > 0);
  assert.equal(atlas.assignedHeadroom, atlas.assignedLimit - atlas.openExposure);
  assert.ok(atlas.channelHeadroom >= 0);
  assert.ok(atlas.utilizationBps > 0);
});

test("capacity digest is sorted and stable across identical scenarios", () => {
  const first = runScenario("balanced-reputation").capacity;
  const second = runScenario("balanced-reputation").capacity;
  assert.deepEqual(first, second);
  assert.deepEqual(first.map((row: any) => row.operator), ["atlas", "boreal", "cypher", "delta"]);
});
