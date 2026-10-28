'use client';
import { Server, ChevronRight } from 'lucide-react';
import type { Site } from '@/shared/api/types';
import { useI18n } from '@/shared/i18n';
import { neverObserved } from '@/shared/lib/observed';
export function SitesOverview({
  sites,
  selectedID,
  onSelect,
  loading,
  unavailable,
}: {
  sites: Site[];
  selectedID: string;
  onSelect: (id: string) => void;
  loading: boolean;
  unavailable: boolean;
}) {
  const { t } = useI18n();
  return (
    <section className="panel" aria-label={t('monitor.sites')}>
      <div className="panel-header">
        <h2 className="section-title">
          <Server size={18} aria-hidden="true" /> {t('monitor.sites')}
        </h2>
      </div>
      {loading && !sites.length ? (
        <div className="empty-state" role="status">
          {t('monitor.loading')}
        </div>
      ) : !sites.length ? (
        <div className="empty-state">
          {unavailable ? t('monitor.unknown') : t('monitor.noSites')}
        </div>
      ) : (
        <div className="stack">
          {sites.map((site) => {
            const unknown = unavailable || neverObserved(site.last_success);
            const label = unknown
              ? 'monitor.unknown'
              : site.stale
                ? 'monitor.stale'
                : 'monitor.fresh';
            return (
              <button
                key={site.id}
                type="button"
                className={`button site-option ${selectedID === site.id ? 'site-option-selected' : ''}`}
                aria-pressed={selectedID === site.id}
                onClick={() => onSelect(site.id)}
              >
                <span className="stack">
                  <span className="mono">{site.id}</span>
                  <span
                    className={`badge badge-${unknown ? 'neutral' : site.stale ? 'warning' : 'success'}`}
                  >
                    {t(label)}
                  </span>
                </span>
                <ChevronRight size={16} aria-hidden="true" />
              </button>
            );
          })}
        </div>
      )}
    </section>
  );
}
