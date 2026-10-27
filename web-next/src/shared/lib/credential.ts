let token = '';
let key = 0;
const listeners = new Set<() => void>();
export function installCredential(value: string): void {
  token = value.trim();
  key += 1;
  listeners.forEach((listener) => listener());
}
export function sessionKey(): number {
  return key;
}
export function hasCredential(): boolean {
  return token.length > 0;
}
export function subscribeSession(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
export function authorizationFor(expectedKey: number): string | null {
  return expectedKey === key && token ? `Bearer ${token}` : null;
}
