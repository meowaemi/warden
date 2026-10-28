import { defineConfig, globalIgnores } from 'eslint/config';
import nextVitals from 'eslint-config-next/core-web-vitals';
import nextTS from 'eslint-config-next/typescript';
import prettier from 'eslint-config-prettier/flat';
// Next.js rules; formatting is left to Prettier.
export default defineConfig([
  ...nextVitals,
  ...nextTS,
  prettier,
  globalIgnores(['.next/**', 'out/**', 'next-env.d.ts', 'coverage/**']),
]);
