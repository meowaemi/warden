import type { CreateCommand, Problem, Resources, TargetKind } from '../../shared/api/types';

/** targetKinds are the workload kinds a fix can target. */
export const targetKinds: TargetKind[] = ['Deployment', 'StatefulSet', 'DaemonSet'];

/** maxReplicas matches the center's limit. */
export const maxReplicas = 1000;

/** Draft is the form state. */
export interface Draft {
  kind: 'set_resources' | 'scale';
  targetKind: TargetKind;
  namespace: string;
  name: string;
  uid: string;
  container: string;
  /** Kubernetes quantities; an empty one is left unchanged. */
  resources: Required<Resources>;
  replicas: number;
  ttl: number;
}

/** emptyDraft is a blank form. */
export const emptyDraft: Draft = {
  kind: 'set_resources',
  targetKind: 'Deployment',
  namespace: '',
  name: '',
  uid: '',
  container: '',
  resources: { cpu_request: '', cpu_limit: '', memory_request: '', memory_limit: '' },
  replicas: 1,
  ttl: 10,
};

/** quantity matches what an operator types: 500m, 1.5, 256Mi, 1Gi. */
const quantity = /^(\d+(\.\d+)?|\.\d+)(m|k|M|G|T|Ki|Mi|Gi|Ti)?$/;

/** validQuantity accepts a positive Kubernetes quantity. */
function validQuantity(value: string): boolean {
  return quantity.test(value) && parseFloat(value) > 0;
}

/**
 * prepareAdjustment builds a command for siteID, or null if the draft is
 * incomplete. It only catches typos; the center and the agent check again.
 */
export function prepareAdjustment(
  siteID: string,
  d: Draft,
  now = Date.now(),
  id = crypto.randomUUID(),
): CreateCommand | null {
  const namespace = d.namespace.trim();
  const name = d.name.trim();
  const uid = d.uid.trim();
  if (
    !siteID ||
    !namespace ||
    !name ||
    !uid ||
    !Number.isFinite(d.ttl) ||
    d.ttl < 1 ||
    d.ttl > 1440
  )
    return null;
  const common = {
    id,
    site_id: siteID,
    target_kind: d.targetKind,
    namespace,
    name,
    target_uid: uid,
    expires_at: new Date(now + d.ttl * 60000).toISOString(),
  };
  if (d.kind === 'scale') {
    if (
      d.targetKind === 'DaemonSet' ||
      !Number.isInteger(d.replicas) ||
      d.replicas < 0 ||
      d.replicas > maxReplicas
    )
      return null;
    return Object.freeze({ ...common, kind: 'scale', replicas: d.replicas });
  }
  const container = d.container.trim();
  const resources: Resources = {};
  for (const key of Object.keys(d.resources) as (keyof Resources)[]) {
    const value = d.resources[key].trim();
    if (!value) continue;
    if (!validQuantity(value)) return null;
    resources[key] = value;
  }
  if (!container || !Object.keys(resources).length) return null;
  return Object.freeze({ ...common, kind: 'set_resources', container, resources });
}

/**
 * draftFromProblem targets the workload behind a failing pod, or returns null
 * if it has none warden can change. For an OOM kill it proposes twice the limit.
 */
export function draftFromProblem(p: Problem): Draft | null {
  const { ownerKind, ownerName, ownerUID, namespace, container } = p.target;
  if (!ownerName || !ownerUID || !targetKinds.includes(ownerKind as TargetKind)) return null;
  const limit = p.details?.find((d) => d.key === 'Memory limit')?.value ?? '';
  const mi = /^(\d+)Mi$/.exec(limit);
  return {
    ...emptyDraft,
    targetKind: ownerKind as TargetKind,
    namespace,
    name: ownerName,
    uid: ownerUID,
    container,
    resources: {
      ...emptyDraft.resources,
      memory_limit: p.reason === 'OOMKilled' && mi && +mi[1] > 0 ? `${+mi[1] * 2}Mi` : '',
    },
  };
}

/** mebibytes reads Mi or Gi for the slider, 0 for anything else. */
export function mebibytes(value: string): number {
  const m = /^(\d+)(Mi|Gi)$/.exec(value.trim());
  if (!m) return 0;
  return m[2] === 'Gi' ? +m[1] * 1024 : +m[1];
}
