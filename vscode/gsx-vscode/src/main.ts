import * as vscode from "vscode";
import fs from "fs/promises";
import path from "path";
import { lookpath } from "lookpath";
import { LanguageClient } from "vscode-languageclient/node";
import { CustomLanguageClient } from "./custom-client";

export async function activate(ctx: vscode.ExtensionContext) {
  try {
    ctx.subscriptions.push(
      vscode.commands.registerCommand("gsx.restartServer", startLanguageClient),
    );
    await startLanguageClient();
  } catch (err) {
    const msg = err && (err as Error) ? (err as Error).message : "unknown";
    vscode.window.showErrorMessage(`error initializing gsx LSP: ${msg}`);
  }
}

interface Configuration {
  goplsLog: string;
  goplsRPCTrace: boolean;
  goplsRemote: string;
  executablePath: string;
}

interface GSXCtx {
  languageClient?: LanguageClient;
  output?: vscode.OutputChannel;
}

const gsxCtx: GSXCtx = {};

const loadConfiguration = (): Configuration => {
  const c = vscode.workspace.getConfiguration("gsx");
  return {
    goplsLog: c.get("goplsLog") || "",
    goplsRPCTrace: c.get("goplsRPCTrace") ? true : false,
    goplsRemote: c.get("goplsRemote") || "",
    executablePath: c.get("executablePath") || "",
  };
};

const gsxLocations = [
  path.join(process.env.GOBIN ?? "", "gsx"),
  path.join(process.env.GOBIN ?? "", "gsx.exe"),
  path.join(process.env.GOPATH ?? "", "bin", "gsx"),
  path.join(process.env.GOPATH ?? "", "bin", "gsx.exe"),
  path.join(process.env.GOROOT || "", "bin", "gsx"),
  path.join(process.env.GOROOT || "", "bin", "gsx.exe"),
  path.join(process.env.HOME || "", "bin", "gsx"),
  path.join(process.env.HOME || "", "bin", "gsx.exe"),
  path.join(process.env.HOME || "", "go", "bin", "gsx"),
  path.join(process.env.HOME || "", "go", "bin", "gsx.exe"),
  "/usr/local/bin/gsx",
  "/usr/bin/gsx",
  "/usr/local/go/bin/gsx",
];

async function findGSX(): Promise<string> {
  const config = loadConfiguration();
  if (config.executablePath) {
    return config.executablePath;
  }

  // Prefer workspace-local ./bin/gsx (avoids macOS ghostscript 'gsx' name collision).
  const ws = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  if (ws) {
    const local = path.join(ws, "bin", process.platform === "win32" ? "gsx.exe" : "gsx");
    try {
      await fs.stat(local);
      return local;
    } catch {
      // ignore
    }
  }

  // Prefer common Go install locations before PATH.
  for (const exe of gsxLocations) {
    try {
      await fs.stat(exe);
      return exe;
    } catch {
      // ignore
    }
  }

  const p = await lookpath("gsx");
  if (p) {
    return p;
  }
  const pw = await lookpath("gsx.exe");
  if (pw) {
    return pw;
  }
  for (const exe of gsxLocations) {
    try {
      await fs.stat(exe);
      return exe;
    } catch {
      // ignore
    }
  }
  throw new Error(`Could not find gsx executable in PATH or common locations`);
}

async function stopLanguageClient() {
  const c = gsxCtx.languageClient;
  gsxCtx.languageClient = undefined;
  if (!c) return;
  if (c.diagnostics) {
    c.diagnostics.clear();
  }
  try {
    c.stop(2000);
  } catch (e) {
    c.outputChannel?.appendLine(`Failed to stop client: ${e}`);
  }
}

async function startLanguageClient() {
  gsxCtx.languageClient = await buildLanguageClient();
  await gsxCtx.languageClient.start();
}

async function buildLanguageClient(): Promise<LanguageClient> {
  const documentSelector = [{ language: "gsx", scheme: "file" }];
  const config = loadConfiguration();

  const args: Array<string> = ["lsp"];
  if (config.goplsLog.length > 0) {
    args.push(`-goplsLog=${config.goplsLog}`);
  }
  if (config.goplsRPCTrace) {
    args.push(`-goplsRPCTrace=true`);
  }
  if (config.goplsRemote.length > 0) {
    args.push(`-goplsRemote=${config.goplsRemote}`);
  }

  const gsxPath = await findGSX();
  if (gsxCtx.languageClient) {
    await stopLanguageClient();
  }

  if (!gsxCtx.output) {
    gsxCtx.output = vscode.window.createOutputChannel("gsx");
  }
  gsxCtx.output.appendLine(`Starting GSX LSP: ${gsxPath} ${args.join(" ")}`);
  vscode.window.setStatusBarMessage(`Starting GSX LSP: ${gsxPath} ${args.join(" ")}`, 3000);

  return new CustomLanguageClient(
    "gsx",
    "gsx",
    {
      command: gsxPath,
      args,
      options: { env: { ...process.env } },
    },
    {
      documentSelector,
      outputChannel: gsxCtx.output,
      uriConverters: {
        code2Protocol: (uri: vscode.Uri): string => (uri.scheme ? uri : uri.with({ scheme: "file" })).toString(),
        protocol2Code: (uri: string) => vscode.Uri.parse(uri),
      },
    },
    false,
  );
}
