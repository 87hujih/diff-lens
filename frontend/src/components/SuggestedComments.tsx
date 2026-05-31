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
    return "General review";
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
          <p className="eyebrow">Review output</p>
          <h3 id="suggested-comments-title">Suggested comments</h3>
        </div>
        <button
          type="button"
          className="secondary compact-button"
          onClick={() => {
            void copyText("full-review", formatFullReview(report));
          }}
        >
          {feedback["full-review"] === "copied"
            ? "Copied"
            : feedback["full-review"] === "failed"
              ? "Copy failed"
              : "Copy full review"}
        </button>
      </div>

      {report.comments.length === 0 ? (
        <div className="comments-empty" role="status">
          No suggested comments were generated for this review.
        </div>
      ) : (
        <div className="comment-list">
          {report.comments.map((comment) => {
            const key = `comment-${comment.id}`;

            return (
              <article className="comment-card" key={comment.id}>
                <div className="comment-card__body">
                  <code>{getCommentLocation(comment)}</code>
                  <p>{comment.body || "No suggested comment body was provided."}</p>
                </div>
                <button
                  type="button"
                  className="secondary compact-button"
                  onClick={() => {
                    void copyText(key, formatSingleComment(comment));
                  }}
                >
                  {feedback[key] === "copied"
                    ? "Copied"
                    : feedback[key] === "failed"
                      ? "Copy failed"
                      : "Copy"}
                </button>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}
