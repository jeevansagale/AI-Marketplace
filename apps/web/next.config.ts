import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
      {
        source: "/v1/:path*",
        destination: `${process.env.NEXT_PUBLIC_API_BASE ?? "http://localhost:8081"}/v1/:path*`,
      },
    ];
  },
};

export default nextConfig;