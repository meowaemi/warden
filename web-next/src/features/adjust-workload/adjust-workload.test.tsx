import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { Problem } from '../../shared/api/types';
import { AdjustWorkload } from './index';
import { draftFromProblem, emptyDraft, mebibytes, prepareAdjustment, type Draft } from './model';
/** state holds the mocked send; the API, session and i18n are mocked, t returns the key. */
const state = vi.hoisted(() => ({ send: vi.fn() }));
vi.mock('../../shared/api', () => ({
  useCreateCommandMutation: () => [state.send, { isLoading: false }],
}));
vi.mock('../../shared/lib/session', () => ({
  useSession: () => ({ sessionKey: 1, connected: true }),
}));
vi.mock('../../shared/i18n', () => ({
  useI18n: () => ({ t: (key: string) => key, date: (value: string) => value }),
}));
beforeEach(() => {
  cleanup();
  state.send.mockReset();
});

/** oom is an OOM kill of a pod run by the api Deployment. */
const oom: Problem = {
  key: 'k',
  severity: 'ERROR',
  outcome: 'REPORTED',
  reason: 'OOMKilled',
  rule: 'oom-limit',
  title: 't',
  text: 't',
  target: {
    namespace: 'prod',
    pod: 'api-7f-x',
    container: 'app',
    ownerKind: 'Deployment',
    ownerName: 'api',
    ownerUID: 'uid-api',
  },
  details: [{ key: 'Memory limit', value: '128Mi' }],
  time: '2026-10-28T10:00:00Z',
};
/** target is a draft aimed at that Deployment, with no change yet. */
const target: Draft = {
  ...emptyDraft,
  namespace: 'prod',
  name: 'api',
  uid: 'uid-api',
  container: 'app',
};

describe('workload adjustments', () => {
  it('fills the target from the failing pod and proposes twice the memory limit', () => {
    const d = draftFromProblem(oom)!;
    expect(d).toMatchObject({
      targetKind: 'Deployment',
      namespace: 'prod',
      name: 'api',
      uid: 'uid-api',
      container: 'app',
    });
    expect(d.resources.memory_limit).toBe('256Mi');
    expect(draftFromProblem({ ...oom, target: { ...oom.target, ownerKind: 'Job' } })).toBeNull();
    expect(draftFromProblem({ ...oom, target: { ...oom.target, ownerUID: undefined } })).toBeNull();
  });
  it('builds only complete commands', () => {
    const r = { ...emptyDraft.resources };
    expect(prepareAdjustment('a', { ...target, resources: r })).toBeNull();
    expect(
      prepareAdjustment('a', { ...target, resources: { ...r, memory_limit: 'lots' } }),
    ).toBeNull();
    expect(prepareAdjustment('a', { ...target, resources: { ...r, cpu_limit: '0' } })).toBeNull();
    expect(
      prepareAdjustment('a', { ...target, container: ' ', resources: { ...r, cpu_limit: '1' } }),
    ).toBeNull();
    const c = prepareAdjustment(
      'a',
      { ...target, resources: { ...r, cpu_request: ' 250m ', memory_limit: '1Gi' } },
      0,
      'id',
    )!;
    expect(c).toMatchObject({
      id: 'id',
      site_id: 'a',
      kind: 'set_resources',
      container: 'app',
      resources: { cpu_request: '250m', memory_limit: '1Gi' },
    });
    expect(c.replicas).toBeUndefined();
    expect(prepareAdjustment('a', { ...target, kind: 'scale', replicas: 3 })).toMatchObject({
      kind: 'scale',
      replicas: 3,
    });
    expect(prepareAdjustment('a', { ...target, kind: 'scale', replicas: 1001 })).toBeNull();
    expect(
      prepareAdjustment('a', { ...target, kind: 'scale', targetKind: 'DaemonSet', replicas: 1 }),
    ).toBeNull();
  });
  it('reads the slider position from Mi and Gi', () => {
    expect(mebibytes('512Mi')).toBe(512);
    expect(mebibytes('2Gi')).toBe(2048);
    expect(mebibytes('500M')).toBe(0);
  });
  it('takes a fix from a problem, but never replaces a request awaiting confirmation', async () => {
    state.send.mockImplementation(() => ({
      unwrap: () => Promise.reject({ status: 'FETCH_ERROR', message: 'connection lost' }),
    }));
    const view = render(<AdjustWorkload siteID="a" simulated={false} seed={null} />);
    view.rerender(<AdjustWorkload siteID="a" simulated={false} seed={draftFromProblem(oom)} />);
    expect((screen.getByDisplayValue('256Mi') as HTMLInputElement).value).toBe('256Mi');
    fireEvent.submit(screen.getByText('commands.adjust').closest('form')!);
    fireEvent.click(screen.getByText('commands.confirm'));
    await waitFor(() => expect(screen.getByText('commands.retry')).toBeTruthy());
    const original = state.send.mock.calls[0][0].command;
    expect(original).toMatchObject({
      kind: 'set_resources',
      target_uid: 'uid-api',
      resources: { memory_limit: '256Mi' },
    });

    const other = { ...draftFromProblem(oom)!, name: 'other' };
    view.rerender(<AdjustWorkload siteID="a" simulated={false} seed={other} />);
    expect(screen.queryByDisplayValue('other')).toBeNull();
    fireEvent.click(screen.getByText('commands.retry'));
    await waitFor(() => expect(state.send).toHaveBeenCalledTimes(2));
    expect(state.send.mock.calls[1][0].command).toBe(original);
  });
});
