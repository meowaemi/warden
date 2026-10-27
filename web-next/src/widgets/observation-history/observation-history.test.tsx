import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { ObservationHistory } from './index';
afterEach(cleanup);
import { monitorEn } from '@/shared/i18n/locales/monitor';
const mock = vi.hoisted(() => ({
  entries: [
    {
      id: 'old',
      site_id: 'site-a',
      observed_at: '2026-10-27T10:00:00Z',
      collected: true,
      messages: [],
    },
  ],
  calls: [] as unknown[],
}));
vi.mock('@/shared/i18n', () => ({
  useI18n: () => ({ t: (key: keyof typeof monitorEn) => monitorEn[key], date: (v: string) => v }),
}));
vi.mock('@/shared/lib/session', () => ({ useSession: () => ({ connected: true, sessionKey: 1 }) }));
vi.mock('@/shared/api', () => ({
  useGetHistoryQuery: (args: unknown, options: unknown) => {
    mock.calls.push([args, options]);
    return { currentData: mock.entries, isFetching: false, isError: false, refetch: vi.fn() };
  },
}));
describe('ObservationHistory', () => {
  it('requires manual load per site and never displays another site response', () => {
    const { rerender } = render(<ObservationHistory siteID="site-a" />);
    fireEvent.click(screen.getByRole('button', { name: monitorEn['monitor.loadHistory'] }));
    expect(screen.getByText('2026-10-27T10:00:00Z')).toBeTruthy();
    rerender(<ObservationHistory siteID="site-b" />);
    expect(screen.queryByText('2026-10-27T10:00:00Z')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: monitorEn['monitor.loadHistory'] }));
    expect(screen.queryByText('2026-10-27T10:00:00Z')).toBeNull();
    expect(screen.getByText(monitorEn['monitor.noHistory'])).toBeTruthy();
  });
});
