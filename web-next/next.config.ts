import type { NextConfig } from 'next';
// A static export to out/, which the Go server serves itself.
const config: NextConfig = { output: 'export', trailingSlash: true, images: { unoptimized: true } };
export default config;
