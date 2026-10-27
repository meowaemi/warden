import { afterEach, describe, expect, it, vi } from 'vitest';
import { configureStore, type Middleware } from '@reduxjs/toolkit';
import { api } from './index';
import { makeStore } from '@/shared/lib/store';
import { installCredential, sessionKey } from '@/shared/lib/credential';

afterEach(() => {
  installCredential('');
  vi.unstubAllGlobals();
});
describe('credential isolation and session boundaries', () => {
  it('sends credential only as HTTP header, without Redux arguments, state or metadata', async () => {
    const secret = 'private-operator-credential';
    const storage = vi.spyOn(Storage.prototype, 'setItem');
    installCredential(secret);
    const fetchMock = vi.fn().mockResolvedValue(new Response('[]', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    const actions: unknown[] = [];
    const capture: Middleware = () => (next) => (action) => {
      actions.push(action);
      return next(action);
    };
    const store = configureStore({
      reducer: { [api.reducerPath]: api.reducer },
      middleware: (defaults) => defaults().concat(capture, api.middleware),
      devTools: false,
    });
    const query = store.dispatch(api.endpoints.getSites.initiate(sessionKey()));
    await query;
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe(`Bearer ${secret}`);
    expect(fetchMock.mock.calls[0][1].redirect).toBe('error');
    expect(JSON.stringify(store.getState())).not.toContain(secret);
    expect(JSON.stringify(actions)).not.toContain(secret);
    expect(storage).not.toHaveBeenCalled();
    query.unsubscribe();
    store.dispatch(api.util.resetApiState());
  });
  it('discards an old response after credential replacement', async () => {
    let complete!: (response: Response) => void;
    vi.stubGlobal(
      'fetch',
      vi.fn(
        () =>
          new Promise<Response>((resolve) => {
            complete = resolve;
          }),
      ),
    );
    installCredential('first-credential');
    const store = makeStore();
    const query = store.dispatch(api.endpoints.getSites.initiate(sessionKey()));
    installCredential('second-credential');
    complete(new Response('[{"id":"old-private-site"}]', { status: 200 }));
    const result = await query;
    expect(result.error).toMatchObject({ status: 'FETCH_ERROR' });
    expect(result.data).toBeUndefined();
    expect(JSON.stringify(store.getState())).not.toContain('old-private-site');
    query.unsubscribe();
    store.dispatch(api.util.resetApiState());
  });
  it('redacts a credential echoed in a backend error before Redux sees it', async () => {
    const secret = 'never-in-redux';
    installCredential(secret);
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ error: `rejected ${secret}` }), { status: 401 }),
        ),
    );
    const store = makeStore();
    const query = store.dispatch(api.endpoints.getSites.initiate(sessionKey()));
    const result = await query;
    expect(result.error).toMatchObject({ status: 401, message: 'rejected [redacted]' });
    expect(JSON.stringify(store.getState())).not.toContain(secret);
    query.unsubscribe();
    store.dispatch(api.util.resetApiState());
  });
  it('does not send requests without a credential', async () => {
    installCredential('');
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    const store = makeStore();
    const query = store.dispatch(api.endpoints.getSites.initiate(sessionKey()));
    expect((await query).error).toMatchObject({ status: 'FETCH_ERROR' });
    expect(fetchMock).not.toHaveBeenCalled();
    query.unsubscribe();
    store.dispatch(api.util.resetApiState());
  });
});

it('does not label an unconfirmed POST as definitively rejected after a session change', async () => {
  let complete!: (response: Response) => void;
  vi.stubGlobal(
    'fetch',
    vi.fn(
      () =>
        new Promise<Response>((resolve) => {
          complete = resolve;
        }),
    ),
  );
  installCredential('before-post');
  const store = makeStore();
  const mutation = store.dispatch(
    api.endpoints.createCommand.initiate({
      sessionKey: sessionKey(),
      command: {
        id: 'same-id-required-on-retry',
        site_id: 'demo-site',
        kind: 'restart_deployment',
        namespace: 'demo',
        name: 'demo-app',
        target_uid: 'uid',
        expires_at: '2026-10-27T15:00:00Z',
      },
    }),
  );
  installCredential('after-post');
  complete(
    new Response(JSON.stringify({ id: 'same-id-required-on-retry', status: 'queued' }), {
      status: 200,
    }),
  );
  const result = await mutation;
  expect(result.error).toMatchObject({ status: 'FETCH_ERROR' });
  expect(result.data).toBeUndefined();
  mutation.reset();
  store.dispatch(api.util.resetApiState());
});
