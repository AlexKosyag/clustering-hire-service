/** @type {import('next').NextConfig} */
const BACKEND = process.env.BACKEND_URL || 'http://localhost:3000';
const BASE_PATH = process.env.BASE_PATH || '';

const nextConfig = {
  reactStrictMode: true,
  output: 'standalone',
  basePath: BASE_PATH,
  async rewrites() {
    return [
      {
        source: `${BASE_PATH}/api/:path*`,
        destination: `${BACKEND}/api/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;