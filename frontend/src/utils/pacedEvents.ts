type TimerHandle = ReturnType<typeof setTimeout>;

export interface PacedEventDispatcher<T> {
  enqueue: (event: T) => void;
  clear: () => void;
  waitForIdle: () => Promise<void>;
}

interface PacedEventDispatcherOptions<T> {
  intervalMs: number;
  onEvent: (event: T) => void;
  setTimer?: (callback: () => void, delay: number) => TimerHandle;
  clearTimer?: (timer: TimerHandle) => void;
}

export function createPacedEventDispatcher<T>({
  intervalMs,
  onEvent,
  setTimer = setTimeout,
  clearTimer = clearTimeout
}: PacedEventDispatcherOptions<T>): PacedEventDispatcher<T> {
  const queue: T[] = [];
  const idleResolvers: Array<() => void> = [];
  let timer: TimerHandle | null = null;
  let running = false;

  function resolveIdle() {
    while (idleResolvers.length > 0) {
      idleResolvers.shift()?.();
    }
  }

  function dispatchNext() {
    const event = queue.shift();
    if (event !== undefined) {
      onEvent(event);
    }

    timer = setTimer(() => {
      timer = null;
      if (queue.length > 0) {
        dispatchNext();
        return;
      }

      running = false;
      resolveIdle();
    }, intervalMs);
  }

  return {
    enqueue(event: T) {
      queue.push(event);

      if (!running) {
        running = true;
        dispatchNext();
      }
    },
    clear() {
      queue.length = 0;

      if (timer !== null) {
        clearTimer(timer);
        timer = null;
      }

      running = false;
      resolveIdle();
    },
    waitForIdle() {
      if (!running && queue.length === 0 && timer === null) {
        return Promise.resolve();
      }

      return new Promise<void>((resolve) => idleResolvers.push(resolve));
    }
  };
}
