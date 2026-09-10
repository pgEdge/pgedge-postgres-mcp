import { readFileSync } from 'node:fs';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// The client's version comes from package.json rather than a literal in the
// source, so that a release bumps it in one place. Keep this define in step
// with vitest.config.js, which needs it for the same reason.
const { version } = JSON.parse(
  readFileSync(new URL('./package.json', import.meta.url), 'utf8')
);

export default defineConfig({
  plugins: [react()],
  define: {
    __APP_VERSION__: JSON.stringify(version),
  },
  server: {
    port: 5173,
    proxy: {
      // Proxy MCP and LLM API endpoints to MCP server
      '/mcp': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/oauth': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        // /oauth/callback is a client-side route the SPA itself handles
        // (AuthContext reads its query string on mount), not a server
        // endpoint, so it must not be proxied like the rest of /oauth.
        bypass: (req) => {
          if (req.url.split('?')[0] === '/oauth/callback') {
            return req.url;
          }
        },
      },
      '/.well-known': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
});
