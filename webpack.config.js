"use strict";

const path = require("path");
const SveltePreprocess = require("svelte-preprocess");

module.exports = {
  entry: "./src/main.ts",
  devtool: "inline-source-map",
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        use: "ts-loader",
        exclude: /node_modules/
      },
      {
        test: /\.(html|svelte)$/,
        use: {
          loader: "svelte-loader",
          options: { preprocess: SveltePreprocess() }
        }
      },
      {
        // required to prevent errors from Svelte on Webpack 5+, omit on Webpack 4
        test: /node_modules\/svelte\/.*\.mjs$/,
        resolve: {
          fullySpecified: false
        }
      }
    ]
  },
  resolve: {
    alias: {
      svelte: path.resolve("node_modules", "svelte")
    },
    mainFields: ["svelte", "browser", "module", "main"],
    extensions: [".tsx", ".ts", ".mjs", ".js", ".svelte"],
    fallback: {
      // buffer: require.resolve("buffer"),
      // assert: require.resolve("assert"),
      stream: require.resolve("stream-browserify"),
      buffer: false,
      assert: false
    }
  },
  output: {
    filename: "bundle.js",
    path: path.resolve(__dirname, "public", "build")
  }
};
