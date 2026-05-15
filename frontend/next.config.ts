import type { NextConfig } from "next";
import withSerwistInit from "@serwist/next";

const baseConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://localhost:8080/api/:path*",
      },
    ];
  },
};

let nextConfig: NextConfig;

if (process.env.NODE_ENV === "production") {
  // Serwist uses a webpack plugin — only load it for production builds
  // which run with `next build --webpack`.
  const withSerwist = withSerwistInit({
    swSrc: "src/app/sw.ts",
    swDest: "public/sw.js",
  });
  nextConfig = withSerwist(baseConfig);
} else {
  nextConfig = baseConfig;
}

export default nextConfig;
