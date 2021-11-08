"use strict";

const path = require("path");

module.exports = {
  entry: "./src/main.ts",

  devtool: "inline-source-map",
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        use: "ts-loader",
        exclude: /node_modules/
      }
    ]
  },
  resolve: {
    extensions: [".tsx", ".ts", ".js"],
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
    path: path.resolve(__dirname, "dist")
  }
};
