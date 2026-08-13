package site

// stylesheet is inlined into every page. The site is a handful of static files
// with no build step and no external requests, so a single inline stylesheet is
// both the simplest and the fastest option.
//
// The palette is light in every environment. The site is mostly code samples
// whose highlighting has to match the prose around it and the playground editor
// beside it, and one palette is one thing to keep in tune.
const stylesheet = `
:root {
  --bg: #fbfbfa;
  --bg-raised: #ffffff;
  --bg-code: #f6f6f4;
  --fg: #1a1a1c;
  --fg-muted: #6b6b73;
  --fg-faint: #92929c;
  --border: #e6e6e2;
  /* Go's Gopher Blue. The brand colour itself, #00add8, carries only 2.4:1
     against this background, so text and fills use the darker brand shade —
     the same swap go.dev makes — and #00add8 is kept for dark backgrounds,
     which here means only the logo variants in assets/. */
  --accent: #007d9c;
  --accent-soft: #e9f6fa;

  --hl-comment: #8a8a94;
  --hl-string: #1a7f5a;
  --hl-number: #b06a00;
  --hl-keyword: #a03aa8;
  --hl-type: #1d54c4;
  --hl-tag: #c0392b;
  --hl-attr: #b06a00;
  --hl-brace: #007d9c;
  --hl-punct: #92929c;

  --radius: 10px;
  --mono: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
  --sans: -apple-system, BlinkMacSystemFont, "Segoe UI", Inter, Roboto, sans-serif;
}

* { box-sizing: border-box; }

/* Form controls and scrollbars follow this, so they stay light under a dark OS. */
html { -webkit-text-size-adjust: 100%; color-scheme: light; }

body {
  margin: 0;
  background: var(--bg);
  color: var(--fg);
  font-family: var(--sans);
  font-size: 16px;
  line-height: 1.65;
  -webkit-font-smoothing: antialiased;
}

a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }

.skip {
  position: absolute;
  left: -9999px;
}
.skip:focus {
  left: 12px;
  top: 12px;
  z-index: 10;
  background: var(--bg-raised);
  padding: 8px 14px;
  border-radius: var(--radius);
  border: 1px solid var(--border);
}

/* Header ------------------------------------------------------------------ */

.topbar {
  position: sticky;
  top: 0;
  z-index: 5;
  display: flex;
  align-items: center;
  gap: 28px;
  padding: 14px 40px;
  background: color-mix(in srgb, var(--bg) 86%, transparent);
  backdrop-filter: saturate(1.6) blur(10px);
  border-bottom: 1px solid var(--border);
}

.brand { display: flex; color: var(--fg); }
.brand:hover { text-decoration: none; }
.brand-logo { display: block; overflow: visible; }
.brand-braces { stroke: var(--accent); }

.topnav { display: flex; gap: 20px; flex-wrap: wrap; font-size: 14.5px; }
.topnav a { color: var(--fg-muted); }
.topnav a:hover { color: var(--fg); text-decoration: none; }
.topnav .ext::after { content: " ↗"; font-size: 11px; color: var(--fg-faint); }

/* Shell ------------------------------------------------------------------- */

.shell {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 56px;
  padding: 40px 40px 80px;
}

/*
 * The page runs full width so code — especially the side-by-side source and
 * generated output — gets all the room it can use. Prose does not: a line of
 * text spanning a wide monitor is unreadable, so paragraphs and lists keep a
 * comfortable measure while tables, code and the split panes expand.
 */
main > .section > p,
main > .section > ul,
main > .section > ol,
main > .section > .note,
.page-head {
  max-width: 78ch;
}

.sidebar { position: sticky; top: 76px; align-self: start; }
.side-title {
  margin: 0 0 12px;
  font-size: 11px;
  letter-spacing: 0.09em;
  text-transform: uppercase;
  color: var(--fg-faint);
  font-weight: 600;
}
.sidebar ul { list-style: none; margin: 0; padding: 0; }
.sidebar li { margin: 1px 0; }
.side-link {
  display: block;
  padding: 6px 12px;
  margin-left: -12px;
  border-radius: 8px;
  color: var(--fg-muted);
  font-size: 14.5px;
}
.side-link:hover { background: var(--bg-raised); color: var(--fg); text-decoration: none; }
.side-link.is-current { background: var(--accent-soft); color: var(--accent); font-weight: 550; }

/* Content ----------------------------------------------------------------- */

main { min-width: 0; }

.page-head { margin-bottom: 40px; }
.page-head h1 {
  margin: 0 0 10px;
  font-size: 40px;
  line-height: 1.12;
  letter-spacing: -0.032em;
  font-weight: 680;
}
.lede { margin: 0; font-size: 18.5px; color: var(--fg-muted); line-height: 1.55; }

.section { margin: 44px 0; scroll-margin-top: 84px; }
.section h2 {
  margin: 0 0 14px;
  font-size: 23px;
  letter-spacing: -0.02em;
  font-weight: 640;
}
.section h3 { margin: 30px 0 10px; font-size: 17px; font-weight: 620; }
.anchor { color: inherit; }
.anchor:hover { text-decoration: none; }
.anchor:hover::after {
  content: " #";
  color: var(--fg-faint);
  font-weight: 400;
}

p { margin: 0 0 16px; }
ul, ol { margin: 0 0 16px; padding-left: 22px; }
li { margin: 6px 0; }

strong { font-weight: 640; }

/* Code -------------------------------------------------------------------- */

code {
  font-family: var(--mono);
  font-size: 0.885em;
}

p code, li code, td code {
  background: var(--bg-code);
  border: 1px solid var(--border);
  border-radius: 5px;
  padding: 1px 5px;
}

.code {
  margin: 0 0 18px;
  background: var(--bg-code);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  overflow: hidden;
}
.code pre {
  margin: 0;
  tab-size: 2;
  -moz-tab-size: 2;
  padding: 15px 17px;
  overflow-x: auto;
  line-height: 1.6;
}
.code code { font-size: 13.5px; }

.code-out { background: transparent; border-style: dashed; }
.code-out pre { color: var(--fg-muted); }
.hl-out-label {
  display: block;
  margin-bottom: 8px;
  font-size: 10.5px;
  letter-spacing: 0.09em;
  text-transform: uppercase;
  color: var(--fg-faint);
  font-weight: 600;
}

.code-sh pre { color: var(--fg); }
.code-sh pre::before { content: "$ "; color: var(--fg-faint); }

.hl-c  { color: var(--hl-comment); font-style: italic; }
.hl-s  { color: var(--hl-string); }
.hl-n  { color: var(--hl-number); }
.hl-k  { color: var(--hl-keyword); }
.hl-t  { color: var(--hl-type); }
.hl-tg { color: var(--hl-tag); }
.hl-at { color: var(--hl-attr); }
.hl-br { color: var(--hl-brace); font-weight: 600; }
.hl-p  { color: var(--hl-punct); }

/* Layout helpers ---------------------------------------------------------- */

.split { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 420px), 1fr)); gap: 16px; }
.split > div { min-width: 0; }

.note {
  margin: 0 0 18px;
  padding: 14px 16px;
  background: var(--accent-soft);
  border-left: 2.5px solid var(--accent);
  border-radius: 0 var(--radius) var(--radius) 0;
}
.note p:last-child { margin-bottom: 0; }

table {
  width: 100%;
  border-collapse: collapse;
  margin: 0 0 18px;
  font-size: 14.5px;
  display: block;
  overflow-x: auto;
}
th, td {
  text-align: left;
  padding: 9px 12px;
  border-bottom: 1px solid var(--border);
  vertical-align: top;
}
th { font-weight: 620; color: var(--fg-muted); font-size: 12.5px; letter-spacing: 0.03em; text-transform: uppercase; }

.footer {
  margin-top: 72px;
  padding-top: 24px;
  border-top: 1px solid var(--border);
  font-size: 14px;
  color: var(--fg-muted);
}
.muted { color: var(--fg-faint); }

/* Landing ----------------------------------------------------------------- */

.hero-cta { display: flex; gap: 12px; flex-wrap: wrap; margin: 4px 0 32px; }
.btn {
  display: inline-block;
  padding: 9px 18px;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  background: var(--bg-raised);
  color: var(--fg);
  font-size: 14.5px;
  font-weight: 550;
}
.btn:hover { text-decoration: none; border-color: var(--accent); }
.btn-primary { background: var(--accent); border-color: var(--accent); color: #fff; }
.btn-primary:hover { filter: brightness(1.08); }

.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 14px; margin-bottom: 8px; }
.card {
  padding: 18px;
  background: var(--bg-raised);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}
.card h3 { margin: 0 0 6px; font-size: 15.5px; font-weight: 620; }
.card p { margin: 0; font-size: 14.5px; color: var(--fg-muted); }

/* Releases ---------------------------------------------------------------- */

.release-index { display: flex; flex-wrap: wrap; gap: 8px; margin: -22px 0 8px; }
.release-pill {
  padding: 3px 12px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--bg-raised);
  font: 550 13px/1.7 var(--mono);
  color: var(--fg-muted);
}
.release-pill:hover { border-color: var(--accent); color: var(--accent); text-decoration: none; }

/* Each release is a heading with a date and a status beside it rather than
   above it, so the version stays the thing you scan for. */
.release-head { display: flex; align-items: baseline; flex-wrap: wrap; gap: 10px; margin-bottom: 12px; }
.release-head h2 { margin: 0; font-family: var(--mono); font-size: 21px; letter-spacing: -0.01em; }
.release-date { font-size: 13.5px; color: var(--fg-faint); }
a.release-date { color: var(--fg-muted); }

.release-badge {
  padding: 2px 9px;
  border-radius: 999px;
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 10.5px;
  font-weight: 660;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
.release-badge-open { background: var(--bg-code); color: var(--fg-muted); }

.release + .release { border-top: 1px solid var(--border); padding-top: 34px; }
.release .release-summary { margin-bottom: 22px; font-size: 17px; color: var(--fg-muted); }
.release h3 { margin-top: 26px; }

.release-ref { margin-left: 7px; font: 500 13px/1 var(--mono); color: var(--fg-faint); }
.release-ref:hover { color: var(--accent); }

.release-links { display: flex; flex-wrap: wrap; gap: 18px; margin-top: 22px; font-size: 14px; }
.release-links .ext::after { content: " ↗"; font-size: 11px; color: var(--fg-faint); }

/* Playground
   ------------------------------------------------------------------ */

.shell-wide { grid-template-columns: minmax(0, 1fr); max-width: 1500px; }
.shell-wide .page-head { margin-bottom: 18px; }

.pg {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 16px;
  /* Fill what is left of the window so the editor is not a letterbox. */
  height: min(72vh, 780px);
  min-height: 420px;
}

.pg-col {
  display: flex;
  flex-direction: column;
  min-width: 0;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-raised);
  overflow: hidden;
}

.pg-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border);
  background: var(--bg-code);
}
.pg-label { font: 600 12px/1 var(--mono); color: var(--fg-muted); }

.pg-status { margin-left: auto; font: 500 11.5px/1 var(--sans); color: var(--fg-faint); }
.pg-status.is-ok { color: var(--hl-string); }
.pg-status.is-bad { color: var(--hl-tag); }

.pg-tab {
  font: 500 12.5px/1 var(--sans);
  color: var(--fg-muted);
  background: none;
  border: 0;
  border-radius: 6px;
  padding: 5px 10px;
  cursor: pointer;
}
.pg-tab:hover { color: var(--fg); background: var(--bg-raised); }
.pg-tab.is-active { color: var(--accent); background: var(--accent-soft); }

/* Monaco's container. Hidden until the bundle attaches, so the textarea below
   is what shows if it never does. */
.pg-mount { flex: 1; min-height: 0; }
.pg-mount[hidden] { display: none; }

.pg-editor {
  flex: 1;
  width: 100%;
  resize: none;
  border: 0;
  outline: none;
  padding: 14px 16px;
  font: 13.5px/1.65 var(--mono);
  tab-size: 2;
  color: var(--fg);
  background: transparent;
}

.pg-body { flex: 1; min-height: 0; display: flex; }
.pg-body[hidden] { display: none; }

.pg-preview { flex: 1; width: 100%; border: 0; background: #fff; }

.pg-out {
  flex: 1;
  margin: 0;
  overflow: auto;
  padding: 14px 16px;
  font: 13px/1.6 var(--mono);
  white-space: pre;
}

.pg-error {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
  background: color-mix(in srgb, var(--hl-tag) 8%, transparent);
  color: var(--hl-tag);
  font: 12.5px/1.55 var(--mono);
  white-space: pre-wrap;
  max-height: 40%;
  overflow: auto;
}
.pg-error[hidden] { display: none; }

.pg-note { margin-top: 22px; max-width: 70ch; }
.pg-note p { margin: 0 0 8px; font-size: 14.5px; color: var(--fg-muted); }
.pg-note .muted { color: var(--fg-faint); font-size: 13.5px; }

@media (max-width: 900px) {
  .pg { grid-template-columns: 1fr; height: auto; }
  .pg-col { height: 420px; }
  .shell { grid-template-columns: 1fr; gap: 24px; padding: 28px 20px 60px; }
  main > .section > p,
  main > .section > ul,
  main > .section > ol,
  main > .section > .note,
  .page-head { max-width: none; }
  .sidebar { position: static; }
  .sidebar ul { display: flex; flex-wrap: wrap; gap: 4px; }
  .side-link { margin-left: 0; }
  .split { grid-template-columns: 1fr; }
  .page-head h1 { font-size: 31px; }
  .lede { font-size: 17px; }
  .topbar { padding: 12px 20px; gap: 18px; }
}
`
