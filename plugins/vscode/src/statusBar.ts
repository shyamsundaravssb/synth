import * as vscode from 'vscode';
import { SynthClient } from './ipcClient';
import { StatusData } from './protocol';

const POLL_INTERVAL_MS = 10000; // 10 seconds

export class SynthStatusBar {
  private item: vscode.StatusBarItem;
  private timer: NodeJS.Timeout | undefined;
  private client: SynthClient;
  private lastStatus: StatusData | undefined;

  constructor(client: SynthClient) {
    this.client = client;
    this.item = vscode.window.createStatusBarItem(
      vscode.StatusBarAlignment.Left,
      100
    );
    this.item.command = 'synth.showStatus';
    this.item.show();
    this.setOffline();
  }

  start(): void {
    this.poll();
    this.timer = setInterval(
      () => this.poll(),
      POLL_INTERVAL_MS
    );
  }

  stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = undefined;
    }
    this.item.dispose();
  }

  getLastStatus(): StatusData | undefined {
    return this.lastStatus;
  }

  private async poll(): Promise<void> {
    try {
      const status = await this.client.getStatus();
      this.lastStatus = status;
      this.setOnline(status);
    } catch {
      this.lastStatus = undefined;
      this.setOffline();
    }
  }

  private setOnline(status: StatusData): void {
    if (status.low_context_count > 0) {
      this.item.text =
        `$(warning) Synth (${status.low_context_count})`;
      this.item.tooltip =
        `${status.low_context_count} file(s) need ` +
        `attention — click for details`;
      this.item.backgroundColor =
        new vscode.ThemeColor(
          'statusBarItem.warningBackground'
        );
    } else {
      this.item.text = '$(check) Synth';
      this.item.tooltip =
        'Synth daemon running — click for details';
      this.item.backgroundColor = undefined;
    }
  }

  private setOffline(): void {
    this.item.text = '$(circle-slash) Synth';
    this.item.tooltip =
      'Synth daemon not reachable — click for details';
    this.item.backgroundColor = undefined;
  }
}
