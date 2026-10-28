// The operator token lives only in memory. key grows on every login and
// logout, so a reply made under an older session can be recognized.
let token = '';
let key = 0;
const listeners = new Set<() => void>();

/** installCredential replaces the token and starts a new session. */
export function installCredential(value: string): void {
  token = value.trim();
  key += 1;
  listeners.forEach((listener) => listener());
}

/** sessionKey returns the current session number. */
export function sessionKey(): number {
  return key;
}

/** hasCredential reports whether a token is set. */
export function hasCredential(): boolean {
  return token.length > 0;
}

/** subscribeSession calls listener on every session change and returns an unsubscribe func. */
export function subscribeSession(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** authorizationFor returns the header for expectedKey, or null if that session is over. */
export function authorizationFor(expectedKey: number): string | null {
  return expectedKey === key && token ? `Bearer ${token}` : null;
}
