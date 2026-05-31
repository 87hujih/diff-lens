#!/usr/bin/env python3
"""Minimal OpenAI-compatible chat completions mock for release QA.

The service intentionally does not simulate model intelligence. It echoes a
valid structured analysis that cites evidence refs present in the bounded PR
context sent by diff-lens.
"""

from __future__ import annotations

import argparse
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any


def _raw_decode_object(text: str) -> dict[str, Any]:
    decoder = json.JSONDecoder()
    for index, char in enumerate(text):
        if char != "{":
            continue
        try:
            value, _ = decoder.raw_decode(text[index:])
        except json.JSONDecodeError:
            continue
        if isinstance(value, dict):
            return value
    return {}


def _extract_context(payload: dict[str, Any]) -> dict[str, Any]:
    messages = payload.get("messages", [])
    if not isinstance(messages, list):
        return {}

    for message in reversed(messages):
        if not isinstance(message, dict):
            continue
        content = message.get("content")
        if isinstance(content, str):
            context = _raw_decode_object(content)
            if context:
                return context
    return {}


def _first_string(values: Any) -> str:
    if isinstance(values, list):
        for value in values:
            if isinstance(value, str) and value:
                return value
    return ""


def _evidence_from_context(context: dict[str, Any]) -> dict[str, Any]:
    for risk in context.get("rule_risks", []) or []:
        if not isinstance(risk, dict):
            continue
        refs = []
        risk_id = risk.get("id")
        if isinstance(risk_id, str) and risk_id:
            refs.append(risk_id)
        for ref in risk.get("evidence_refs", []) or []:
            if isinstance(ref, str) and ref:
                refs.append(ref)
        if refs:
            return {
                "refs": refs,
                "file": risk.get("file") if isinstance(risk.get("file"), str) else "",
                "line": risk.get("line") if isinstance(risk.get("line"), int) else 1,
                "category": risk.get("category") if isinstance(risk.get("category"), str) else "correctness",
                "rule_id": risk.get("rule_id") if isinstance(risk.get("rule_id"), str) else "",
            }

    for file_info in context.get("files", []) or []:
        if not isinstance(file_info, dict):
            continue
        filename = file_info.get("filename") if isinstance(file_info.get("filename"), str) else ""
        for snippet in file_info.get("snippets", []) or []:
            if not isinstance(snippet, dict):
                continue
            snippet_id = snippet.get("id")
            if isinstance(snippet_id, str) and snippet_id:
                line = snippet.get("start_line")
                return {
                    "refs": [snippet_id],
                    "file": filename or (snippet.get("file") if isinstance(snippet.get("file"), str) else ""),
                    "line": line if isinstance(line, int) and line > 0 else 1,
                    "category": "correctness",
                    "rule_id": "",
                }

    fallback_ref = _first_string(context.get("evidence_refs"))
    return {
        "refs": [fallback_ref] if fallback_ref else ["mock-evidence-ref"],
        "file": "",
        "line": 1,
        "category": "correctness",
        "rule_id": "",
    }


def build_analysis(request_payload: dict[str, Any]) -> dict[str, Any]:
    context = _extract_context(request_payload)
    evidence = _evidence_from_context(context)
    pr_summary = context.get("pr_summary") if isinstance(context.get("pr_summary"), dict) else {}
    title = pr_summary.get("title") if isinstance(pr_summary.get("title"), str) else "the PR"
    file_name = evidence["file"]
    line = evidence["line"]

    return {
        "summary": (
            "Mock analysis completed for release QA. Review the cited evidence "
            f"before treating findings on {title} as actionable."
        ),
        "risks": [
            {
                "id": "mock-ai-context-review",
                "severity": "medium" if file_name else "low",
                "confidence": 0.72,
                "category": evidence["category"],
                "title": "Mock AI follow-up on cited PR context",
                "file": file_name,
                "line": line,
                "rule_id": evidence["rule_id"],
                "evidence_refs": evidence["refs"],
                "reason": "The mock service cites existing bounded context to exercise the AI success path.",
                "suggestion": "Use this only to verify parsing, normalization, and UI rendering; validate real code with a real model.",
            }
        ],
        "comments": [
            {
                "id": "mock-comment-context-review",
                "file": file_name,
                "line": line,
                "body": "Mock LLM QA note: please verify this cited context with a real reviewer or model before acting on it.",
                "evidence_refs": evidence["refs"],
            }
        ],
        "attention_items": [
            "Confirm the final release run uses a real model or clearly records that this was mock-only."
        ],
        "meta": {"completed": True},
    }


class Handler(BaseHTTPRequestHandler):
    server_version = "diff-lens-mock-openai/1.0"

    def do_POST(self) -> None:
        if self.path != "/v1/chat/completions":
            self.send_error(404, "not found")
            return

        try:
            length = int(self.headers.get("Content-Length", "0") or "0")
        except ValueError:
            self.send_error(400, "invalid content length")
            return

        try:
            body = self.rfile.read(length)
            request_payload = json.loads(body or b"{}")
        except json.JSONDecodeError:
            self.send_error(400, "invalid json")
            return
        if not isinstance(request_payload, dict):
            self.send_error(400, "json body must be an object")
            return

        analysis = build_analysis(request_payload)
        response = {
            "id": "chatcmpl-mock-diff-lens",
            "object": "chat.completion",
            "created": 0,
            "model": request_payload.get("model") or "mock-model",
            "choices": [
                {
                    "index": 0,
                    "message": {
                        "role": "assistant",
                        "content": json.dumps(analysis, separators=(",", ":")),
                    },
                    "finish_reason": "stop",
                }
            ],
        }
        encoded = json.dumps(response).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def log_message(self, format: str, *args: Any) -> None:
        print("%s - %s" % (self.address_string(), format % args))


def main() -> None:
    parser = argparse.ArgumentParser(description="Run a minimal OpenAI-compatible mock server.")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=8787)
    args = parser.parse_args()

    server = ThreadingHTTPServer((args.host, args.port), Handler)
    print(f"mock OpenAI-compatible server listening on http://{args.host}:{args.port}")
    print("POST /v1/chat/completions")
    server.serve_forever()


if __name__ == "__main__":
    main()
