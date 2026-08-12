// Monaco wired to the GSX TextMate grammar.
//
// The grammar and language configuration are imported from the VS Code
// extension rather than copied, so the browser highlights .gsx exactly the way
// the editor does and there is only ever one grammar to maintain.
//
// The grammar delegates to source.go for the Go inside a file, so the Go
// grammar VS Code itself uses is registered alongside it — without that,
// everything outside a tag would render unstyled.

import * as monaco from "monaco-editor/esm/vs/editor/editor.api.js";
import { INITIAL, Registry, parseRawGrammar } from "vscode-textmate";
import { loadWASM, createOnigScanner, createOnigString } from "vscode-oniguruma";

import gsxGrammar from "../../vscode/gsx-vscode/syntaxes/gsx.tmLanguage.json";
import gsxLanguageConfig from "../../vscode/gsx-vscode/language-configuration.json";
import goGrammarBundle from "shiki/langs/go.mjs";

const LANG = "gsx";
const SCOPE = "source.gsx";

// Monaco resolves its worker through this global, and the regex engine needs
// its wasm. Both sit beside this file, so they are resolved relative to it
// rather than to the page — the vendor directory can move as a unit.
self.MonacoEnvironment = {
  getWorker: () =>
    new Worker(new URL("./editor.worker.js", import.meta.url), { type: "module" }),
};

const ONIG_WASM_URL = new URL("./onig.wasm", import.meta.url);

// shiki ships each language as an array of grammars; source.go is the one the
// GSX grammar includes.
const goGrammar = goGrammarBundle.find((g) => g.scopeName === "source.go") ?? goGrammarBundle[0];

// TextMate scope prefixes mapped to the palette the rest of the site uses, so
// a snippet in the playground matches a snippet on a documentation page.
function themeRules(c) {
  return [
    { token: "comment", foreground: c.comment },
    { token: "string", foreground: c.string },
    { token: "constant.numeric", foreground: c.number },
    { token: "constant.character.escape", foreground: c.number },
    { token: "keyword", foreground: c.keyword },
    { token: "storage", foreground: c.keyword },
    { token: "entity.name.type", foreground: c.type },
    { token: "entity.name.function", foreground: c.type },
    { token: "support.type", foreground: c.type },
    { token: "support.function", foreground: c.type },
    { token: "entity.name.tag", foreground: c.tag },
    { token: "entity.other.attribute-name", foreground: c.attr },
    { token: "punctuation.section.embedded", foreground: c.brace },
    { token: "punctuation.definition.tag", foreground: c.tag },
    { token: "punctuation", foreground: c.punct },
    { token: "variable", foreground: c.fg },
  ];
}

// The documentation site is light whatever the operating system prefers, so the
// editor embedded in it is too — the same palette as the stylesheet, so a
// snippet keeps its colours when it moves between a page and the playground.
const LIGHT = {
  comment: "8a8a94", string: "1a7f5a", number: "b06a00", keyword: "a03aa8",
  type: "1d54c4", tag: "c0392b", attr: "b06a00", brace: "007d9c",
  punct: "92929c", fg: "1a1a1c", bg: "ffffff",
};

// Bracket highlighting paints brackets by nesting depth from its own palette,
// over whatever the grammar said. Turning it off in the editor options is not
// enough on its own, so the palette is also collapsed to the one colour that
// marks the Go/markup boundary: whichever path is live, a `{...}` splice comes
// out the same, and a stray bracket still stands out.
function bracketColors(c) {
  const colors = { "editorBracketHighlight.unexpectedBracket.foreground": "#" + c.tag };
  for (let i = 1; i <= 6; i++) {
    colors["editorBracketHighlight.foreground" + i] = "#" + c.brace;
  }
  return colors;
}

function defineTheme() {
  monaco.editor.defineTheme("gsx-light", {
    base: "vs",
    inherit: true,
    rules: themeRules(LIGHT),
    colors: {
      "editor.background": "#" + LIGHT.bg,
      "editor.foreground": "#" + LIGHT.fg,
      ...bracketColors(LIGHT),
    },
  });
}

// Monaco's language configuration uses the same shape as the extension's JSON
// except that pairs are tuples, so the two bracket lists are converted.
function toMonacoConfig(cfg) {
  return {
    comments: cfg.comments,
    brackets: cfg.brackets,
    autoClosingPairs: cfg.autoClosingPairs,
    surroundingPairs: cfg.surroundingPairs?.map((p) => ({ open: p.open, close: p.close })),
  };
}

// The GSX grammar lists #tag in its top-level patterns, which only reaches text
// at the top level of a file. Once the Go grammar opens a region — a function
// body, a parenthesised expression — its own patterns take over, and every tag
// a reader actually writes lives inside one. Injecting the tag patterns across
// source.gsx makes them reachable at any depth.
//
// This does not show up when source.go is stubbed out, because with no Go
// patterns nothing ever opens a region to hide the tags.
const TAG_INJECTION = {
  scopeName: "gsx.tags.injection",
  injectionSelector: "L:source.gsx -comment -string",
  patterns: [{ include: "source.gsx#tag" }],
};

let registry = null;

async function initGrammars(onigWasmUrl) {
  const res = await fetch(onigWasmUrl);
  await loadWASM(await res.arrayBuffer());

  registry = new Registry({
    onigLib: Promise.resolve({ createOnigScanner, createOnigString }),
    getInjections: (scopeName) =>
      scopeName === SCOPE ? [TAG_INJECTION.scopeName] : undefined,
    loadGrammar: async (scopeName) => {
      if (scopeName === SCOPE) {
        return parseRawGrammar(JSON.stringify(gsxGrammar), "gsx.tmLanguage.json");
      }
      if (scopeName === "source.go") {
        return parseRawGrammar(JSON.stringify(goGrammar), "go.tmLanguage.json");
      }
      if (scopeName === TAG_INJECTION.scopeName) {
        return parseRawGrammar(JSON.stringify(TAG_INJECTION), "gsx.injection.json");
      }
      return null;
    },
  });

  const grammar = await registry.loadGrammar(SCOPE);
  if (!grammar) throw new Error("could not load " + SCOPE);

  // Monaco's IToken carries a single scope string, so the most specific scope
  // wins and the theme rules above match it by prefix.
  monaco.languages.setTokensProvider(LANG, {
    getInitialState: () => INITIAL,
    tokenize: (line, state) => {
      const r = grammar.tokenizeLine(line, state);
      return {
        tokens: r.tokens.map((t) => ({
          startIndex: t.startIndex,
          scopes: t.scopes[t.scopes.length - 1] ?? "",
        })),
        endState: r.ruleStack,
      };
    },
  });
}

const SEVERITY = {
  error: monaco.MarkerSeverity.Error,
  warning: monaco.MarkerSeverity.Warning,
  info: monaco.MarkerSeverity.Info,
};

export async function create(container, opts = {}) {
  monaco.languages.register({ id: LANG, extensions: [".gsx"] });
  monaco.languages.setLanguageConfiguration(LANG, toMonacoConfig(gsxLanguageConfig));
  defineTheme();

  await initGrammars(opts.onigWasmUrl ?? ONIG_WASM_URL);

  const editor = monaco.editor.create(container, {
    value: opts.value ?? "",
    language: LANG,
    theme: "gsx-light",
    automaticLayout: true,
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    fontSize: 13.5,
    lineHeight: 22,
    tabSize: 2,
    insertSpaces: false,
    renderLineHighlight: "none",
    // Bracket pair colouring overrides the theme, which would paint the braces
    // of a `{...}` splice by nesting depth. In GSX those braces are the
    // Go/markup boundary, so they should keep the one colour that marks it.
    bracketPairColorization: { enabled: false },
    padding: { top: 12, bottom: 12 },
    fontFamily:
      'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
  });

  if (opts.onChange) {
    editor.onDidChangeModelContent(() => opts.onChange(editor.getValue()));
  }

  return {
    getValue: () => editor.getValue(),
    setValue: (v) => editor.setValue(v),
    focus: () => editor.focus(),

    // markers: [{line, col, endLine?, endCol?, message, severity?}], 1-based,
    // already mapped back to .gsx coordinates by the Go side.
    setMarkers(markers) {
      monaco.editor.setModelMarkers(
        editor.getModel(),
        "gsx",
        (markers ?? []).map((m) => ({
          startLineNumber: m.line,
          startColumn: m.col,
          endLineNumber: m.endLine ?? m.line,
          endColumn: m.endCol ?? m.col + 1,
          message: m.message,
          severity: SEVERITY[m.severity] ?? monaco.MarkerSeverity.Error,
        }))
      );
    },
  };
}

window.GSXEditor = { create };
