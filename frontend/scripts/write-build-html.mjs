import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

// 基于脚本位置解析路径，确保从任意当前目录运行构建都可用。
const scriptDir = dirname(fileURLToPath(import.meta.url));
const root = resolve(scriptDir, "..");
const distDir = resolve(root, "dist");
const htmlPath = resolve(root, "index.html");
const outputPath = resolve(distDir, "index.html");
const cssPath = resolve(distDir, "assets", "index.css");

mkdirSync(distDir, { recursive: true });

let html = readFileSync(htmlPath, "utf8");
// 源 HTML 指向 Vite 开发入口；生产输出需要指向
// npm run build:assets 生成的 esbuild bundle 和可选 CSS 产物。
html = html.replace(
  '<script type="module" src="/src/main.tsx"></script>',
  `${existsSync(cssPath) ? '<link rel="stylesheet" href="/assets/index.css" />\n    ' : ""}<script type="module" src="/assets/index.js"></script>`
);

writeFileSync(outputPath, html);
