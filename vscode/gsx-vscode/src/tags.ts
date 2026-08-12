import * as vscode from "vscode";

const TAG_NAME = "[A-Za-z_][A-Za-z0-9_.-]*";

/**
 * Void elements never take a closing tag, so typing `>` after one must not
 * insert `</img>`. This is the HTML void set; GSX still requires `/>` on them,
 * but a user mid-edit has not typed the slash yet.
 */
const VOID_ELEMENTS = new Set([
  "area", "base", "br", "col", "embed", "hr", "img", "input",
  "link", "meta", "param", "source", "track", "wbr",
]);

/**
 * Close a tag when the user types `>`, and close a fragment on `<>`.
 *
 * This is the single highest-value editor behaviour for a JSX-like language,
 * and it cannot come from the language server: it has to react to a keystroke
 * before the buffer is in a parseable state.
 */
export function registerAutoCloseTag(ctx: vscode.ExtensionContext) {
  ctx.subscriptions.push(
    vscode.workspace.onDidChangeTextDocument(async (event) => {
      if (event.document.languageId !== "gsx") return;
      if (!vscode.workspace.getConfiguration("gsx").get<boolean>("autoCloseTags", true)) return;

      // Only react to a single typed ">" — not to undo, formatting, or a paste.
      if (event.contentChanges.length !== 1) return;
      const change = event.contentChanges[0];
      if (change.text !== ">" || change.rangeLength !== 0) return;

      const editor = vscode.window.activeTextEditor;
      if (!editor || editor.document !== event.document) return;

      const closeAt = change.range.start.translate(0, 1);
      const before = event.document.getText(
        new vscode.Range(new vscode.Position(closeAt.line, 0), closeAt),
      );

      const insert = closingTextFor(before);
      if (!insert) return;

      await editor.edit(
        (edit) => edit.insert(closeAt, insert),
        { undoStopBefore: false, undoStopAfter: false },
      );
      // Leave the caret between the tags rather than after the inserted text.
      editor.selection = new vscode.Selection(closeAt, closeAt);
    }),
  );
}

/**
 * The text to insert after a just-typed `>`, or null to insert nothing.
 *
 * Exported for testing: this is pure string logic, so it can be checked without
 * a running editor.
 */
export function closingTextFor(lineUpToCaret: string): string | null {
  // Fragment: `<>` closes with `</>`.
  if (/<>$/.test(lineUpToCaret)) return "</>";

  // Already a closing tag, or self-closing: nothing to do.
  if (/<\/[^<>]*>$/.test(lineUpToCaret)) return null;
  if (/\/>$/.test(lineUpToCaret)) return null;

  const open = lineUpToCaret.match(new RegExp(`<(${TAG_NAME})(?:\\s[^<>]*)?>$`));
  if (!open) return null;

  const name = open[1];
  if (VOID_ELEMENTS.has(name.toLowerCase())) return null;

  return `</${name}>`;
}

/**
 * Keep an opening and closing tag name in sync while either is edited, the way
 * VS Code does for HTML.
 */
export function registerLinkedEditing(ctx: vscode.ExtensionContext) {
  ctx.subscriptions.push(
    vscode.languages.registerLinkedEditingRangeProvider(
      { language: "gsx", scheme: "file" },
      {
        provideLinkedEditingRanges(document, position) {
          const found = tagPairAt(document, position);
          if (!found) return undefined;
          return new vscode.LinkedEditingRanges(found, new RegExp(TAG_NAME));
        },
      },
    ),
  );
}

/**
 * Find the opening/closing name ranges for the tag whose name contains
 * position.
 *
 * The scan is deliberately simple — a depth counter over same-named tags in the
 * document — because it has to work on a buffer that is mid-edit and therefore
 * often unparseable.
 */
function tagPairAt(
  document: vscode.TextDocument,
  position: vscode.Position,
): vscode.Range[] | undefined {
  const text = document.getText();
  const offset = document.offsetAt(position);

  const tagRe = new RegExp(`<(/?)(${TAG_NAME})`, "g");
  type Tag = { closing: boolean; name: string; start: number; end: number; selfClosing: boolean };

  const tags: Tag[] = [];
  for (let m = tagRe.exec(text); m !== null; m = tagRe.exec(text)) {
    const nameStart = m.index + 1 + m[1].length;
    const nameEnd = nameStart + m[2].length;

    // Determine whether this tag self-closes, by finding its `>`.
    const gt = text.indexOf(">", nameEnd);
    const selfClosing = gt > 0 && text[gt - 1] === "/";

    tags.push({ closing: m[1] === "/", name: m[2], start: nameStart, end: nameEnd, selfClosing });
  }

  const index = tags.findIndex((t) => offset >= t.start && offset <= t.end);
  if (index < 0) return undefined;

  const target = tags[index];
  if (target.selfClosing) return undefined;

  const range = (t: Tag) =>
    new vscode.Range(document.positionAt(t.start), document.positionAt(t.end));

  if (!target.closing) {
    let depth = 0;
    for (let i = index + 1; i < tags.length; i++) {
      const t = tags[i];
      if (t.name !== target.name || t.selfClosing) continue;
      if (!t.closing) {
        depth++;
        continue;
      }
      if (depth === 0) return [range(target), range(t)];
      depth--;
    }
    return undefined;
  }

  let depth = 0;
  for (let i = index - 1; i >= 0; i--) {
    const t = tags[i];
    if (t.name !== target.name || t.selfClosing) continue;
    if (t.closing) {
      depth++;
      continue;
    }
    if (depth === 0) return [range(t), range(target)];
    depth--;
  }
  return undefined;
}
