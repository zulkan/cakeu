export const DEFAULT_TIMEOUT_MS = 30_000;
export const CHAT_TIMEOUT_MS = 120_000;

export interface AbortScope {
  signal: AbortSignal;
  dispose: () => void;
}

export function abortableScope(
  timeoutMs: number,
  signal?: AbortSignal | null
): AbortScope {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => {
    controller.abort(
      new DOMException(`Request timed out after ${timeoutMs}ms`, 'TimeoutError')
    );
  }, timeoutMs);

  const forwardAbort = () => {
    controller.abort(signal?.reason);
  };

  if (signal) {
    if (signal.aborted) {
      forwardAbort();
    } else {
      signal.addEventListener('abort', forwardAbort, { once: true });
    }
  }

  return {
    signal: controller.signal,
    dispose: () => {
      clearTimeout(timeoutId);
      signal?.removeEventListener('abort', forwardAbort);
    },
  };
}
