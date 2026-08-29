import path from 'path'
import { fileURLToPath } from 'url'
import { defineConfig, loadEnv } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'
import { tanstackRouter } from '@tanstack/router-plugin/rspack'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig(({ envMode }) => {
  const env = loadEnv({ mode: envMode, prefixes: ['VITE_'] })
  const serverUrl =
    process.env.VITE_REACT_APP_SERVER_URL ||
    env.rawPublicVars.VITE_REACT_APP_SERVER_URL ||
    'http://localhost:3000'

  const isProd = envMode === 'production'
  const devProxy = Object.fromEntries(
    (['/api', '/mj', '/pg'] as const).map((key) => [
      key,
      { target: serverUrl, changeOrigin: true },
    ]),
  ) as Record<string, { target: string; changeOrigin: boolean }>

  return {
    plugins: [pluginReact()],
    // Rsbuild 2: replaces deprecated `performance.chunkSplit` (RSPack 2 aligned)
    splitChunks: {
      preset: 'none',
      cacheGroups: {
        default: {
          chunks: 'async',
          minChunks: 2,
          priority: -20,
          reuseExistingChunk: true,
        },
        defaultVendors: {
          test: /node_modules[\\/]/,
          chunks: 'async',
          priority: -10,
          reuseExistingChunk: true,
        },
        'vendor-react': {
          test: /node_modules[\\/](react|react-dom)[\\/]/,
          name: 'vendor-react',
          chunks: 'all',
          priority: 0,
          enforce: true,
        },
        'vendor-ui-primitives': {
          test: /node_modules[\\/](@base-ui|@radix-ui)[\\/]/,
          name: 'vendor-ui-primitives',
          chunks: 'async',
          priority: 0,
          enforce: true,
        },
        'vendor-tanstack': {
          test: /node_modules[\\/]@tanstack[\\/]/,
          name: 'vendor-tanstack',
          chunks: 'async',
          priority: 0,
          enforce: true,
        },
        'vendor-lucide-async': {
          test: /node_modules[\\/]lucide-react[\\/]/,
          name: false,
          chunks: 'async',
          priority: 20,
          reuseExistingChunk: true,
          enforce: true,
        },
        'vendor-motion-async': {
          test: /node_modules[\\/](framer-motion|motion|motion-dom|motion-utils)[\\/]/,
          name: false,
          chunks: 'async',
          priority: 20,
          reuseExistingChunk: true,
          enforce: true,
        },
        'vendor-markdown-async': {
          test: /node_modules[\\/](react-markdown|rehype-[^\\/]+|remark-[^\\/]+|unified|vfile(?:-message)?|hast-util-[^\\/]+|mdast-util-[^\\/]+|micromark(?:-[^\\/]+)?|unist-util-[^\\/]+)[\\/]/,
          name: false,
          chunks: 'async',
          priority: 20,
          reuseExistingChunk: true,
          enforce: true,
        },
        'vendor-calendar-async': {
          test: /node_modules[\\/](react-day-picker|date-fns|@date-fns[\\/]tz)[\\/]/,
          name: false,
          chunks: 'async',
          priority: 20,
          reuseExistingChunk: true,
          enforce: true,
        },
      },
    },
    source: {
      entry: {
        index: './src/main.tsx',
      },
    },
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    html: {
      template: './index.html',
    },
    server: {
      host: '0.0.0.0',
      strictPort: false,
      proxy: devProxy,
    },
    output: {
      // Production optimizations
      minify: isProd,
      target: 'web',
      distPath: {
        root: 'dist',
      },
      // Rely on Rsbuild default legalComments ("linked" → per-chunk *.LICENSE.txt) in all modes.
      // Do not set "none" in production: that strips minifier-preserved third-party notices and
      // extracted license files, which some distributions require for open-source compliance.
    },
    performance: {
      // Remove console in production
      removeConsole: isProd ? ['log'] : false,
      // Speed up repeated `rsbuild build` (local + CI when node_modules/.cache is preserved).
      // @see https://v2.rsbuild.dev/config/performance/build-cache
      buildCache: {
        cacheDigest: [process.env.VITE_REACT_APP_VERSION],
      },
    },
    tools: {
      rspack: {
        plugins: [
          tanstackRouter({
            target: 'react',
            // Dev: avoid per-route async chunks (reduces white flash on navigation + faster HMR feedback).
            // Prod: keep route-based code splitting.
            autoCodeSplitting: isProd,
          }),
        ],
      },
    },
  }
})
