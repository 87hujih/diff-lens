#!/usr/bin/env python3
"""Serve the built frontend and proxy API calls to the local Go backend.

This is a Vite-free preview path for locked-down Windows shells where Vite cannot
spawn esbuild while loading vite.config.ts.
"""

from __future__ import annotations

import argparse
import os
import sys
import urllib.error
import urllib.request
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


SKIP_PROXY_HEADERS = {
    "connection",
    "content-encoding",
    "content-length",
    "keep-alive",
    "proxy-authenticate",
    "proxy-authorization",
    "te",
    "trailer",
    "transfer-encoding",
    "upgrade",
}


class StaticProxyHandler(SimpleHTTPRequestHandler):
    protocol_version = "HTTP/1.0"

    def __init__(
        self,
        *args: object,
        static_root: Path,
        backend: str,
        timeout: float,
        **kwargs: object,
    ) -> None:
        self.backend = backend.rstrip("/")
        self.timeout = timeout
        super().__init__(*args, directory=str(static_root), **kwargs)

    def log_message(self, fmt: str, *args: object) -> None:
        sys.stderr.write("%s - - [%s] %s\n" % (self.client_address[0], self.log_date_time_string(), fmt % args))

    def end_headers(self) -> None:
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Headers", "content-type, accept")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        super().end_headers()

    def do_OPTIONS(self) -> None:
        self.send_response(204)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def do_GET(self) -> None:
        if self.path.startswith("/api/"):
            self.proxy()
            return

        local_path = Path(self.translate_path(self.path))
        if not local_path.exists() and "." not in self.path.rsplit("/", 1)[-1]:
            self.path = "/index.html"
        super().do_GET()

    def do_POST(self) -> None:
        if self.path.startswith("/api/"):
            self.proxy()
            return
        self.send_error(404, "Only /api/* POST requests are proxied")

    def proxy(self) -> None:
        content_length = int(self.headers.get("Content-Length", "0") or "0")
        body = self.rfile.read(content_length) if content_length else None
        headers = {
            key: self.headers[key]
            for key in ("Accept", "Content-Type")
            if key in self.headers
        }
        request = urllib.request.Request(
            self.backend + self.path,
            data=body,
            headers=headers,
            method=self.command,
        )

        try:
            with urllib.request.urlopen(request, timeout=self.timeout) as upstream:
                self.send_response(upstream.status)
                for key, value in upstream.headers.items():
                    if key.lower() not in SKIP_PROXY_HEADERS:
                        self.send_header(key, value)
                self.send_header("Cache-Control", "no-store")
                self.send_header("Connection", "close")
                self.end_headers()
                while True:
                    chunk = upstream.read(8192)
                    if not chunk:
                        break
                    self.wfile.write(chunk)
                    self.wfile.flush()
        except urllib.error.HTTPError as exc:
            self.send_response(exc.code)
            self.send_header("Content-Type", exc.headers.get("Content-Type", "text/plain; charset=utf-8"))
            self.send_header("Connection", "close")
            self.end_headers()
            self.wfile.write(exc.read())
        except Exception as exc:  # pragma: no cover - exercised manually in local dev
            payload = f"proxy error: {exc!r}\n".encode("utf-8")
            self.send_response(502)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
            self.send_header("Content-Length", str(len(payload)))
            self.send_header("Connection", "close")
            self.end_headers()
            self.wfile.write(payload)


def parse_args() -> argparse.Namespace:
    repo_root = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(description="Serve frontend/dist and proxy /api to the Go backend.")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=5173)
    parser.add_argument("--backend", default="http://127.0.0.1:8080")
    parser.add_argument("--static-root", type=Path, default=repo_root / "frontend" / "dist")
    parser.add_argument("--timeout", type=float, default=90)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    static_root = args.static_root.resolve()
    if not static_root.exists():
        sys.stderr.write(f"Static root does not exist: {static_root}\n")
        sys.stderr.write("Run: npm --prefix frontend run build\n")
        return 1

    handler = lambda *handler_args, **handler_kwargs: StaticProxyHandler(
        *handler_args,
        static_root=static_root,
        backend=args.backend,
        timeout=args.timeout,
        **handler_kwargs,
    )
    server = ThreadingHTTPServer((args.host, args.port), handler)
    print(f"Serving {static_root} at http://{args.host}:{args.port}")
    print(f"Proxying /api/* to {args.backend}")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
