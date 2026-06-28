import * as vscode from 'vscode';
import { SynthClient } from './ipcClient';
import { SearchResultItem } from './protocol';

const DEFAULT_LIMIT = 10;

interface SearchQuickPickItem
    extends vscode.QuickPickItem {
  result: SearchResultItem;
}

function formatScoreLabel(item: SearchResultItem): string {
  if (item.search_mode === 'fts5' || item.score === 0) {
    return 'keyword match';
  }
  const pct = Math.round(item.score * 100);
  return `${pct}% match`;
}

function buildQuickPickItems(
  results: SearchResultItem[]
): SearchQuickPickItem[] {
  return results.map((r) => ({
    label: `$(file) ${r.file}`,
    description: formatScoreLabel(r),
    detail: `${r.type} · ${r.branch} · What: ${r.what}`,
    result: r,
  }));
}

async function openResultFile(
  result: SearchResultItem
): Promise<void> {
  const found = await vscode.workspace.findFiles(
    `**/${result.file}`,
    '**/node_modules/**',
    1
  );
  if (found.length === 0) {
    vscode.window.showWarningMessage(
      `Synth: could not locate ${result.file} ` +
      `in the current workspace`
    );
    return;
  }
  const document = await vscode.workspace
    .openTextDocument(found[0]);
  await vscode.window.showTextDocument(document);
}

export async function runSearch(
  client: SynthClient
): Promise<void> {
  const query = await vscode.window.showInputBox({
    title: 'Synth: Search Intent Notes',
    placeHolder:
      'e.g. "why did we remove email verification"',
    prompt: 'Search by meaning, not just keywords',
  });

  if (!query || query.trim().length === 0) {
    return;
  }

  let data;
  try {
    data = await client.search({
      query: query.trim(),
      limit: DEFAULT_LIMIT,
    });
  } catch (err) {
    const message = err instanceof Error
      ? err.message
      : String(err);
    vscode.window.showErrorMessage(
      `Synth: ${message}. Note: unlike the CLI, ` +
      `the extension cannot fall back to keyword ` +
      `search when the daemon is offline — start ` +
      `the daemon and try again.`
    );
    return;
  }

  if (data.results.length === 0) {
    vscode.window.showInformationMessage(
      `Synth: no results found for "${data.query}"`
    );
    return;
  }

  const items = buildQuickPickItems(data.results);
  const picked = await vscode.window.showQuickPick(
    items,
    {
      title:
        `Synth Search · "${data.query}" · ` +
        `${data.count} result(s) · ${data.search_mode}`,
      placeHolder: 'Select a result to open the file',
      matchOnDescription: true,
      matchOnDetail: true,
    }
  );

  if (picked) {
    await openResultFile(picked.result);
  }
}
