import * as net from 'net';
import * as os from 'os';
import * as path from 'path';
import {
  SynthRequest,
  SynthResponse,
  encodeRequest,
  TYPE_PING,
  PingData,
  parsePingData,
  parseErrorData,
  STATUS_ERROR,
  TYPE_STATUS,
  StatusData,
  parseStatusData,
  TYPE_SEARCH,
  SearchPayload,
  SearchData,
  parseSearchData,
} from './protocol';

// IMPORTANT: The macOS path is implemented based on the known behavior of the adrg/xdg Go library,
// but has NOT been manually verified against a real running daemon on macOS.
// This must be verified before Phase 1 is considered fully cross-platform complete.
export function getDefaultSocketPath(): string {
  const home = os.homedir();
  if (process.platform === 'darwin') {
    return path.join(
      home, 'Library', 'Application Support',
      'synth', 'daemon.sock'
    );
  }
  const xdgDataHome = process.env.XDG_DATA_HOME
    || path.join(home, '.local', 'share');
  return path.join(xdgDataHome, 'synth', 'daemon.sock');
}

export class SynthClient {
  constructor(private sockPath: string) {}

  send(req: SynthRequest, timeoutMs = 5000): Promise<SynthResponse> {
    return new Promise((resolve, reject) => {
      const socket = net.createConnection(this.sockPath);
      let buffer = '';
      let settled = false;

      const timer = setTimeout(() => {
        if (settled) return;
        settled = true;
        socket.destroy();
        reject(new Error(
          `timed out connecting to daemon at ${this.sockPath}`
        ));
      }, timeoutMs);

      socket.on('connect', () => {
        socket.write(encodeRequest(req));
      });

      socket.on('data', (chunk) => {
        buffer += chunk.toString('utf8');
        const newlineIdx = buffer.indexOf('\n');
        if (newlineIdx === -1) return;
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        try {
          const resp = JSON.parse(
            buffer.slice(0, newlineIdx)
          ) as SynthResponse;
          socket.end();
          resolve(resp);
        } catch (err) {
          socket.destroy();
          reject(new Error(
            `failed to parse daemon response: ${err}`
          ));
        }
      });

      socket.on('error', (err) => {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        reject(new Error(
          `cannot connect to daemon socket: ${err.message} ` +
          `— is the daemon running?`
        ));
      });
    });
  }

  async ping(): Promise<PingData> {
    const resp = await this.send({
      type: TYPE_PING,
      payload: {},
    });
    if (resp.status === STATUS_ERROR) {
      const errData = parseErrorData(resp);
      throw new Error(`ping failed: ${errData.message}`);
    }
    return parsePingData(resp);
  }

  async getStatus(): Promise<StatusData> {
    const resp = await this.send({
      type: TYPE_STATUS,
      payload: {},
    });
    if (resp.status === STATUS_ERROR) {
      const errData = parseErrorData(resp);
      throw new Error(
        `status request failed: ${errData.message}`
      );
    }
    return parseStatusData(resp);
  }

  async search(
    payload: SearchPayload
  ): Promise<SearchData> {
    const resp = await this.send({
      type: TYPE_SEARCH,
      payload,
    });
    if (resp.status === STATUS_ERROR) {
      const errData = parseErrorData(resp);
      throw new Error(
        `search failed: ${errData.message}`
      );
    }
    return parseSearchData(resp);
  }

  async isDaemonReachable(): Promise<boolean> {
    try {
      await this.ping();
      return true;
    } catch {
      return false;
    }
  }
}
