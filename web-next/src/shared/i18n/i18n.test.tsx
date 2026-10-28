import { afterEach, expect, it } from 'vitest';
import { act, cleanup, renderHook } from '@testing-library/react';
import { I18nProvider, useI18n } from './index';
// Defaults, language and theme switching, and what is saved to localStorage.
afterEach(() => {
  cleanup();
  localStorage.clear();
});
it('uses Russian dark defaults, handles zero dates, and stores preferences only', () => {
  const { result } = renderHook(useI18n, { wrapper: I18nProvider });
  expect(result.current.locale).toBe('ru');
  expect(result.current.theme).toBe('dark');
  expect(result.current.date('0001-01-01T00:00:00Z')).toBe('—');
  act(() => result.current.setLocale('en'));
  act(() => result.current.setTheme('light'));
  expect(document.documentElement.lang).toBe('en');
  expect(document.title).toBe('Warden — site operations');
  expect(document.documentElement.dataset.theme).toBe('light');
  expect(result.current.t('common.refresh')).toBe('Refresh');
  expect(Object.keys(localStorage).sort()).toEqual(['warden.locale', 'warden.theme']);
  expect(result.current.date('2026-10-27T10:00:00Z')).not.toBe('—');
});
