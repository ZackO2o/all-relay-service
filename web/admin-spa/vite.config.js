import { defineConfig, loadEnv } from "vite"
import vue from "@vitejs/plugin-vue"
import AutoImport from "unplugin-auto-import/vite"
import Components from "unplugin-vue-components/vite"
import { ElementPlusResolver } from "unplugin-vue-components/resolvers"
import { fileURLToPath, URL } from "node:url"

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "")
  const apiTarget = env.VITE_API_TARGET || "http://localhost:3000"
  const httpProxy = env.VITE_HTTP_PROXY || env.HTTP_PROXY || env.http_proxy
  const basePath = env.VITE_APP_BASE_URL || (mode === "development" ? "/admin/" : "/admin-next/")
  const proxyConfig = { target: apiTarget, changeOrigin: true, secure: false }
  if (httpProxy && mode === "development") {
    process.env.HTTP_PROXY = httpProxy
    process.env.HTTPS_PROXY = httpProxy
  }
  return {
    base: basePath,
    plugins: [
      vue(),
      AutoImport({ resolvers: [ElementPlusResolver()], imports: ["vue", "vue-router", "pinia"] }),
      Components({ resolvers: [ElementPlusResolver()] }),
    ],
    resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
    server: {
      port: 3001, host: true,
      proxy: {
        "/webapi": { ...proxyConfig, rewrite: (p) => p.replace(/^\/webapi/, "") },
        "/apiStats": { ...proxyConfig },
      },
    },
    build: {
      outDir: "dist",
      assetsDir: "assets",
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (!id.includes("node_modules")) return
            if (id.includes("element-plus")) return "element-plus"
            if (id.includes("chart.js")) return "chart"
            if (id.includes("vue") || id.includes("pinia") || id.includes("vue-router")) return "vue-vendor"
            return "vendor"
          },
        },
      },
    },
  }
})
