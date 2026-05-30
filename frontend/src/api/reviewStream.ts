import type { AnalyzeRequest, ReviewEvent, ReviewEventType } from "../types/review";

// EventHandler 让 UI 状态管理与事件流解析解耦。
type EventHandler = (event: ReviewEvent) => void;

// analyzeReviewStream 发送分析请求，并把解析后的 SSE 消息交给调用方。
export async function analyzeReviewStream(
  request: AnalyzeRequest,
  onEvent: EventHandler,
  signal?: AbortSignal
): Promise<void> {
  const response = await fetch("/api/reviews/analyze/stream", {
    method: "POST",
    headers: {
      Accept: "text/event-stream",
      "Content-Type": "application/json"
    },
    body: JSON.stringify(request),
    signal
  });

  if (!response.ok || !response.body) {
    throw new Error(`Stream request failed with status ${response.status}`);
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { value, done } = await reader.read();
    if (done) {
      break;
    }

    // SSE chunk 可能截断在消息中间，因此保留最后一个未完整帧。
    buffer += decoder.decode(value, { stream: true });
    const messages = buffer.split(/\n\n/);
    buffer = messages.pop() ?? "";

    for (const message of messages) {
      const event = parseSSEMessage(message);
      if (event) {
        onEvent(event);
      }
    }
  }
}

// parseSSEMessage 处理 Go 后端输出的简单 event/data 帧格式。
function parseSSEMessage(message: string): ReviewEvent | null {
  const eventLine = message.split("\n").find((line) => line.startsWith("event: "));
  const dataLine = message.split("\n").find((line) => line.startsWith("data: "));

  if (!eventLine || !dataLine) {
    return null;
  }

  return {
    type: eventLine.slice("event: ".length) as ReviewEventType,
    data: JSON.parse(dataLine.slice("data: ".length))
  };
}
