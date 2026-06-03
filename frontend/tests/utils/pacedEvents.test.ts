import test from "node:test";
import assert from "node:assert/strict";
import { createPacedEventDispatcher } from "../../src/utils/pacedEvents";

test("createPacedEventDispatcher sends the first event immediately and paces queued events", () => {
  const dispatched: number[] = [];
  const timers: Array<() => void> = [];
  const delays: number[] = [];

  const dispatcher = createPacedEventDispatcher<number>({
    intervalMs: 300,
    onEvent: (event) => dispatched.push(event),
    setTimer: (callback, delay) => {
      delays.push(delay);
      timers.push(callback);
      return timers.length;
    },
    clearTimer: () => {}
  });

  dispatcher.enqueue(1);
  dispatcher.enqueue(2);
  dispatcher.enqueue(3);

  assert.deepEqual(dispatched, [1]);
  assert.deepEqual(delays, [300]);

  timers.shift()?.();
  assert.deepEqual(dispatched, [1, 2]);
  assert.deepEqual(delays, [300, 300]);

  timers.shift()?.();
  assert.deepEqual(dispatched, [1, 2, 3]);
});

test("createPacedEventDispatcher can clear queued events", () => {
  const dispatched: number[] = [];
  const timers: Array<() => void> = [];
  let clearedTimer: number | null = null;

  const dispatcher = createPacedEventDispatcher<number>({
    intervalMs: 300,
    onEvent: (event) => dispatched.push(event),
    setTimer: (callback) => {
      timers.push(callback);
      return timers.length;
    },
    clearTimer: (timer) => {
      clearedTimer = timer;
    }
  });

  dispatcher.enqueue(1);
  dispatcher.enqueue(2);
  dispatcher.clear();
  timers.shift()?.();

  assert.deepEqual(dispatched, [1]);
  assert.equal(clearedTimer, 1);
});
