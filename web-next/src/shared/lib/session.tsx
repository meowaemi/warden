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
interface Session {
  connected: boolean;
  sessionKey: number;
  connect(token: string): void;
  disconnect(): void;
}
const SessionContext = createContext<Session | null>(null);
export function SessionProvider({ children }: { children: ReactNode }) {
  const dispatch = useDispatch();
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
export function useSession(): Session {
  const value = useContext(SessionContext);
  if (!value) throw new Error('useSession requires SessionProvider');
  return value;
}
