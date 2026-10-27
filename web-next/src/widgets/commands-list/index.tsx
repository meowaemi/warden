'use client';
import React from 'react';
import { useGetCommandsQuery } from '../../shared/api';
import type { CommandStatus } from '../../shared/api/types';
import { useSession } from '../../shared/lib/session';
import { useI18n } from '../../shared/i18n';
const tone: Record<CommandStatus, string> = {
  queued: 'neutral',
  dispatched: 'warning',
  applied: 'success',
  failed: 'error',
  expired: 'neutral',
  unknown: 'warning',
};
export function CommandsList({ siteID }: { siteID: string }) {
  const { t, date } = useI18n();
  const { connected, sessionKey } = useSession();
  const {
    currentData: commands,
    isFetching,
    isError,
  } = useGetCommandsQuery(
    { siteID, sessionKey },
    { skip: !connected || !siteID, pollingInterval: 5000 },
  );
  return (
    <section className="panel stack" aria-labelledby="commands-title">
      <div className="panel-header">
        <h2 id="commands-title" className="section-title">
          {t('commands.list')}
        </h2>
      </div>
      {isError && (
        <p role="alert" className="notice notice-error">
          {t('commands.listError')}
        </p>
      )}
      {!commands && isFetching && <p className="empty-state">{t('commands.loading')}</p>}
      {commands?.length === 0 && !isError && <p className="empty-state">{t('commands.empty')}</p>}
      <p className="muted">{t('commands.appliedHelp')}</p>
      {commands?.map((command) => (
        <article key={command.id} className="stack command-item">
          <div className="row">
            <strong className="mono">
              {command.namespace}/{command.name}
            </strong>
            <span className={`badge badge-${tone[command.status]}`}>
              {t(`commands.${command.status}`)}
            </span>
          </div>
          <p className="muted">
            {t('commands.expires')}: {date(command.expires_at)}
          </p>
          {command.status === 'unknown' && (
            <p className="notice notice-warning">{t('commands.unknownHelp')}</p>
          )}
          <details className="technical">
            <summary>{t('commands.details')}</summary>
            <dl>
              <dt>{t('commands.id')}</dt>
              <dd className="mono">{command.id}</dd>
              <dt>{t('commands.uid')}</dt>
              <dd className="mono">{command.target_uid}</dd>
            </dl>
            {command.note && <pre>{command.note}</pre>}
          </details>
        </article>
      ))}
    </section>
  );
}
