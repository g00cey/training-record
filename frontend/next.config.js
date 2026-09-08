const path = require('path');

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  // このディレクトリを file tracing のルートに固定（standalone ビルドを Docker で安定させる）
  outputFileTracingRoot: path.join(__dirname),
  reactStrictMode: true,
};

module.exports = nextConfig;
