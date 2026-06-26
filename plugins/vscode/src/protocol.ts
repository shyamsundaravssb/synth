export interface SynthRequest {
  type: string;
  payload: unknown;
}

export interface SynthResponse {
  status: 'ok' | 'error';
  data: unknown;
}

export interface PingPayload {}

export interface PingData {
  pid: number;
  version: string;
}

export interface ErrorData {
  message: string;
  code?: string;
}

export const TYPE_PING = 'ping';
export const STATUS_OK = 'ok';
export const STATUS_ERROR = 'error';

export function encodeRequest(req: SynthRequest): Buffer {
  return Buffer.from(JSON.stringify(req) + '\n', 'utf8');
}

export function parsePingData(resp: SynthResponse): PingData {
  return resp.data as PingData;
}

export function parseErrorData(resp: SynthResponse): ErrorData {
  return resp.data as ErrorData;
}
