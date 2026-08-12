// Tests for the auto-close-tag logic. It runs on every ">" keystroke, so a
// wrong answer is felt constantly — and it is pure string logic, so it can be
// checked without a running editor.
//
//   make extension-test

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

// The source is TypeScript, so strip the annotations rather than build it.
const here = dirname(fileURLToPath(import.meta.url));
const src = readFileSync(join(here, "../src/tags.ts"), "utf8");

const body = src
  .slice(src.indexOf("const TAG_NAME"))
  .replace(/export function registerAutoCloseTag[\s\S]*?\n}\n/, "")
  .replace(/export function registerLinkedEditing[\s\S]*$/, "")
  .replace(/export function closingTextFor\(lineUpToCaret: string\): string \| null/,
           "function closingTextFor(lineUpToCaret)");

const closingTextFor = new Function(`${body}\nreturn closingTextFor;`)();

test("closes an element", () => {
  assert.equal(closingTextFor("  return <div>"), "</div>");
  assert.equal(closingTextFor("<section>"), "</section>");
  assert.equal(closingTextFor('<div class="card">'), "</div>");
  assert.equal(closingTextFor("<div class={c} id={i}>"), "</div>");
});

test("closes a component, including a dotted one", () => {
  assert.equal(closingTextFor("<Card>"), "</Card>");
  assert.equal(closingTextFor('<Card variant="primary">'), "</Card>");
  assert.equal(closingTextFor("<ui.widgets.Card>"), "</ui.widgets.Card>");
});

test("closes a fragment", () => {
  assert.equal(closingTextFor("<>"), "</>");
  assert.equal(closingTextFor("  return <>"), "</>");
});

test("does not close a void element", () => {
  // GSX wants `<br />`, but the user has not typed the slash yet and inserting
  // `</br>` would be wrong in either language.
  assert.equal(closingTextFor("<br>"), null);
  assert.equal(closingTextFor('<img src="/a.png">'), null);
  assert.equal(closingTextFor("<INPUT>"), null);
});

test("does not close a self-closing tag", () => {
  assert.equal(closingTextFor("<br />"), null);
  assert.equal(closingTextFor('<img src="/a.png"/>'), null);
});

test("does not close a closing tag", () => {
  assert.equal(closingTextFor("</div>"), null);
  assert.equal(closingTextFor("</>"), null);
});

test("ignores Go code that merely ends in >", () => {
  assert.equal(closingTextFor("if a > b"), null);
  assert.equal(closingTextFor("x := a >> 2"), null);
  assert.equal(closingTextFor("for i := 0; i > n;"), null);
  assert.equal(closingTextFor("m := map[string]int>"), null);
});
