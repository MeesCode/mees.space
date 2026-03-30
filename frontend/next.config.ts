import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  distDir: "../dist",
  images: { unoptimized: true },
  async rewrites() {
    return [
      { source: "/api/:path*", destination: "http://localhost:8080/api/:path*" },
      {
        source: "/uploads/:path*",
        destination: "http://localhost:8080/uploads/:path*",
      },
    ];
  },
};

export default nextConfig;
