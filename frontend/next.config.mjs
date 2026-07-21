/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone', // Required for Docker multi-stage build
  experimental: {
    // Enable server actions if needed in future
  },
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080',
    NEXT_PUBLIC_WS_URL: process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080',
  },
};

export default nextConfig;
