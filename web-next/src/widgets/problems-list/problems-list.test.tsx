import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { ProblemsList } from './index';
afterEach(cleanup);
import { monitorRu, monitorEn } from '@/shared/i18n/locales/monitor';
import type { Site } from '@/shared/api/types';
let locale: 'ru' | 'en' = 'ru';
vi.mock('@/shared/i18n', () => ({
  useI18n: () => ({
    t: (key: keyof typeof monitorRu) => (locale === 'ru' ? monitorRu : monitorEn)[key],
    date: (v: string) => v,
  }),
}));
const site: Site = {
  id: 'a',
  last_contact: '2026-10-27T10:00:00Z',
  last_success: '2026-10-27T10:00:00Z',
  stale: false,
  last_error: '',
  messages: [
    {
      key: 'one',
      severity: 'ERROR',
      outcome: 'REPORTED',
      reason: 'OOMKilled',
      rule: 'memory',
      title: 'raw title',
      text: 'raw text',
      target: { namespace: 'prod', pod: 'billing', container: 'app' },
      time: '2026-10-27T10:00:00Z',
    },
    {
      key: 'two',
      severity: 'WARNING',
      outcome: 'REPORTED',
      reason: 'OtherReason',
      rule: 'general',
      title: 'other',
      text: 'other',
      target: { namespace: 'test', pod: 'search', container: 'app' },
      time: '2026-10-27T10:00:00Z',
    },
  ],
};
describe('ProblemsList', () => {
  it('preserves active filters while locale changes and retains raw text', () => {
    locale = 'ru';
    const { rerender } = render(<ProblemsList site={site} unavailable={false} />);
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'billing' } });
    expect(screen.queryByText('OtherReason')).toBeNull();
    locale = 'en';
    rerender(<ProblemsList site={site} unavailable={false} />);
    expect((screen.getByRole('textbox') as HTMLInputElement).value).toBe('billing');
    expect(screen.getByText(monitorEn['monitor.reason.OOMKilled'])).toBeTruthy();
    expect(screen.queryByText('OtherReason')).toBeNull();
    expect(screen.getByText(/raw title/)).toBeTruthy();
  });
  it('does not claim a healthy current state when the center is unavailable', () => {
    locale = 'en';
    render(<ProblemsList site={{ ...site, messages: [] }} unavailable />);
    expect(screen.getByText(monitorEn['monitor.noProblemsUnknown'])).toBeTruthy();
  });
});
