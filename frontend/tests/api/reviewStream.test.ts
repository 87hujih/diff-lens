import test from "node:test";
import assert from "node:assert/strict";
import { parseSSEMessage } from "../../src/api/reviewStream";

test("parseSSEMessage joins multi-line data and ignores comments and blank lines", () => {
  const message = [
    ": stream heartbeat",
    "",
    "event: ai_delta",
    'data: {"text":"first"',
    'data: ,"more":"second"}'
  ].join("\n");

  assert.deepEqual(parseSSEMessage(message), {
    type: "ai_delta",
    data: {
      text: "first",
      more: "second"
    }
  });
});
