import * as vscode from "vscode";

/**
 * Commands that run the CLI in a terminal.
 *
 * A terminal rather than a background task on purpose: `gsx dev` is long-lived
 * and its output — rebuild timings, compile errors — is what the user wants to
 * watch, and generation errors are worth reading in full.
 */
export function registerCommands(ctx: vscode.ExtensionContext) {
  ctx.subscriptions.push(
    vscode.commands.registerCommand("gsx.generate", () => runInTerminal("gsx: generate", "gsx ./...")),
    vscode.commands.registerCommand("gsx.generateFile", () => {
      const doc = vscode.window.activeTextEditor?.document;
      if (!doc || doc.languageId !== "gsx") {
        vscode.window.showWarningMessage("gsx: no .gsx file is active");
        return;
      }
      runInTerminal("gsx: generate", `gsx ${quote(doc.uri.fsPath)}`);
    }),
    vscode.commands.registerCommand("gsx.dev", () => runInTerminal("gsx: dev", "gsx dev")),
    vscode.commands.registerCommand("gsx.openGenerated", async () => {
      const doc = vscode.window.activeTextEditor?.document;
      if (!doc || !doc.uri.fsPath.endsWith(".gsx")) {
        vscode.window.showWarningMessage("gsx: no .gsx file is active");
        return;
      }
      const generated = doc.uri.with({ path: `${doc.uri.path}.go` });
      try {
        await vscode.window.showTextDocument(await vscode.workspace.openTextDocument(generated), {
          viewColumn: vscode.ViewColumn.Beside,
        });
      } catch {
        vscode.window.showWarningMessage(
          `gsx: ${generated.path.split("/").pop()} does not exist yet — run GSX: Generate`,
        );
      }
    }),
  );
}

const terminals = new Map<string, vscode.Terminal>();

function runInTerminal(name: string, command: string) {
  let terminal = terminals.get(name);
  if (!terminal || terminal.exitStatus !== undefined) {
    terminal = vscode.window.createTerminal({ name, cwd: workspaceRoot() });
    terminals.set(name, terminal);
  }
  terminal.show(true);
  terminal.sendText(command);
}

function workspaceRoot(): string | undefined {
  return vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
}

function quote(p: string): string {
  return /[\s"']/.test(p) ? `"${p.replace(/"/g, '\\"')}"` : p;
}
