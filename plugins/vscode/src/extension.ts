import * as vscode from 'vscode';
import { SynthClient, getDefaultSocketPath } from './ipcClient';
import { SynthStatusBar } from './statusBar';

export function activate(context: vscode.ExtensionContext) {
  const sockPath = getDefaultSocketPath();
  const client = new SynthClient(sockPath);

  // Existing ping command stays unchanged
  const pingCommand = vscode.commands.registerCommand(
    'synth.ping',
    async () => {
      try {
        const data = await client.ping();
        vscode.window.showInformationMessage(
          `Synth daemon reachable — pid ${data.pid}, ` +
          `version ${data.version}`
        );
      } catch (err) {
        vscode.window.showErrorMessage(
          `Synth: ${err instanceof Error ? err.message : err}`
        );
      }
    }
  );

  const statusBar = new SynthStatusBar(client);
  statusBar.start();

  const showStatusCommand = vscode.commands.registerCommand(
    'synth.showStatus',
    async () => {
      const status = statusBar.getLastStatus();
      if (!status) {
        vscode.window.showWarningMessage(
          'Synth daemon is not reachable. ' +
          'Run "synth daemon start" in a terminal.'
        );
        return;
      }

      const lines = [
        `Synth daemon · running · pid ${status.pid}`,
        `Uptime: ${status.uptime_seconds}s`,
        `Notes: ${status.notes_count}`,
        `Embedded: ${status.embeddings_count} / ` +
          `${status.notes_count}`,
        `File saves: ${status.file_saves_count}`,
        `Low context: ${status.low_context_count} ` +
          `file(s) need attention`,
      ];

      if (status.low_context_files.length > 0) {
        const items = status.low_context_files.map(
          (f) => f.file
        );
        const picked = await vscode.window.showQuickPick(
          items,
          {
            placeHolder: lines.join('  ·  '),
          }
        );
        if (picked) {
          const doc = await vscode.workspace.findFiles(
            `**/${picked}`,
            '**/node_modules/**',
            1
          );
          if (doc.length > 0) {
            const document =
              await vscode.workspace.openTextDocument(
                doc[0]
              );
            await vscode.window.showTextDocument(document);
          }
        }
      } else {
        vscode.window.showInformationMessage(
          lines.join('  ·  ')
        );
      }
    }
  );

  context.subscriptions.push(
    pingCommand,
    showStatusCommand,
    { dispose: () => statusBar.stop() }
  );
}

export function deactivate() {}
