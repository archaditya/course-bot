/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone', // Required for Docker multi-stage build
  experimental: {
    // Enable server actions if needed in future
  },
};

export default nextConfig;
