import { API_BASE } from '../constants/app.constants';
import { AppException } from '../errors/AppException';
import { logger } from '../logger/logger';
import type { Transfer } from '../types/domain';

export interface CreateTransferPayload {
  machineCode: string;
  toField: string;
  expectedArriveAt: string;
  applicant?: string;
}

// unwrap 解包后端统一响应 {code, message, data}，并把业务错误转成带后端提示的异常。
const unwrap = async <T>(response: Response, fallback: string): Promise<T> => {
  const body = await response.json().catch(() => null);
  if (!response.ok || (body && typeof body === 'object' && body.code !== 0)) {
    const message = body?.message || fallback;
    logger.error('transfer request failed', response.status, message);
    throw new AppException('TRANSFER_FAILED', message);
  }
  if (body && typeof body === 'object' && body.code === 0) {
    return body.data as T;
  }
  return body as T;
};

export const createTransfer = async (payload: CreateTransferPayload): Promise<Transfer> => {
  const response = await fetch(`${API_BASE}/transfers`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return unwrap<Transfer>(response, '发起转场失败');
};

export const arriveTransfer = async (id: string): Promise<Transfer> => {
  const response = await fetch(`${API_BASE}/transfers/${id}/arrive`, { method: 'POST' });
  return unwrap<Transfer>(response, '到达确认失败');
};

export const cancelTransfer = async (id: string, reason?: string): Promise<Transfer> => {
  const response = await fetch(`${API_BASE}/transfers/${id}/cancel`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ reason: reason ?? '' }),
  });
  return unwrap<Transfer>(response, '取消转场失败');
};

export const fetchTransfers = async (machineCode = ''): Promise<Transfer[]> => {
  const query = machineCode ? `?machineCode=${encodeURIComponent(machineCode)}` : '';
  const response = await fetch(`${API_BASE}/transfers${query}`);
  const data = await unwrap<{ items: Transfer[] } | Transfer[]>(response, '转场记录加载失败');
  return Array.isArray(data) ? data : data.items;
};
