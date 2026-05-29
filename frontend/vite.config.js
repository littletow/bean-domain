import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue(), wails("./bindings")],
  server: {
    port: 9245,
    strictPort: true, // 强制使用该端口，避免 Vite 自动跳到其他端口
  },
});
