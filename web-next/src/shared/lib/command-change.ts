import type { Command, Resources } from '@/shared/api/types';
import type { TranslationKey } from '@/shared/i18n';

/** resourceLabels names each resource field for the UI. */
const resourceLabels: Record<keyof Resources, TranslationKey> = {
  cpu_request: 'commands.cpuRequest',
  cpu_limit: 'commands.cpuLimit',
  memory_request: 'commands.memoryRequest',
  memory_limit: 'commands.memoryLimit',
};

/** changes lists what a command sets as label/value pairs; a restart sets nothing. */
export function changes(
  c: Pick<Command, 'kind' | 'container' | 'resources' | 'replicas'>,
): { label: TranslationKey; value: string }[] {
  if (c.kind === 'scale') return [{ label: 'commands.replicas', value: String(c.replicas ?? '') }];
  if (c.kind !== 'set_resources') return [];
  const out: { label: TranslationKey; value: string }[] = [
    { label: 'commands.container', value: c.container ?? '' },
  ];
  for (const key of Object.keys(resourceLabels) as (keyof Resources)[]) {
    const value = c.resources?.[key];
    if (value) out.push({ label: resourceLabels[key], value });
  }
  return out;
}
