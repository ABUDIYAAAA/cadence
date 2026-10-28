import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  /* config options here */
  devIndicators: false,
  cacheComponents: true,
  partialPrefetching: true,
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
  async rewrites() {
    return [
      {
        source: "/auth/:path*",
        destination: "/:path*",
      },
      {
        source: "/setup-account",
        destination: "/setup-password",
      },
    ];
  },
};

export default nextConfig;
