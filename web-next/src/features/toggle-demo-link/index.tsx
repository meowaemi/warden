'use client';
import React from 'react';
import { useGetDemoQuery, useSetDemoMutation } from '../../shared/api';
import { useSession } from '../../shared/lib/session';
import { useI18n } from '../../shared/i18n';
export function DemoControls({ siteID }: { siteID: string }) {
  const { connected, sessionKey } = useSession();
  const { t } = useI18n();
  const { currentData: demo } = useGetDemoQuery(sessionKey, { skip: !connected });
  const [setLink, result] = useSetDemoMutation();
  if (!demo?.simulated || siteID !== 'demo-site') return null;
  return (
    <section className="panel stack notice notice-warning" aria-labelledby="demo-title">
      <h2 id="demo-title" className="section-title">
        {t('commands.demoTitle')}
      </h2>
      <p>{t('commands.demoDescription')}</p>
      <p>
        {t('commands.link')}:{' '}
        <span className={`badge badge-${demo.connected ? 'success' : 'warning'}`}>
          {t(demo.connected ? 'commands.connected' : 'commands.disconnected')}
        </span>
      </p>
      <button
        className="button button-ghost"
        type="button"
        disabled={result.isLoading}
        onClick={() => {
          void setLink({ connected: !demo.connected, sessionKey })
            .unwrap()
            .catch(() => {});
        }}
      >
        {t(
          result.isLoading
            ? 'commands.updating'
            : demo.connected
              ? 'commands.disconnect'
              : 'commands.reconnect',
        )}
      </button>
      {result.isError && (
        <p className="notice notice-error" role="alert">
          {t('commands.demoError')}
        </p>
      )}
    </section>
  );
}
