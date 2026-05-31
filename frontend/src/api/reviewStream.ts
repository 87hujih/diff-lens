import type { AnalyzeRequest, ReviewEvent, ReviewEventType } from "../types/review";

// EventHandler 让界面状态管理与事件流解析解耦。
type EventHandler = (event: ReviewEvent) => void;

// analyzeReviewStream 发送分析请求，并把解析后的 SSE 消息交给调用方。
export async function analyzeReviewStream(
  request: AnalyzeRequest,
  onEvent: EventHandler,
  signal?: AbortSignal
): Promise<void> {
  try {
    const response = await fetch("/api/reviews/analyze/stream", {
      method: "POST",
      headers: {
        Accept: "text/event-stream",
        "Content-Type": "application/json"
      },
      body: JSON.stringify(request),
      signal
    });

    if (!response.ok) {
      throw new Error(
        `Stream request failed with status ${response.status} ${response.statusText}`.trim()
      );
    }

    if (!response.body) {
      throw new Error("Stream request failed: response body is missing");
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      const { value, done } = await reader.read();
      if (done) {
        break;
      }

      // SSE 数据块可能截断在消息中间，因此保留最后一个未完整帧。
      buffer += decoder.decode(value, { stream: true });
      const messages = buffer.split(/\r?\n\r?\n/);
      buffer = messages.pop() ?? "";

      for (const message of messages) {
        const event = parseSSEMessage(message);
        if (event) {
          onEvent(event);
        }
      }
    }

    buffer += decoder.decode();

    for (const message of buffer.split(/\r?\n\r?\n/)) {
      const event = parseSSEMessage(message);
      if (event) {
        onEvent(event);
      }
    }
  } catch (error) {
    if (isAbortError(error)) {
      return;
    }

    throw error;
  }
}

// isAbortError 将主动取消请求视作静默结束。
function isAbortError(error: unknown): boolean {
  return error instanceof Error && error.name === "AbortError";
}

// parseSSEMessage 处理标准 SSE event/data 帧格式。
export function parseSSEMessage(message: string): ReviewEvent | null {
  let eventType: ReviewEventType | null = null;
  const dataLines: string[] = [];

  for (const rawLine of message.split(/\r?\n/)) {
    if (rawLine === "" || rawLine.startsWith(":")) {
      continue;
    }

    const separatorIndex = rawLine.indexOf(":");
    const field = separatorIndex === -1 ? rawLine : rawLine.slice(0, separatorIndex);
    let value = separatorIndex === -1 ? "" : rawLine.slice(separatorIndex + 1);

    if (value.startsWith(" ")) {
      value = value.slice(1);
    }

    if (field === "event") {
      eventType = value as ReviewEventType;
    }

    if (field === "data") {
      dataLines.push(value);
    }
  }

  if (!eventType || dataLines.length === 0) {
    return null;
  }

  try {
    return {
      type: eventType,
      data: JSON.parse(dataLines.join("\n"))
    };
  } catch (error) {
    const reason = error instanceof Error ? error.message : "unknown parse error";
    throw new Error(`Failed to parse SSE data for event "${eventType}": ${reason}`);
  }
}
