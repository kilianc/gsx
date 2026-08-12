// Tokenize fixtures with the same TextMate engine VS Code uses, and check each
// line against the scopes it is expected to carry.
//
// A TextMate grammar cannot be verified by reading it — the interaction between
// begin/end regions, backreferences and injections only shows up when something
// actually runs it. This harness runs inside the tools image:
//
//   make grammar-test
//
// Fixtures live in testdata/*.gsx — `testdata` so the GSX compiler skips them;
// they are highlighting samples, not programs meant to compile. Expectations are written as comments in
// the fixture itself:
//
//   <div class="x">hi</div>
//   //= div -> entity.name.tag
//   //= class -> entity.other.attribute-name
//
// Each `//=` line asserts that the first occurrence of the quoted text on the
// preceding source line carries a scope containing the given suffix.

import { readFileSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
// ESM does not consult NODE_PATH, so the engine is imported from where the
// tools image installed it.
const MODULES = process.env.GSX_NODE_MODULES ?? "/opt/grammar/node_modules";
// Both are CommonJS, so a dynamic import puts their exports under .default.
const onigMod = await import(`${MODULES}/vscode-oniguruma/release/main.js`);
const tmMod = await import(`${MODULES}/vscode-textmate/release/main.js`);
const oniguruma = onigMod.default ?? onigMod;
const textmate = tmMod.default ?? tmMod;

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, "..");

const wasm = readFileSync(join(MODULES, "vscode-oniguruma/release/onig.wasm"));

await oniguruma.loadWASM(wasm.buffer);

const registry = new textmate.Registry({
  onigLib: Promise.resolve({
    createOnigScanner: (sources) => new oniguruma.OnigScanner(sources),
    createOnigString: (s) => new oniguruma.OnigString(s),
  }),
  loadGrammar: async (scopeName) => {
    if (scopeName === "source.gsx") {
      const raw = readFileSync(join(root, "syntaxes/gsx.tmLanguage.json"), "utf8");
      return textmate.parseRawGrammar(raw, "gsx.tmLanguage.json");
    }
    // source.go is provided by the built-in Go grammar at runtime. Tests only
    // assert on GSX's own scopes, so an empty stand-in is enough here.
    return textmate.parseRawGrammar(
      JSON.stringify({ scopeName, patterns: [] }),
      "stub.json",
    );
  },
});

const grammar = await registry.loadGrammar("source.gsx");

/** Tokenize a whole document, returning one array of {text, scopes} per line. */
function tokenizeDocument(text) {
  let ruleStack = textmate.INITIAL;
  return text.split("\n").map((line) => {
    const result = grammar.tokenizeLine(line, ruleStack);
    ruleStack = result.ruleStack;
    return result.tokens.map((t) => ({
      text: line.substring(t.startIndex, t.endIndex),
      start: t.startIndex,
      end: t.endIndex,
      scopes: t.scopes,
    }));
  });
}

/**
 * Every scope carried by any token overlapping the first occurrence of needle.
 *
 * A construct like `<div` is several tokens — the punctuation and the name are
 * scoped separately — so an assertion has to consider the whole span.
 */
function scopesFor(lineTokens, line, needle) {
  const at = line.indexOf(needle);
  if (at < 0) return null;
  const until = at + needle.length;

  const scopes = new Set();
  let matched = false;
  for (const token of lineTokens) {
    if (token.start < until && token.end > at) {
      matched = true;
      token.scopes.forEach((s) => scopes.add(s));
    }
  }
  return matched ? [...scopes] : null;
}

let failures = 0;
let checks = 0;

const fixturesDir = join(root, "testdata");
for (const name of readdirSync(fixturesDir).sort()) {
  if (!name.endsWith(".gsx")) continue;

  const path = join(fixturesDir, name);
  const source = readFileSync(path, "utf8");
  const lines = source.split("\n");
  const tokens = tokenizeDocument(source);

  lines.forEach((line, i) => {
    const m = line.match(/^\s*\/\/=\s*(!?)(.+?)\s*->\s*(\S+)\s*$/);
    if (!m) return;

    const [, negated, needle, wantScope] = m;

    // Assertions attach to the nearest preceding non-assertion line.
    let target = i - 1;
    while (target >= 0 && /^\s*\/\/=/.test(lines[target])) target--;
    if (target < 0) return;

    checks++;
    const scopes = scopesFor(tokens[target], lines[target], needle);
    const has = scopes !== null && scopes.some((s) => s.includes(wantScope));

    if (negated ? has : !has) {
      failures++;
      console.error(
        `${name}:${target + 1}: ${JSON.stringify(needle)} ` +
          (negated ? `must NOT have scope ${wantScope}` : `expected scope ${wantScope}`) +
          `\n  line:   ${lines[target]}` +
          `\n  scopes: ${scopes ? scopes.join(" ") : "<no token matched>"}`,
      );
    }
  });
}

if (failures > 0) {
  console.error(`\n${failures} of ${checks} grammar assertions failed`);
  process.exit(1);
}
console.log(`grammar: ${checks} assertions passed`);
