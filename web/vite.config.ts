import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  publicDir: false,
  css: {
    transformer: "lightningcss",
  },
  resolve: {
    alias: {
      "@": new URL("./src", import.meta.url).pathname,
    },
  },
  build: {
    assetsInlineLimit: 0,
    chunkSizeWarningLimit: 5000,
    cssCodeSplit: false,
  },
  test: {
    include: ["src/**/*.test.{ts,tsx}", "eslint.boundaries.test.js"],
  },
});
