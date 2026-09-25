import { API_BASE, HTTP_CONFLICT } from '../constants/app.constants';
import { AppException } from '../errors/AppException';
import { logger } from '../logger/logger';
import type { Transfer } from '../types/domain';

interface BackendBody {
  code: number;
  message: string;
  data?: unknown;
}

// request 统一处理 JSON 请求/响应与后端错误消息解包。
const request = async <T>(path: string, options?: RequestInit): Promise<T> => {
  let response: Response;
  try {
    response = await fetch(`${API_BASE}${path}`, {
      headers: { 'Content-Type': 'application/json' },
      ...options,
    });
  } catch (err) {
    logger.error('transfer request network error', path, err);
    throw new AppException('NETWORK_ERROR', '网络异常，请稍后重试');
  }

  let body: BackendBody | null = null;
  try {
    body = (await response.json()) as BackendBody;
  } catch {
    body = null;
  }
  if (!response.ok || !body || body.code !== 0) {
    const message = body?.message || '操作失败';
    throw new AppException(response.status === HTTP_CONFLICT ? 'TRANSFER_CONFLICT' : 'TRANSFER_FAILED', message);
  }
  return body.data as T;
};

export interface CreateTransferPayload {
  machineCode: string;
  toField: string;
  estimatedArrival: string;
  applicant: string;
}

export interface TransferActionResult {
  transferId: string;
  machineCode?: string;
  status: string;
  field?: string;
  message: string;
}

export const createTransfer = (payload: CreateTransferPayload) =>
  request<TransferActionResult>('/transfers', { method: 'POST', body: JSON.stringify(payload) });

export const arriveTransfer = (id: string) =>
  request<TransferActionResult>(`/transfers/${id}/arrive`, { method: 'POST' });

export const cancelTransfer = (id: string, operator: string) =>
  request<TransferActionResult>(`/transfers/${id}/cancel`, {
    method: 'POST',
    body: JSON.stringify({ operator }),
  });

export const fetchTransfers = async (machineCode = ''): Promise<Transfer[]> => {
  const query = machineCode.trim() ? `?machineCode=${encodeURIComponent(machineCode.trim())}` : '';
  const data = await request<{ items: Transfer[]; total: number }>(`/transfers${query}`);
  return data.items;
};
