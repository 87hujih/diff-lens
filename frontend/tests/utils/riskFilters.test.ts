import test from "node:test";
import assert from "node:assert/strict";
import { filterRisksBySeverity, getVisibleRisks } from "../../src/utils/riskFilters";
import type { Risk } from "../../src/types/review";

function risk(id: string, severity: string): Risk {
  return {
    id,
    source: "rule",
    severity,
    confidence: 0.8,
    category: "test",
    title: `Risk ${id}`,
    reason: "Because.",
    suggestion: "Fix it."
  };
}

const risks = [risk("a", "low"), risk("b", "HIGH"), risk("c", "medium"), risk("d", "unknown")];

test("filterRisksBySeverity filters by normalized severity", () => {
  assert.deepEqual(
    filterRisksBySeverity(risks, "high").map((item) => item.id),
    ["b"]
  );
});

test("getVisibleRisks filters by severity before sorting", () => {
  assert.deepEqual(
    getVisibleRisks(risks, "all").map((item) => item.id),
    ["b", "c", "a", "d"]
  );
});
