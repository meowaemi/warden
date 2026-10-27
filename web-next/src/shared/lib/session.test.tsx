import { afterEach, expect, it, vi } from 'vitest';
import { act, cleanup, renderHook } from '@testing-library/react';
import { Provider } from 'react-redux';
import type { ReactNode } from 'react';
import { makeStore } from './store';
import { SessionProvider, useSession } from './session';
import { installCredential } from './credential';
import { api } from '@/shared/api';
afterEach(() => {
  cleanup();
  installCredential('');
});
it('connect and disconnect invalidate cached authenticated data and never persist credential', () => {
  const store = makeStore();
  const reset = vi.spyOn(api.util, 'resetApiState');
  const storage = vi.spyOn(Storage.prototype, 'setItem');
  const wrapper = ({ children }: { children: ReactNode }) => (
    <Provider store={store}>
      <SessionProvider>{children}</SessionProvider>
    </Provider>
  );
  const { result } = renderHook(useSession, { wrapper });
  const before = result.current.sessionKey;
  act(() => result.current.connect('temporary-secret'));
  expect(result.current.connected).toBe(true);
  expect(result.current.sessionKey).toBeGreaterThan(before);
  act(() => result.current.disconnect());
  expect(result.current.connected).toBe(false);
  expect(reset).toHaveBeenCalledTimes(2);
  expect(storage).not.toHaveBeenCalled();
  expect(JSON.stringify(store.getState())).not.toContain('temporary-secret');
});
