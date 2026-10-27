import type { APIError, CreateCommand } from '../../shared/api/types';
export type Target = { siteID: string; namespace: string; name: string; uid: string; ttl: number };
export function prepareCommand(
  target: Target,
  now = Date.now(),
  id = crypto.randomUUID(),
): CreateCommand | null {
  if (
    !target.siteID ||
    !target.namespace.trim() ||
    !target.name.trim() ||
    !target.uid.trim() ||
    !Number.isFinite(target.ttl) ||
    target.ttl < 1 ||
    target.ttl > 1440
  )
    return null;
  return Object.freeze({
    id,
    site_id: target.siteID,
    kind: 'restart_deployment',
    namespace: target.namespace.trim(),
    name: target.name.trim(),
    target_uid: target.uid.trim(),
    expires_at: new Date(now + target.ttl * 60000).toISOString(),
  });
}
export function definitiveRejection(error: unknown): boolean {
  return [400, 401, 403, 404, 409, 422].includes((error as APIError)?.status as number);
}
