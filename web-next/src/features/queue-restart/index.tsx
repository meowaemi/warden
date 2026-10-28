'use client';
import React, { useState } from 'react';
import * as AlertDialog from '@radix-ui/react-alert-dialog';
import { useCreateCommandMutation } from '../../shared/api';
import type { APIError, CreateCommand } from '../../shared/api/types';
import { useSession } from '../../shared/lib/session';
import { useI18n } from '../../shared/i18n';
import { prepareCommand, definitiveRejection } from './model';

/**
 * QueueRestart queues a Deployment restart after explicit confirmation.
 * If the outcome of a send is unknown, the same request is kept for retry.
 */
export function QueueRestart({ siteID, simulated }: { siteID: string; simulated: boolean }) {
  const { t, date } = useI18n();
  const { sessionKey, connected } = useSession();
  const [send, { isLoading }] = useCreateCommandMutation();
  const [namespace, setNamespace] = useState('');
  const [name, setName] = useState('');
  const [uid, setUID] = useState('');
  const [ttl, setTTL] = useState(10);
  // review is the command in the confirmation dialog; pending is the one sent
  // whose outcome is not known yet.
  const [review, setReview] = useState<CreateCommand | null>(null);
  const [pending, setPending] = useState<CreateCommand | null>(null);
  const [notice, setNotice] = useState<
    'validation' | 'error' | 'rejected' | 'conflict' | 'accepted' | null
  >(null);
  const [rawError, setRawError] = useState('');
  // prepare checks the form and opens the confirmation dialog.
  function prepare() {
    const command = prepareCommand({ siteID, namespace, name, uid, ttl });
    if (!command) {
      setNotice('validation');
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
  // The form is locked while a request is in flight or its outcome is unknown.
  const locked = !!pending || isLoading;
  return (
    <section className="panel stack" aria-labelledby="restart-title">
      <div className="panel-header">
        <h2 id="restart-title" className="section-title">
          {t('commands.title')}
        </h2>
      </div>
      <p className="muted">{t('commands.description')}</p>
      <p className="notice notice-info">{t('commands.reload')}</p>
      {simulated && siteID === 'demo-site' && (
        <button
          className="button button-ghost"
          type="button"
          disabled={locked}
          onClick={() => {
            setNamespace('demo');
            setName('demo-app');
            setUID('demo-deployment-uid');
          }}
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
          className="toolbar"
          disabled={locked || !connected}
          style={{ border: 0, padding: 0, margin: 0 }}
        >
          <label className="field">
            {t('commands.namespace')}
            <input
              className="input"
              value={namespace}
              onChange={(e) => setNamespace(e.target.value)}
              required
            />
          </label>
          <label className="field">
            {t('commands.name')}
            <input
              className="input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </label>
          <label className="field">
            {t('commands.uid')}
            <input
              className="input mono"
              value={uid}
              onChange={(e) => setUID(e.target.value)}
              required
            />
          </label>
          <label className="field">
            {t('commands.ttl')}
            <input
              className="input"
              type="number"
              min="1"
              max="1440"
              value={Number.isNaN(ttl) ? '' : ttl}
              onChange={(e) => setTTL(e.target.valueAsNumber)}
              required
            />
          </label>
        </fieldset>
        {!pending && (
          <button
            className="button button-primary"
            disabled={isLoading || !connected || !siteID}
            type="submit"
          >
            {t('commands.queue')}
          </button>
        )}
      </form>
      {pending && (
        <div className="notice notice-warning stack">
          <p>{t('commands.pending')}</p>
          <p className="mono">
            {pending.site_id} · {pending.namespace}/{pending.name} · {pending.id}
          </p>
          <p>
            {t('commands.expires')}: {date(pending.expires_at)}
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
              {t('commands.confirmTitle')}
            </AlertDialog.Title>
            <AlertDialog.Description className="muted">
              {t('commands.confirmDescription')}
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
                    {review.namespace}/{review.name}
                  </dd>
                </div>
                <div>
                  <dt>{t('commands.uid')}</dt>
                  <dd className="mono">{review.target_uid}</dd>
                </div>
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
