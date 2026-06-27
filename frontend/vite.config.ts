import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

function includesAny(id: string, patterns: string[]) {
  return patterns.some((pattern) => id.includes(pattern));
}

function vendorChunk(id: string) {
  const normalizedId = id.replace(/\\/g, '/');
  if (!normalizedId.includes('node_modules')) return undefined;
  if (
    normalizedId.includes('/react/') ||
    normalizedId.includes('/react-dom/') ||
    normalizedId.includes('/react-router/') ||
    normalizedId.includes('/react-router-dom/') ||
    normalizedId.includes('/scheduler/')
  ) {
    return 'vendor-react';
  }
  if (
    normalizedId.includes('/ol/') ||
    normalizedId.includes('/rbush/') ||
    normalizedId.includes('/quickselect/') ||
    normalizedId.includes('/earcut/') ||
    normalizedId.includes('/pbf/') ||
    normalizedId.includes('/geotiff/')
  ) {
    return 'vendor-map';
  }
  if (normalizedId.includes('/lucide-react/')) {
    return 'vendor-icons';
  }
  if (
    normalizedId.includes('/@ant-design/icons/') ||
    normalizedId.includes('/@ant-design/icons-svg/')
  ) {
    return 'vendor-antd-icons';
  }
  if (
    normalizedId.includes('/@rc-component/') ||
    normalizedId.includes('/async-validator/') ||
    normalizedId.includes('/@ant-design/cssinjs/') ||
    normalizedId.includes('/@emotion/') ||
    normalizedId.includes('/@ant-design/fast-color/') ||
    normalizedId.includes('/@ant-design/colors/') ||
    normalizedId.includes('/rc-')
  ) {
    return 'vendor-antd-rc';
  }
  if (
    normalizedId.includes('/@ant-design/') ||
    normalizedId.includes('/dayjs/') ||
    normalizedId.includes('/classnames/') ||
    normalizedId.includes('/copy-to-clipboard/') ||
    normalizedId.includes('/resize-observer-polyfill/') ||
    normalizedId.includes('/scroll-into-view-if-needed/') ||
    normalizedId.includes('/compute-scroll-into-view/') ||
    normalizedId.includes('/throttle-debounce/') ||
    normalizedId.includes('/antd/')
  ) {
    return 'vendor-antd';
  }
  return 'vendor-misc';
}

export default defineConfig({
  plugins: [react()],
  build: {
    chunkSizeWarningLimit: 900,
    rollupOptions: {
      output: {
        manualChunks: vendorChunk,
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
});
