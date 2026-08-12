// Bundles the Monaco editor into docs/static/vendor.
//
// Runs inside the image in tools/Dockerfile — see `make editor`. The output is
// checked in, so building the documentation site needs only Go.

import { build } from "esbuild";
import { copyFile, mkdir, rm } from "node:fs/promises";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const OUT = "../docs/static/vendor";

await rm(OUT, { recursive: true, force: true });
await mkdir(OUT, { recursive: true });

const common = {
  bundle: true,
  format: "esm",
  minify: true,
  legalComments: "linked",
  loader: { ".ttf": "file", ".json": "json" },
  logLevel: "info",
};

// The editor itself, plus the CSS Monaco needs.
await build({
  ...common,
  entryPoints: ["src/editor.js"],
  outfile: `${OUT}/editor.js`,
});

// Monaco always wants a worker, even for a language with no language service.
await build({
  ...common,
  entryPoints: [
    require.resolve("monaco-editor/esm/vs/editor/editor.worker.js"),
  ],
  outfile: `${OUT}/editor.worker.js`,
});

// The regex engine the TextMate tokenizer runs on.
await copyFile(
  require.resolve("vscode-oniguruma/release/onig.wasm"),
  `${OUT}/onig.wasm`
);

console.log("editor: wrote", OUT);
