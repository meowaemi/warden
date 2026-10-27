import type { BaseQueryFn } from '@reduxjs/toolkit/query';
import { authorizationFor, sessionKey } from '@/shared/lib/credential';
import type { APIError } from './types';
export interface RequestArgs {
  path: string;
  method?: 'GET' | 'POST';
  body?: unknown;
  sessionKey: number;
}
const staleSession = (): { error: APIError } => ({
  error: {
    status: 'FETCH_ERROR',
    message: 'Session changed; response discarded. Operation outcome may be unknown.',
  },
});
export const authenticatedQuery: BaseQueryFn<RequestArgs, unknown, APIError> = async (
  args,
  api,
) => {
  const authorization = authorizationFor(args.sessionKey);
  if (!authorization) return staleSession();
  if (!args.path.startsWith('/api/') || args.path.includes('://')) {
    return { error: { status: 'FETCH_ERROR', message: 'Invalid API path.' } };
  }
  const controller = new AbortController();
  let timedOut = false;
  const abort = () => controller.abort();
  api.signal.addEventListener('abort', abort, { once: true });
  if (api.signal.aborted) controller.abort();
  const timeout = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, 15_000);
  try {
    const response = await fetch(args.path, {
      method: args.method ?? 'GET',
      headers: {
        Authorization: authorization,
        Accept: 'application/json',
        ...(args.body === undefined ? {} : { 'Content-Type': 'application/json' }),
      },
      body: args.body === undefined ? undefined : JSON.stringify(args.body),
      signal: controller.signal,
      credentials: 'omit',
      redirect: 'error',
      cache: 'no-store',
    });
    const text = await response.text();
    if (sessionKey() !== args.sessionKey) return staleSession();
    let data: unknown;
    try {
      data = text ? JSON.parse(text) : null;
    } catch {
      return {
        error: {
          status: response.ok ? 'PARSING_ERROR' : response.status,
          message: 'API returned an unreadable response.',
        },
      };
    }
    if (!response.ok) {
      const raw =
        data && typeof data === 'object' && 'error' in data && typeof data.error === 'string'
          ? data.error
          : `HTTP ${response.status}`;
      const secret = authorization.slice('Bearer '.length);
      return {
        error: {
          status: response.status,
          message: raw.split(secret).join('[redacted]').slice(0, 500),
        },
      };
    }
    return { data };
  } catch {
    if (sessionKey() !== args.sessionKey) return staleSession();
    return {
      error: {
        status: timedOut ? 'TIMEOUT_ERROR' : 'FETCH_ERROR',
        message: timedOut
          ? 'Request timed out; operation outcome may be unknown.'
          : 'Connection unavailable; operation outcome may be unknown.',
      },
    };
  } finally {
    clearTimeout(timeout);
    api.signal.removeEventListener('abort', abort);
  }
};
