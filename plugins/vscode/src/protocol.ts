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
export const TYPE_STATUS = 'status';
export const STATUS_OK = 'ok';
export const STATUS_ERROR = 'error';

export interface StatusPayload {}

export interface LowContextFileItem {
  file: string;
  save_count: number;
  has_note: boolean;
  days_since_note: number;
}

export interface StatusData {
  running: boolean;
  pid: number;
  uptime_seconds: number;
  notes_count: number;
  file_saves_count: number;
  embeddings_count: number;
  low_context_count: number;
  low_context_files: LowContextFileItem[];
  log_file: string;
  socket_file: string;
}

export function encodeRequest(req: SynthRequest): Buffer {
  return Buffer.from(JSON.stringify(req) + '\n', 'utf8');
}

export function parsePingData(resp: SynthResponse): PingData {
  return resp.data as PingData;
}

export function parseStatusData(
  resp: SynthResponse
): StatusData {
  return resp.data as StatusData;
}

export function parseErrorData(resp: SynthResponse): ErrorData {
  return resp.data as ErrorData;
}
