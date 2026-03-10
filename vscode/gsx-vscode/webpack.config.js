// @ts-check

"use strict";

const path = require("path");

/** @type {import('webpack').Configuration} */
const config = {
  target: "node",
  entry: "./src/main.ts",
  output: {
    path: path.resolve(__dirname, "dist"),
    filename: "main.js",
    libraryTarget: "commonjs2",
  },
  devtool: "source-map",
  externals: {
    vscode: "commonjs vscode",
  },
  resolve: {
    extensions: [".ts", ".js"],
  },
  module: {
    rules: [{ test: /\.ts$/, use: "ts-loader" }],
  },
};

module.exports = config;
