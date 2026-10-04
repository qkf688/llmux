import { fileURLToPath } from "node:url"
import tailwindcss from "@tailwindcss/vite"
import { defineConfig } from "vitest/config"
import react from '@vitejs/plugin-react-swc'

// https://vite.dev/config/
export default defineConfig({
    plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      // Vite 8 是纯 ESM（type: module），__dirname 不再存在：
      // 用标准 ESM 的 URL 换算绝对路径，不依赖打包器是否注入 CJS 垫片。
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
  },
  build: {
    chunkSizeWarningLimit: 1000,
    // Vite 8 的打包器换成 Rolldown 后，output.manualChunks 的对象形式已被移除
    // （官方迁移指南：not supported anymore），等价能力是 codeSplitting.groups。
    rolldownOptions: {
      output: {
        codeSplitting: {
          // 分组用 [\\/] 而非 /：模块 id 在 Windows 上可能带反斜杠。
          // 结尾的 [\\/] 保证 react 不会误吃掉 react-dom / react-router-dom。
          groups: [
            // React 生态系统
            {
              name: "react-vendor",
              test: /[\\/]node_modules[\\/](react|react-dom|react-router-dom)[\\/]/,
            },
            // UI 组件库
            { name: "ui-vendor", test: /[\\/]node_modules[\\/]@radix-ui[\\/]/ },
            // 图表库
            { name: "charts-vendor", test: /[\\/]node_modules[\\/](recharts|lucide-react)[\\/]/ },
            // 表单和验证
            {
              name: "form-vendor",
              test: /[\\/]node_modules[\\/](react-hook-form|zod|@hookform[\\/]resolvers)[\\/]/,
            },
            // 工具库
            {
              name: "utils-vendor",
              test: /[\\/]node_modules[\\/](clsx|tailwind-merge|class-variance-authority)[\\/]/,
            },
          ],
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:7070',
        changeOrigin: true,
        secure: false,
      }
    }
  }
})
 
