'use client';
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useSyncExternalStore,
  type ReactNode,
} from 'react';
import { useDispatch } from 'react-redux';
import { api } from '@/shared/api';
import { hasCredential, installCredential, sessionKey, subscribeSession } from './credential';
/** Session is the operator's connection; sessionKey changes on every connect and disconnect. */
interface Session {
  connected: boolean;
  sessionKey: number;
  connect(token: string): void;
  disconnect(): void;
}
/** SessionContext is empty outside SessionProvider. */
const SessionContext = createContext<Session | null>(null);
/** SessionProvider connects and disconnects, clearing the API cache each time. */
export function SessionProvider({ children }: { children: ReactNode }) {
  const dispatch = useDispatch();
  // The token lives in credential.ts; this re-renders on every session change.
  const key = useSyncExternalStore(subscribeSession, sessionKey, () => 0);
  const connect = useCallback(
    (token: string) => {
      if (!token.trim()) return;
      installCredential(token);
      dispatch(api.util.resetApiState());
    },
    [dispatch],
  );
  const disconnect = useCallback(() => {
    installCredential('');
    dispatch(api.util.resetApiState());
  }, [dispatch]);
  const value = useMemo(
    () => ({ connected: key !== 0 && hasCredential(), sessionKey: key, connect, disconnect }),
    [key, connect, disconnect],
  );
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}
/** useSession returns the current session. */
export function useSession(): Session {
  const value = useContext(SessionContext);
  if (!value) throw new Error('useSession requires SessionProvider');
  return value;
}
