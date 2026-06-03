import { useEffect, useRef, useState } from "react";

import type { Report, SuggestedComment } from "../types/review";
import { copyToClipboard } from "../utils/clipboard";
import { formatFullReview, formatSingleComment } from "../utils/copyReview";

type CopyFeedback = "idle" | "copied" | "failed";

interface SuggestedCommentsProps {
  report: Report;
}

function getCommentLocation(comment: SuggestedComment): string {
  if (!comment.file) {
    return "通用评审";
  }

  return comment.line ? `${comment.file}:${comment.line}` : comment.file;
}

export function SuggestedComments({ report }: SuggestedCommentsProps) {
  const [feedback, setFeedback] = useState<Record<string, CopyFeedback>>({});
  const isMountedRef = useRef(true);
  const timersRef = useRef<number[]>([]);

  useEffect(() => {
    return () => {
      isMountedRef.current = false;
      timersRef.current.forEach((timer) => window.clearTimeout(timer));
    };
  }, []);

  async function copyText(key: string, text: string) {
    const result = await copyToClipboard(text);

    if (!isMountedRef.current) {
      return;
    }

    setFeedback((current) => ({ ...current, [key]: result.ok ? "copied" : "failed" }));

    const timer = window.setTimeout(() => {
      if (!isMountedRef.current) {
        return;
      }

      setFeedback((current) => ({ ...current, [key]: "idle" }));
    }, 1800);
    timersRef.current.push(timer);
  }

  return (
    <section className="suggested-comments" aria-labelledby="suggested-comments-title">
      <div className="section-heading suggested-comments__heading">
        <div>
          <p className="eyebrow">评审输出</p>
          <h3 id="suggested-comments-title">建议评论</h3>
        </div>
        <button
          type="button"
          className="secondary compact-button"
          onClick={() => {
            void copyText("full-review", formatFullReview(report));
          }}
        >
          {feedback["full-review"] === "copied"
            ? "已复制"
            : feedback["full-review"] === "failed"
              ? "复制失败"
              : "复制完整评审"}
        </button>
      </div>

      {report.comments.length === 0 ? (
        <div className="comments-empty" role="status">
          本次评审未生成建议评论。
        </div>
      ) : (
        <div className="comment-list">
          {report.comments.map((comment) => {
            const key = `comment-${comment.id}`;

            return (
              <article className="comment-card" key={comment.id}>
                <div className="comment-card__body">
                  <code>{getCommentLocation(comment)}</code>
                  <p>{comment.body || "未提供建议评论正文。"}</p>
                </div>
                <button
                  type="button"
                  className="secondary compact-button"
                  onClick={() => {
                    void copyText(key, formatSingleComment(comment));
                  }}
                >
                  {feedback[key] === "copied"
                    ? "已复制"
                    : feedback[key] === "failed"
                      ? "复制失败"
                      : "复制"}
                </button>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}
