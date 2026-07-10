import test from "node:test";
import assert from "node:assert/strict";
import { channelFor, runScenario } from "../helpers/cli.ts";

test("open obligations consume visible channel capacity", () => {
  const report = runScenario("capacity-limits");
  const atlasAB = channelFor(report, "atlas", "alpha", "beta");
  const borealBG = channelFor(report, "boreal", "beta", "gamma");
  assert.ok(atlasAB.limit > atlasAB.available);
  assert.ok(borealBG.limit > borealBG.available);
  assert.ok(atlasAB.reserved + atlasAB.used > 0);
  assert.ok(borealBG.reserved + borealBG.used > 0);
});

test("capacity report keeps channel debt within assigned limits", () => {
  const report = runScenario("capacity-limits");
  for (const channel of report.channels) {
    assert.ok(channel.limit >= channel.reserved + channel.inFlight);
    assert.ok(channel.available >= 0);
    assert.ok(channel.utilizationBps >= 0);
    assert.ok(channel.utilizationBps <= 10000);
  }
});

