import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import legacy from '@vitejs/plugin-legacy'
import faroUploader from '@grafana/faro-rollup-plugin'
import { fileURLToPath, URL } from 'node:url'

// Backend origin the dev-server proxy forwards /api, /ping, /docs to.
// Host dev: defaults to localhost:3000. Containerized dev: docker-compose.dev.yml
// sets VITE_PROXY_TARGET=http://backend:3000 to reach the backend service.
const proxyTarget = process.env.VITE_PROXY_TARGET ?? 'http://localhost:3000'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    legacy({
      targets: ['ios >= 13', 'chrome >= 64', 'safari >= 13'],
    }),
    // Upload source maps to Grafana Faro at build time so stack traces in the
    // Frontend Observability UI are deobfuscated. Only runs when credentials
    // are present (i.e. production builds). Get values from Grafana Cloud →
    // Frontend Observability → your app → Settings → Source Maps.
    ...(process.env.GRAFANA_FARO_API_KEY ? [faroUploader({
      appName: process.env.VITE_FARO_APP_NAME ?? 'actionphase',
      endpoint: process.env.GRAFANA_FARO_SOURCEMAP_ENDPOINT ?? '',
      apiKey: process.env.GRAFANA_FARO_API_KEY,
      appId: process.env.GRAFANA_FARO_APP_ID ?? '',
      stackId: process.env.GRAFANA_FARO_STACK_ID ?? '',
      gzipContents: true,
    })] : []),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    host: true,
    // Allow the compose service hostname so the Playwright container (and any
    // other in-network client) can reach the dev server. Vite 7 rejects
    // unknown Host headers with 403 by default.
    allowedHosts: ['localhost', 'frontend'],
    proxy: {
      '/api': {
        target: proxyTarget,
        changeOrigin: true,
        secure: false,
      },
      '/ping': {
        target: proxyTarget,
        changeOrigin: true,
        secure: false,
      },
      '/health': {
        target: proxyTarget,
        changeOrigin: true,
        secure: false,
      },
      // Uploaded avatars/banners are served by the backend at /uploads/*. In dev
      // the backend hands out relative /uploads URLs (see docker-compose.dev.yml),
      // so the browser requests them from the Vite origin and we proxy them here.
      '/uploads': {
        target: proxyTarget,
        changeOrigin: true,
        secure: false,
      },
      '/docs': {
        target: proxyTarget,
        changeOrigin: true,
        secure: false,
      },
      '/api/v1/docs': {
        target: proxyTarget,
        changeOrigin: true,
        secure: false,
      },
    },
  },
  optimizeDeps: {
    include: ['axios'],
  },
  build: {
    sourcemap: process.env.SOURCEMAP === 'true' ? 'hidden' : false,
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-react': ['react', 'react-dom', 'react-router-dom'],
          'vendor-query': ['@tanstack/react-query'],
          'vendor-ui': ['@headlessui/react', '@heroicons/react', 'lucide-react'],
          'vendor-markdown': ['marked', 'dompurify'],
          'vendor-utils': ['axios', 'date-fns', 'clsx', 'tailwind-merge'],
        },
      },
    },
    chunkSizeWarningLimit: 1000,
    modulePreload: false,
    minify: 'esbuild',
  },
  esbuild: {
    supported: {
      destructuring: true,
    },
  },
  // Test config lives in vitest.config.ts, which takes precedence over this
  // file when vitest resolves its config. A `test` block here would be silently
  // ignored — put test settings in vitest.config.ts instead.
})