import type { CreateCommand } from '../../shared/api/types';
/** Target is the restart form state; ttl is in minutes. */
export type Target = { siteID: string; namespace: string; name: string; uid: string; ttl: number };

/** prepareCommand builds a restart command, or null if the form is incomplete. */
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
// Re-exported, so the component takes everything from its model.
export { definitiveRejection } from '../../shared/api/errors';
