import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // 本地 Go API 独立端口运行时，保持前端请求同源调用体验。
      "/api": "http://localhost:8080"
    }
  }
});
