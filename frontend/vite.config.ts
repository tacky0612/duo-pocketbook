import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// GitHub Pages のサブパス配信でも動くよう base は相対パスにする。
export default defineConfig({
  base: "./",
  plugins: [react(), tailwindcss()],
});
