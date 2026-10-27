'use client';
import { useState } from 'react';
import {
  Activity,
  ArrowRight,
  Cable,
  ChevronRight,
  Clock3,
  Command,
  Globe2,
  Layers3,
  LockKeyhole,
  Moon,
  Radio,
  Settings2,
  ShieldCheck,
  Sun,
  TerminalSquare,
  Unplug,
  Wifi,
  WifiOff,
} from 'lucide-react';
import { useGetDemoQuery, useGetSitesQuery } from '@/shared/api';
import type { APIError } from '@/shared/api/types';
import { useI18n } from '@/shared/i18n';
import { useSession } from '@/shared/lib/session';
import { SitesOverview } from '@/widgets/sites-overview';
import { SiteDetails } from '@/widgets/site-details';
import { ProblemsList } from '@/widgets/problems-list';
import { ObservationHistory } from '@/widgets/observation-history';
import { CommandsList } from '@/widgets/commands-list';
import { QueueRestart } from '@/features/queue-restart';
import { DemoControls } from '@/features/toggle-demo-link';

export function Dashboard() {
  const { t, locale, setLocale, theme, setTheme, timeZone } = useI18n();
  const { connected, sessionKey, connect, disconnect } = useSession();
  const [settings, setSettings] = useState(false);
  const [selected, setSelected] = useState('');
  const sitesQuery = useGetSitesQuery(sessionKey, { skip: !connected, pollingInterval: 5000 });
  const demo = useGetDemoQuery(sessionKey, { skip: !connected, pollingInterval: 5000 });
  const sites = connected ? (sitesQuery.currentData ?? []) : [];
  const selectedID = sites.some((s) => s.id === selected) ? selected : (sites[0]?.id ?? '');
  const site = sites.find((s) => s.id === selectedID);
  const unavailable = sitesQuery.isError;
  const authenticated = connected && !!sitesQuery.currentData && !unavailable;
  const error = sitesQuery.error as APIError | undefined;
  const statusKey = !connected
    ? 'shell.notConnected'
    : unavailable
      ? 'shell.centerUnavailable'
      : authenticated
        ? 'shell.centerAvailable'
        : 'shell.connecting';
  const problemCount = sites.reduce((n, s) => n + s.messages.length, 0);
  const staleCount = sites.filter(
    (s) => unavailable || s.stale || !s.last_success || s.last_success.startsWith('0001-'),
  ).length;
  function login(event: React.FormEvent) {
    event.preventDefault();
    const form = event.currentTarget as HTMLFormElement;
    const value = String(new FormData(form).get('operator-token') ?? '').trim();
    if (!value) return;
    connect(value);
    form.reset();
    setSettings(false);
  }
  return (
    <div className="app-shell">
      <a className="skip-link" href="#main">
        {t('shell.skip')}
      </a>
      <aside className="sidebar">
        <a className="brand" href="#overview">
          <span className="brand-mark">
            <Layers3 size={22} strokeWidth={2.2} />
          </span>
          <span>
            warden<span className="brand-dot">.</span>
          </span>
        </a>
        <p className="sidebar-caption">{t('shell.brandSubtitle')}</p>
        <nav className="side-nav" aria-label={t('shell.workspace')}>
          <a href="#overview" className="nav-link nav-active">
            <Globe2 size={18} />
            {t('shell.overview')}
          </a>
          <a href="#commands" className="nav-link">
            <TerminalSquare size={18} />
            {t('shell.commands')}
          </a>
          <a href="#history" className="nav-link">
            <Clock3 size={18} />
            {t('shell.history')}
          </a>
        </nav>
        <div className="sidebar-bottom">
          <div className="sidebar-status">
            <span className={`status-dot ${authenticated ? 'is-online' : ''}`} />
            <span>{t(statusKey)}</span>
          </div>
          <span className="sidebar-footnote">{t('shell.observationScope')}</span>
          <div className="operator">
            <span className="operator-avatar">
              <ShieldCheck size={18} />
            </span>
            <span>
              {t('shell.operator')}
              <small>{timeZone}</small>
            </span>
            <button
              className="icon-button"
              onClick={() => setSettings((v) => !v)}
              aria-label={t('shell.settings')}
            >
              <Settings2 size={17} />
            </button>
          </div>
        </div>
      </aside>
      <div className="workspace">
        <header className="topbar">
          <div className="breadcrumb">
            <span>{t('shell.workspace')}</span>
            <ChevronRight size={13} />
            <strong>{t('shell.infrastructure')}</strong>
          </div>
          <div className="topbar-actions">
            <div className="language-switch" role="group" aria-label={t('shell.language')}>
              <button onClick={() => setLocale('ru')} aria-pressed={locale === 'ru'}>
                RU
              </button>
              <button onClick={() => setLocale('en')} aria-pressed={locale === 'en'}>
                EN
              </button>
            </div>
            <button
              className="icon-button theme-switch"
              onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
              aria-label={t('shell.theme')}
              title={t(theme === 'dark' ? 'shell.light' : 'shell.dark')}
            >
              {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
            </button>
            <button
              className="button button-ghost connection-button"
              onClick={() => setSettings((v) => !v)}
            >
              <Cable size={15} />
              {t('shell.connection')}
            </button>
          </div>
        </header>
        <main id="main" className="main-content">
          <section className="page-heading" id="overview">
            <div>
              <p className="eyebrow">
                <span className="accent-tick" />
                {t('shell.console')}
              </p>
              <h1>{t('shell.title')}</h1>
              <p className="page-description">{t('shell.description')}</p>
            </div>
            <span
              className={`connection-state ${authenticated ? 'online' : unavailable ? 'offline' : ''}`}
            >
              {authenticated ? (
                <Wifi size={14} />
              ) : unavailable ? (
                <WifiOff size={14} />
              ) : (
                <Radio size={14} />
              )}{' '}
              {t(statusKey)}
            </span>
          </section>
          {(!connected || settings) && (
            <section className="panel connect-panel">
              <div className="connect-intro">
                <span className="connect-icon">
                  <LockKeyhole size={21} />
                </span>
                <div>
                  <h2 className="section-title">{t('shell.connection')}</h2>
                  <p className="muted">{t('shell.accessDescription')}</p>
                </div>
              </div>
              <form className="connect-form" onSubmit={login}>
                <label className="field">
                  <span className="sr-only">{t('shell.token')}</span>
                  <input
                    className="input"
                    type="password"
                    autoComplete="off"
                    name="operator-token"
                    placeholder={t('shell.tokenPlaceholder')}
                    required
                  />
                </label>
                <button className="button button-primary" type="submit">
                  {t('shell.connect')}
                  <ArrowRight size={15} />
                </button>
                {connected && (
                  <button
                    className="button button-ghost"
                    type="button"
                    onClick={() => {
                      disconnect();
                    }}
                  >
                    <Unplug size={15} />
                    {t('shell.disconnect')}
                  </button>
                )}
              </form>
            </section>
          )}
          {unavailable && (
            <div className="notice notice-error" role="alert">
              <p>
                {t(
                  error?.status === 401 || error?.status === 403
                    ? 'shell.unauthorized'
                    : error?.status === 'TIMEOUT_ERROR'
                      ? 'shell.timeout'
                      : 'shell.failed',
                )}
              </p>
              {error?.message && (
                <details className="technical">
                  <summary>{t('shell.details')}</summary>
                  <pre>{error.message}</pre>
                </details>
              )}
            </div>
          )}
          {connected && (
            <div className="stats-strip">
              <div className="stat">
                <span className="stat-icon">
                  <Globe2 size={19} />
                </span>
                <div>
                  <span>{t('shell.siteCount')}</span>
                  <strong>
                    {sitesQuery.isLoading ? '—' : sites.length.toString().padStart(2, '0')}
                  </strong>
                </div>
              </div>
              <div className="stat">
                <span className="stat-icon warning">
                  <Activity size={19} />
                </span>
                <div>
                  <span>{t('shell.problemCount')}</span>
                  <strong>
                    {sitesQuery.isLoading ? '—' : problemCount.toString().padStart(2, '0')}
                  </strong>
                </div>
              </div>
              <div className="stat">
                <span className="stat-icon">
                  <Clock3 size={19} />
                </span>
                <div>
                  <span>{t('shell.staleCount')}</span>
                  <strong>
                    {sitesQuery.isLoading ? '—' : staleCount.toString().padStart(2, '0')}
                  </strong>
                </div>
              </div>
            </div>
          )}
          {demo.currentData?.simulated && <DemoControls siteID={selectedID} />}
          <div className="monitoring-grid">
            <div className="sites-column">
              <SitesOverview
                sites={sites}
                selectedID={selectedID}
                onSelect={setSelected}
                loading={sitesQuery.isLoading}
                unavailable={unavailable}
              />
              <div className="scope-note">
                <ShieldCheck size={16} />
                <p>{t('shell.noFabrication')}</p>
              </div>
            </div>
            <div className="detail-column">
              {site ? (
                <>
                  <div className="selected-heading">
                    <h2>
                      <span
                        className={`status-dot ${!site.stale && !unavailable ? 'is-online' : ''}`}
                      />
                      {site.id}
                    </h2>
                    <span className="live-label">
                      <span className={`status-dot ${unavailable ? '' : 'is-online'}`} />
                      {t('shell.live')}
                    </span>
                  </div>
                  <SiteDetails site={site} unavailable={unavailable} />
                  <ProblemsList site={site} unavailable={unavailable} />
                </>
              ) : (
                <section className="panel empty-dashboard">
                  <span className="empty-icon">
                    <Layers3 size={30} />
                  </span>
                  <h2>{t('shell.selectSite')}</h2>
                  <p className="muted">{t('shell.selectHint')}</p>
                </section>
              )}
            </div>
          </div>
          <section id="commands" className="commands-section">
            <div className="section-eyebrow">
              <TerminalSquare size={16} />
              <span>{t('shell.commands')}</span>
            </div>
            <div className="commands-grid">
              <QueueRestart siteID={selectedID} simulated={demo.currentData?.simulated === true} />
              {site ? (
                <CommandsList siteID={selectedID} />
              ) : (
                <section className="panel empty-state">
                  <Command size={25} />
                  <p>{t('shell.selectHint')}</p>
                </section>
              )}
            </div>
          </section>
          <section id="history">{site && <ObservationHistory siteID={selectedID} />}</section>
          <footer className="workspace-footer">
            <span>
              <ShieldCheck size={14} /> Warden
            </span>
            <span>
              {t('shell.timezone')}: {timeZone}
            </span>
            <span>{t('shell.live')}</span>
          </footer>
        </main>
      </div>
    </div>
  );
}
