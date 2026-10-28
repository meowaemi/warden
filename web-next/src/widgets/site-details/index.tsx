'use client';
import { Activity, AlertTriangle } from 'lucide-react';
import type { Site } from '@/shared/api/types';
import { useI18n } from '@/shared/i18n';
import { neverObserved } from '@/shared/lib/observed';
export function SiteDetails({ site, unavailable }: { site: Site; unavailable: boolean }) {
  const { t, date, relative, timeZone } = useI18n();
  const never = neverObserved(site.last_success);
  return (
    <section className="panel">
      <div className="panel-header">
        <h2 className="section-title">
          <Activity size={18} aria-hidden="true" /> {t('monitor.details')}
        </h2>
        <span
          className={`badge badge-${unavailable || never ? 'neutral' : site.stale ? 'warning' : 'success'}`}
        >
          {t(
            unavailable
              ? 'monitor.unknown'
              : never
                ? 'monitor.noObservation'
                : site.stale
                  ? 'monitor.stale'
                  : 'monitor.fresh',
          )}
        </span>
      </div>
      {unavailable && (
        <p className="notice notice-warning" role="status">
          {t('monitor.connectionUnknown')}
        </p>
      )}
      <dl className="site-facts">
        <div>
          <dt className="muted">{t('monitor.lastContact')}</dt>
          <dd title={date(site.last_contact)}>
            {date(site.last_contact)} <span className="muted">{relative(site.last_contact)}</span>
          </dd>
        </div>
        <div>
          <dt className="muted">{t('monitor.lastSuccess')}</dt>
          <dd title={date(site.last_success)}>
            {date(site.last_success)} <span className="muted">{relative(site.last_success)}</span>
          </dd>
        </div>
        <div>
          <dt className="muted">{t('monitor.timezone')}</dt>
          <dd className="mono">{timeZone}</dd>
        </div>
      </dl>
      {site.last_error && (
        <div className="notice notice-warning">
          <p>
            <AlertTriangle size={16} aria-hidden="true" /> {t('monitor.collectionWarning')}
          </p>
          <details className="technical">
            <summary>{t('monitor.raw')}</summary>
            <pre>{site.last_error}</pre>
          </details>
        </div>
      )}
    </section>
  );
}
