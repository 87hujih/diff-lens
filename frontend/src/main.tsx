import React from "react";
import { createRoot } from "react-dom/client";

import App from "./App";

// StrictMode 在本地开发时暴露不安全的渲染副作用。
createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
