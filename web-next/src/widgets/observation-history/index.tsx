'use client';
import { useState } from 'react';
import { History, RefreshCw } from 'lucide-react';
import { useGetHistoryQuery } from '@/shared/api';
import { useSession } from '@/shared/lib/session';
import { useI18n } from '@/shared/i18n';
export function ObservationHistory({ siteID }: { siteID: string }) {
  const { t, date } = useI18n();
  const { connected, sessionKey } = useSession();
  const [requested, setRequested] = useState<string | null>(null);
  const requestKey = `${sessionKey}:${siteID}`;
  const enabled = connected && requested === requestKey;
  const { currentData, isFetching, isError, refetch } = useGetHistoryQuery(
    { siteID, sessionKey },
    { skip: !enabled },
  );
  const entries = currentData
    ?.filter((entry) => entry.site_id === siteID)
    .slice()
    .sort((a, b) => Date.parse(b.observed_at) - Date.parse(a.observed_at));
  return (
    <section className="panel">
      <div className="panel-header">
        <h2 className="section-title">
          <History size={18} aria-hidden="true" /> {t('monitor.history')}
        </h2>
        <button
          className="button button-ghost"
          type="button"
          disabled={!connected || isFetching}
          onClick={() => {
            if (enabled) {
              void refetch();
            } else {
              setRequested(requestKey);
            }
          }}
        >
          <RefreshCw size={15} aria-hidden="true" className={isFetching ? 'spin' : ''} />
          {t(enabled ? 'monitor.refresh' : 'monitor.loadHistory')}
        </button>
      </div>
      <p className="muted">{t('monitor.historyHint')}</p>
      {!connected && <p className="empty-state">{t('monitor.signIn')}</p>}
      {enabled && isError && (
        <p className="notice notice-warning" role="alert">
          {t('monitor.historyError')}
        </p>
      )}
      {enabled && isFetching && !entries && (
        <div className="skeleton" role="status" aria-label={t('monitor.loading')} />
      )}
      {enabled &&
        entries &&
        (!entries.length ? (
          <p className="empty-state">{t('monitor.noHistory')}</p>
        ) : (
          <div className="table-scroll">
            <table className="data-table">
              <thead>
                <tr>
                  <th>{t('monitor.observed')}</th>
                  <th>{t('monitor.details')}</th>
                  <th>{t('monitor.problemCount')}</th>
                  <th>{t('monitor.raw')}</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((entry) => (
                  <tr key={entry.id}>
                    <td>{date(entry.observed_at)}</td>
                    <td>
                      <span className={`badge badge-${entry.collected ? 'success' : 'warning'}`}>
                        {t(entry.collected ? 'monitor.collected' : 'monitor.collectionFailed')}
                      </span>
                      {entry.error && (
                        <span className="badge badge-warning">
                          {t('monitor.collectionWarning')}
                        </span>
                      )}
                    </td>
                    <td>{entry.collected ? (entry.messages?.length ?? 0) : '—'}</td>
                    <td>
                      <details className="technical">
                        <summary>{t('monitor.raw')}</summary>
                        <pre>{JSON.stringify(entry, null, 2)}</pre>
                      </details>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ))}
    </section>
  );
}
