import test from "node:test";
import assert from "node:assert/strict";
import { byId, runScenario } from "../helpers/cli.ts";

test("higher reputation operators receive larger assigned capacity", () => {
  const report = runScenario("balanced-reputation");
  const atlas = byId(report.operators, "atlas");
  const boreal = byId(report.operators, "boreal");
  const cypher = byId(report.operators, "cypher");
  assert.equal(atlas.band, "prime");
  assert.equal(boreal.band, "senior");
  assert.ok(atlas.score > boreal.score);
  assert.ok(boreal.score > cypher.score);
  assert.ok(atlas.assignedCapacity > boreal.assignedCapacity);
  assert.ok(boreal.assignedCapacity > cypher.assignedCapacity);
});

test("settled routes credit operator fee accounts", () => {
  const report = runScenario("balanced-reputation");
  const atlasBalance = report.accounts.find((row: any) => row.account === "op-atlas" && row.asset === "ZUSD");
  const borealBalance = report.accounts.find((row: any) => row.account === "op-boreal" && row.asset === "ZUSD");
  assert.ok(atlasBalance.free > 0);
  assert.ok(borealBalance.free > 0);
  assert.equal(report.summary.settlement.settled, 2);
  assert.ok(report.summary.settlement.feeAmount > 0);
});

