import * as vscode from 'vscode';
import { SynthClient, getDefaultSocketPath } from './ipcClient';

export function activate(context: vscode.ExtensionContext) {
  const sockPath = getDefaultSocketPath();

  const pingCommand = vscode.commands.registerCommand(
    'synth.ping',
    async () => {
      const client = new SynthClient(sockPath);
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

  context.subscriptions.push(pingCommand);
}

export function deactivate() {}
