'use client';
import { useEffect, useState, type ReactNode } from 'react';
import { Provider } from 'react-redux';
import { makeStore, setupListeners } from '@/shared/lib/store';
import { SessionProvider } from '@/shared/lib/session';
import { I18nProvider } from '@/shared/i18n';
/** Providers supplies the store, the session and i18n to the app. */
export function Providers({ children }: { children: ReactNode }) {
  const [store] = useState(makeStore);
  useEffect(() => setupListeners(store.dispatch), [store]);
  return (
    <Provider store={store}>
      <SessionProvider>
        <I18nProvider>{children}</I18nProvider>
      </SessionProvider>
    </Provider>
  );
}
