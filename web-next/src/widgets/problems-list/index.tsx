'use client';
import { useMemo, useState } from 'react';
import { Search, CircleAlert } from 'lucide-react';
import type { Problem, Site, Severity } from '@/shared/api/types';
import { useI18n } from '@/shared/i18n';
import { neverObserved } from '@/shared/lib/observed';
/** reasonKeys maps known Kubernetes reasons to translations; others are shown as is. */
const reasonKeys = {
  OOMKilled: 'monitor.reason.OOMKilled',
  CrashLoopBackOff: 'monitor.reason.CrashLoopBackOff',
  ImagePullBackOff: 'monitor.reason.ImagePullBackOff',
  ErrImagePull: 'monitor.reason.ErrImagePull',
  Unschedulable: 'monitor.reason.Unschedulable',
  Error: 'monitor.reason.Error',
  CreateContainerConfigError: 'monitor.reason.CreateContainerConfigError',
  NodeNotReady: 'monitor.reason.NodeNotReady',
  NodeUnreachable: 'monitor.reason.NodeUnreachable',
  MemoryPressure: 'monitor.reason.MemoryPressure',
  DiskPressure: 'monitor.reason.DiskPressure',
  PIDPressure: 'monitor.reason.PIDPressure',
  NodeCordoned: 'monitor.reason.NodeCordoned',
} as const;

/** isNode reports whether a problem is of the server itself. */
const isNode = (p: Problem) => !p.target.pod && !!p.target.node;

/** ProblemsList shows and filters a site's problems; onFix is offered for pods run by a workload. */
export function ProblemsList({
  site,
  unavailable,
  onFix,
}: {
  site: Site;
  unavailable: boolean;
  onFix?: (problem: Problem) => void;
}) {
  const { t, date } = useI18n();
  const [search, setSearch] = useState('');
  const [severity, setSeverity] = useState<Severity | ''>('');
  const [namespace, setNamespace] = useState('');
  const problems = site.messages;
  const namespaces = useMemo(
    () =>
      Array.from(new Set(problems.map((p) => p.target.namespace)))
        .filter(Boolean)
        .sort(),
    [problems],
  );
  // A problem is shown when it matches the severity, the namespace and the search text.
  const filtered = problems.filter(
    (p) =>
      (!severity || p.severity === severity) &&
      (!namespace || p.target.namespace === namespace) &&
      [
        p.target.namespace,
        p.target.pod,
        p.target.container,
        p.target.node,
        p.reason,
        p.title,
        p.text,
      ]
        .join(' ')
        .toLocaleLowerCase()
        .includes(search.toLocaleLowerCase()),
  );
  // Without fresh data, "no problems" cannot be claimed.
  const unknown = unavailable || site.stale || neverObserved(site.last_success);
  return (
    <section className="panel">
      <div className="panel-header">
        <h2 className="section-title">
          <CircleAlert size={18} aria-hidden="true" /> {t('monitor.problems')}
        </h2>
        <span className="badge badge-neutral">{problems.length}</span>
      </div>
      <div className="toolbar">
        <label className="field search-field">
          <span className="muted">
            <Search size={14} aria-hidden="true" /> {t('monitor.search')}
          </span>
          <input className="input" value={search} onChange={(e) => setSearch(e.target.value)} />
        </label>
        <label className="field">
          <span>{t('monitor.severity')}</span>
          <select
            className="input"
            value={severity}
            onChange={(e) => setSeverity(e.target.value as Severity | '')}
          >
            <option value="">{t('monitor.all')}</option>
            <option value="ERROR">{t('monitor.error')}</option>
            <option value="WARNING">{t('monitor.warning')}</option>
            <option value="INFO">{t('monitor.info')}</option>
          </select>
        </label>
        <label className="field">
          <span>{t('monitor.namespace')}</span>
          <select
            className="input"
            value={namespace}
            onChange={(e) => setNamespace(e.target.value)}
          >
            <option value="">{t('monitor.all')}</option>
            {/* The chosen namespace stays selectable after its problems are gone. */}
            {namespace && !namespaces.includes(namespace) && (
              <option value={namespace}>{namespace}</option>
            )}
            {namespaces.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
        </label>
      </div>
      {!filtered.length ? (
        <div className="empty-state">
          {!problems.length
            ? t(unknown ? 'monitor.noProblemsUnknown' : 'monitor.noProblems')
            : t('monitor.noMatches')}
        </div>
      ) : (
        <div className="stack">
          {filtered.map((p) => {
            const reason = reasonKeys[p.reason as keyof typeof reasonKeys];
            return (
              <article className="problem-card" key={p.key}>
                <div className="row">
                  <span
                    className={`badge badge-${p.severity === 'ERROR' ? 'error' : p.severity === 'WARNING' ? 'warning' : 'neutral'}`}
                  >
                    {t(
                      p.severity === 'ERROR'
                        ? 'monitor.error'
                        : p.severity === 'WARNING'
                          ? 'monitor.warning'
                          : 'monitor.info',
                    )}
                  </span>
                  <h3>{reason ? t(reason) : p.reason}</h3>
                </div>
                <p className="mono problem-target">
                  {isNode(p) ? (
                    `${t('monitor.server')}: ${p.target.node}`
                  ) : (
                    <>
                      {p.target.namespace}/{p.target.pod}
                      {p.target.container ? ` · ${p.target.container}` : ''}
                      {p.target.node ? ` · ${t('monitor.server')} ${p.target.node}` : ''}
                    </>
                  )}
                </p>
                <p className="muted">
                  {t('monitor.observed')}: {date(p.time)}
                </p>
                {onFix && p.target.pod && p.target.ownerUID && (
                  <button className="button button-ghost" type="button" onClick={() => onFix(p)}>
                    {t('commands.fix')} · {p.target.ownerKind} {p.target.ownerName}
                  </button>
                )}
                <details className="technical">
                  <summary>{t('monitor.raw')}</summary>
                  <dl>
                    <dt>{t('monitor.reason')}</dt>
                    <dd className="mono">{p.reason}</dd>
                  </dl>
                  <pre>
                    {JSON.stringify(
                      {
                        title: p.title,
                        text: p.text,
                        rule: p.rule,
                        outcome: p.outcome,
                        target: p.target,
                        details: p.details,
                      },
                      null,
                      2,
                    )}
                  </pre>
                </details>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}
