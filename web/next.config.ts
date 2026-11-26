import type { NextConfig } from "next";

const apiOrigin = process.env.API_URL ?? process.env.API_ORIGIN ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  experimental: {
    agentFeedback: true,
  },
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
  rewrites() {
    return [
      {
        source: "/api/v1/:path*",
        destination: `${apiOrigin}/api/v1/:path*`,
      },
      {
        source: "/healthz",
        destination: `${apiOrigin}/healthz`,
      },
    ];
  },
};

export default nextConfig;
