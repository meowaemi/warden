import { defineConfig } from 'vitest/config';
import { fileURLToPath } from 'node:url';
// Tests run in jsdom; @ points to src, as in tsconfig.
export default defineConfig({
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  test: { environment: 'jsdom', restoreMocks: true, include: ['src/**/*.test.{ts,tsx}'] },
});
