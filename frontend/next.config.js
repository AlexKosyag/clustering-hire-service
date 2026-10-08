/** @type {import('next').NextConfig} */
const BACKEND = process.env.BACKEND_URL || 'http://localhost:3000';

const nextConfig = {
  reactStrictMode: true,
  // standalone-сборка для production-образа (Docker)
  output: 'standalone',
  async rewrites() {
    // Проксируем API на backend, чтобы фронтенд ходил на тот же origin
    return [{ source: '/api/:path*', destination: `${BACKEND}/api/:path*` }];
  },
};

module.exports = nextConfig;
