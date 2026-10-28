'use client';
import React, { useState } from 'react';
import * as AlertDialog from '@radix-ui/react-alert-dialog';
import { useCreateCommandMutation } from '../../shared/api';
import { definitiveRejection } from '../../shared/api/errors';
import type { APIError, CreateCommand, Resources, TargetKind } from '../../shared/api/types';
import { useSession } from '../../shared/lib/session';
import { useI18n, type TranslationKey } from '../../shared/i18n';
import { changes } from '../../shared/lib/command-change';
import {
  emptyDraft,
  maxReplicas,
  mebibytes,
  prepareAdjustment,
  targetKinds,
  type Draft,
} from './model';

/** resourceFields are the resource inputs, in form order. */
const resourceFields: { key: keyof Resources; label: TranslationKey }[] = [
  { key: 'cpu_request', label: 'commands.cpuRequest' },
  { key: 'cpu_limit', label: 'commands.cpuLimit' },
  { key: 'memory_request', label: 'commands.memoryRequest' },
  { key: 'memory_limit', label: 'commands.memoryLimit' },
];

/**
 * AdjustWorkload queues a change of container resources or replicas.
 * A new seed refills the form, unless a request awaits confirmation.
 */
export function AdjustWorkload({
  siteID,
  simulated,
  seed,
}: {
  siteID: string;
  simulated: boolean;
  seed: Draft | null;
}) {
  const { t, date } = useI18n();
  const { sessionKey, connected } = useSession();
  const [send, { isLoading }] = useCreateCommandMutation();
  const [draft, setDraft] = useState<Draft>(seed ?? emptyDraft);
  // review is the command in the confirmation dialog; pending is the one sent
  // whose outcome is not known yet.
  const [review, setReview] = useState<CreateCommand | null>(null);
  const [pending, setPending] = useState<CreateCommand | null>(null);
  const [notice, setNotice] = useState<
    'adjustValidation' | 'error' | 'rejected' | 'conflict' | 'accepted' | null
  >(null);
  const [rawError, setRawError] = useState('');
  // The form is locked while a request is in flight or its outcome is unknown.
  const locked = !!pending || isLoading;
  // A new seed is applied during render, as React advises instead of an effect.
  const [seenSeed, setSeenSeed] = useState(seed);
  if (seed !== seenSeed) {
    setSeenSeed(seed);
    if (seed && !locked) {
      setDraft(seed);
      setNotice(null);
    }
  }

  // set and setResource change part of the draft.
  const set = (patch: Partial<Draft>) => setDraft((d) => ({ ...d, ...patch }));
  const setResource = (key: keyof Resources, value: string) =>
    setDraft((d) => ({ ...d, resources: { ...d.resources, [key]: value } }));

  // prepare checks the draft and opens the confirmation dialog.
  function prepare() {
    const command = prepareAdjustment(siteID, draft);
    if (!command) {
      setNotice('adjustValidation');
      return;
    }
    setNotice(null);
    setReview(command);
  }
  // submit sends command. A definitive rejection drops it; any other failure
  // keeps it, so a retry sends the same ID.
  async function submit(command: CreateCommand) {
    setPending(command);
    setNotice(null);
    setRawError('');
    setReview(null);
    try {
      await send({ command, sessionKey }).unwrap();
      setPending(null);
      setNotice('accepted');
    } catch (error) {
      const apiError = error as APIError;
      setRawError(apiError?.message ?? '');
      if (definitiveRejection(error)) {
        setPending(null);
        setNotice(apiError.status === 409 ? 'conflict' : 'rejected');
      } else setNotice('error');
    }
  }

  // The slider shows the memory limit in Mi; 0 when it is not in Mi or Gi.
  const memory = mebibytes(draft.resources.memory_limit);
  return (
    <section id="adjust" className="panel stack" aria-labelledby="adjust-title">
      <div className="panel-header">
        <h2 id="adjust-title" className="section-title">
          {t('commands.adjustTitle')}
        </h2>
      </div>
      <p className="muted">{t('commands.adjustDescription')}</p>
      {simulated && siteID === 'demo-site' && (
        <button
          className="button button-ghost"
          type="button"
          disabled={locked}
          onClick={() =>
            setDraft({
              ...emptyDraft,
              namespace: 'demo',
              name: 'demo-app',
              uid: 'demo-deployment-uid',
              container: 'app',
              resources: { ...emptyDraft.resources, memory_limit: '256Mi' },
            })
          }
        >
          {t('commands.fillDemo')}
        </button>
      )}
      <form
        className="stack"
        onSubmit={(event) => {
          event.preventDefault();
          prepare();
        }}
      >
        <fieldset
          className="stack"
          disabled={locked || !connected}
          style={{ border: 0, padding: 0, margin: 0 }}
        >
          <div className="language-switch" role="group" aria-label={t('commands.kind')}>
            {(['set_resources', 'scale'] as const).map((kind) => (
              <button
                key={kind}
                type="button"
                aria-pressed={draft.kind === kind}
                onClick={() =>
                  set({
                    kind,
                    targetKind:
                      kind === 'scale' && draft.targetKind === 'DaemonSet'
                        ? 'Deployment'
                        : draft.targetKind,
                  })
                }
              >
                {t(`commands.kind.${kind}`)}
              </button>
            ))}
          </div>
          <div className="toolbar">
            <label className="field">
              {t('commands.targetKind')}
              <select
                className="input"
                value={draft.targetKind}
                onChange={(e) => set({ targetKind: e.target.value as TargetKind })}
              >
                {targetKinds
                  .filter((k) => draft.kind !== 'scale' || k !== 'DaemonSet')
                  .map((k) => (
                    <option key={k} value={k}>
                      {k}
                    </option>
                  ))}
              </select>
            </label>
            <label className="field">
              {t('commands.namespace')}
              <input
                className="input"
                value={draft.namespace}
                onChange={(e) => set({ namespace: e.target.value })}
                required
              />
            </label>
            <label className="field">
              {t('commands.workloadName')}
              <input
                className="input"
                value={draft.name}
                onChange={(e) => set({ name: e.target.value })}
                required
              />
            </label>
            <label className="field">
              {t('commands.workloadUID')}
              <input
                className="input mono"
                value={draft.uid}
                onChange={(e) => set({ uid: e.target.value })}
                required
              />
            </label>
            {draft.kind === 'set_resources' ? (
              <>
                <label className="field">
                  {t('commands.container')}
                  <input
                    className="input"
                    value={draft.container}
                    onChange={(e) => set({ container: e.target.value })}
                    required
                  />
                </label>
                {resourceFields.map((f) => (
                  <label className="field" key={f.key}>
                    {t(f.label)}
                    <input
                      className="input mono"
                      value={draft.resources[f.key]}
                      placeholder="—"
                      onChange={(e) => setResource(f.key, e.target.value)}
                    />
                  </label>
                ))}
              </>
            ) : (
              <label className="field">
                {t('commands.replicas')}
                <input
                  className="input"
                  type="number"
                  min="0"
                  max={maxReplicas}
                  value={Number.isNaN(draft.replicas) ? '' : draft.replicas}
                  onChange={(e) => set({ replicas: e.target.valueAsNumber })}
                  required
                />
              </label>
            )}
            <label className="field">
              {t('commands.ttl')}
              <input
                className="input"
                type="number"
                min="1"
                max="1440"
                value={Number.isNaN(draft.ttl) ? '' : draft.ttl}
                onChange={(e) => set({ ttl: e.target.valueAsNumber })}
                required
              />
            </label>
          </div>
          {draft.kind === 'set_resources' && (
            <label className="field">
              {t('commands.memorySlider')}: {memory ? `${memory}Mi` : '—'}
              <input
                className="range"
                type="range"
                min="64"
                max="8192"
                step="64"
                value={memory || 64}
                onChange={(e) => setResource('memory_limit', `${e.target.value}Mi`)}
              />
            </label>
          )}
          {draft.kind === 'set_resources' && <p className="muted">{t('commands.resourcesHint')}</p>}
        </fieldset>
        {!pending && (
          <button
            className="button button-primary"
            disabled={isLoading || !connected || !siteID}
            type="submit"
          >
            {t('commands.adjust')}
          </button>
        )}
      </form>
      {pending && (
        <div className="notice notice-warning stack">
          <p>{t('commands.pending')}</p>
          <p className="mono">
            {pending.site_id} · {pending.target_kind} {pending.namespace}/{pending.name} ·{' '}
            {pending.id}
          </p>
          <button
            className="button button-primary"
            disabled={isLoading || !connected}
            type="button"
            onClick={() => void submit(pending)}
          >
            {t(isLoading ? 'commands.submitting' : 'commands.retry')}
          </button>
        </div>
      )}
      {notice && (
        <p
          role="status"
          className={`notice ${notice === 'accepted' ? 'notice-info' : 'notice-warning'}`}
        >
          {t(`commands.${notice}`)}
        </p>
      )}
      {rawError && (
        <details className="technical">
          <summary>{t('commands.details')}</summary>
          <pre>{rawError}</pre>
        </details>
      )}
      <AlertDialog.Root
        open={!!review}
        onOpenChange={(open) => {
          if (!open) setReview(null);
        }}
      >
        <AlertDialog.Portal>
          <AlertDialog.Overlay className="dialog-overlay" />
          <AlertDialog.Content className="dialog-content panel stack">
            <AlertDialog.Title className="section-title">
              {t('commands.adjustConfirmTitle')}
            </AlertDialog.Title>
            <AlertDialog.Description className="muted">
              {t('commands.adjustConfirmDescription')}
            </AlertDialog.Description>
            {review && (
              <dl className="stack">
                <div>
                  <dt>{t('commands.site')}</dt>
                  <dd className="mono">{review.site_id}</dd>
                </div>
                <div>
                  <dt>{t('commands.target')}</dt>
                  <dd className="mono">
                    {review.target_kind} {review.namespace}/{review.name}
                  </dd>
                </div>
                <div>
                  <dt>{t('commands.workloadUID')}</dt>
                  <dd className="mono">{review.target_uid}</dd>
                </div>
                {changes(review).map((c) => (
                  <div key={c.label}>
                    <dt>{t(c.label)}</dt>
                    <dd className="mono">{c.value}</dd>
                  </div>
                ))}
                <div>
                  <dt>{t('commands.expires')}</dt>
                  <dd>{date(review.expires_at)}</dd>
                </div>
              </dl>
            )}
            <div className="row">
              <AlertDialog.Cancel className="button button-ghost">
                {t('commands.cancel')}
              </AlertDialog.Cancel>
              <AlertDialog.Action
                className="button button-primary"
                onClick={() => {
                  if (review) void submit(review);
                }}
              >
                {t('commands.confirm')}
              </AlertDialog.Action>
            </div>
          </AlertDialog.Content>
        </AlertDialog.Portal>
      </AlertDialog.Root>
    </section>
  );
}
