import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueueRestart } from './index';
import { prepareCommand, definitiveRejection } from './model';
const state = vi.hoisted(() => ({ language: 'ru', send: vi.fn() }));
vi.mock('../../shared/api', () => ({
  useCreateCommandMutation: () => [state.send, { isLoading: false }],
}));
vi.mock('../../shared/lib/session', () => ({
  useSession: () => ({ sessionKey: 1, connected: true }),
}));
vi.mock('../../shared/i18n', () => ({
  useI18n: () => ({
    t: (key: string) => `${state.language}:${key}`,
    date: (value: string) => value,
  }),
}));
beforeEach(() => {
  cleanup();
  state.language = 'ru';
  state.send.mockReset();
});
describe('restart requests', () => {
  it('requires explicit confirmation and preserves pending target/id/expiry across site and language changes', async () => {
    state.send.mockImplementation(() => ({
      unwrap: () => Promise.reject({ status: 'FETCH_ERROR', message: 'connection lost' }),
    }));
    const view = render(<QueueRestart siteID="demo-site" simulated />);
    fireEvent.click(screen.getByText('ru:commands.fillDemo'));
    fireEvent.submit(screen.getByText('ru:commands.queue').closest('form')!);
    expect(state.send).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText('ru:commands.confirm'));
    await waitFor(() => expect(screen.getByText('ru:commands.retry')).toBeTruthy());
    const original = state.send.mock.calls[0][0].command;
    expect(original.site_id).toBe('demo-site');
    state.language = 'en';
    view.rerender(<QueueRestart siteID="another-site" simulated={false} />);
    expect(screen.queryByText('en:commands.fillDemo')).toBeNull();
    fireEvent.click(screen.getByText('en:commands.retry'));
    await waitFor(() => expect(state.send).toHaveBeenCalledTimes(2));
    expect(state.send.mock.calls[1][0].command).toBe(original);
    expect(state.send.mock.calls[1][0].command).toEqual(original);
    expect(original.target_uid).toBe('demo-deployment-uid');
    expect(original.expires_at).toBeTruthy();
  });
  it('validates limits and distinguishes ambiguous failures from definitive rejection', () => {
    expect(
      prepareCommand({ siteID: 'a', namespace: 'demo', name: 'app', uid: '', ttl: 10 }),
    ).toBeNull();
    expect(
      prepareCommand({ siteID: 'a', namespace: 'demo', name: 'app', uid: 'uid', ttl: 1441 }),
    ).toBeNull();
    expect(definitiveRejection({ status: 'FETCH_ERROR' })).toBe(false);
    expect(definitiveRejection({ status: 408 })).toBe(false);
    expect(definitiveRejection({ status: 500 })).toBe(false);
    expect(definitiveRejection({ status: 409 })).toBe(true);
  });
});
