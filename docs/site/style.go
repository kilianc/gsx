package site

// stylesheet is inlined into every page. The site is a handful of static files
// with no build step and no external requests, so a single inline stylesheet is
// both the simplest and the fastest option.
const stylesheet = `
:root {
  --bg: #fbfbfa;
  --bg-raised: #ffffff;
  --bg-code: #f6f6f4;
  --fg: #1a1a1c;
  --fg-muted: #6b6b73;
  --fg-faint: #92929c;
  --border: #e6e6e2;
  --accent: #6d4aff;
  --accent-soft: #f0ecff;

  --hl-comment: #8a8a94;
  --hl-string: #1a7f5a;
  --hl-number: #b06a00;
  --hl-keyword: #a03aa8;
  --hl-type: #0f6fbd;
  --hl-tag: #c0392b;
  --hl-attr: #b06a00;
  --hl-brace: #6d4aff;
  --hl-punct: #92929c;

  --radius: 10px;
  --mono: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
  --sans: -apple-system, BlinkMacSystemFont, "Segoe UI", Inter, Roboto, sans-serif;
}

@media (prefers-color-scheme: dark) {
  :root {
    --bg: #101012;
    --bg-raised: #161619;
    --bg-code: #1a1a1f;
    --fg: #ececf0;
    --fg-muted: #a0a0ac;
    --fg-faint: #75757f;
    --border: #26262c;
    --accent: #a78bfa;
    --accent-soft: #241f3a;

    --hl-comment: #6f6f7b;
    --hl-string: #6ee7a8;
    --hl-number: #f0b866;
    --hl-keyword: #e08ce8;
    --hl-type: #6cc2ff;
    --hl-tag: #ff8f80;
    --hl-attr: #f0b866;
    --hl-brace: #a78bfa;
    --hl-punct: #75757f;
  }
}

* { box-sizing: border-box; }

html { -webkit-text-size-adjust: 100%; }

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

.brand { color: var(--fg); font-weight: 650; letter-spacing: -0.02em; }
.brand:hover { text-decoration: none; }
.brand-mark { font-family: var(--mono); color: var(--accent); font-size: 15px; }

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

@media (max-width: 900px) {
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
