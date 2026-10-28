'use client';
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useSyncExternalStore,
  type ReactNode,
} from 'react';
import { commonRu, commonEn } from './locales/common';
import { monitorRu, monitorEn } from './locales/monitor';
import { commandsRu, commandsEn } from './locales/commands';
import { shellRu, shellEn } from './locales/shell';
/** Locale is the UI language. */
export type Locale = 'ru' | 'en';
/** Theme is the color scheme. */
export type Theme = 'dark' | 'light';
// ru and en join the dictionaries of every area; en must have every ru key.
const ru = { ...commonRu, ...monitorRu, ...commandsRu, ...shellRu };
const en = { ...commonEn, ...monitorEn, ...commandsEn, ...shellEn } satisfies Record<
  keyof typeof ru,
  string
>;
/** TranslationKey is any key of the Russian dictionary; English must have the same keys. */
export type TranslationKey = keyof typeof ru;
/** I18n is what useI18n returns; date and relative print "—" for a missing date. */
interface I18n {
  locale: Locale;
  setLocale(locale: Locale): void;
  theme: Theme;
  setTheme(theme: Theme): void;
  t(key: TranslationKey): string;
  date(value: string): string;
  relative(value: string): string;
  timeZone: string;
}
// preference is "locale|theme|timeZone" in one string, so useSyncExternalStore
// can compare it. The server always renders the initial one.
const initialPreference = 'ru|dark|UTC';
let preference = initialPreference;
const listeners = new Set<() => void>();
/** subscribe calls listener whenever the preference changes. */
function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
/** publish applies language and theme to the page and, if persist, remembers them. */
function publish(locale: Locale, theme: Theme, timeZone: string, persist: boolean) {
  preference = `${locale}|${theme}|${timeZone}`;
  document.documentElement.lang = locale;
  document.title = (locale === 'ru' ? commonRu : commonEn)['common.documentTitle'];
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
  if (persist) {
    try {
      localStorage.setItem('warden.locale', locale);
      localStorage.setItem('warden.theme', theme);
    } catch {}
  }
  listeners.forEach((listener) => listener());
}
/** I18nContext is empty outside I18nProvider. */
const I18nContext = createContext<I18n | null>(null);
/** validDate parses a date; Go's zero time means none. */
function validDate(value: string): Date | null {
  if (!value || value.startsWith('0001-')) return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}
/** I18nProvider supplies translations, theme and date formatting in the viewer's time zone. */
export function I18nProvider({ children }: { children: ReactNode }) {
  const current = useSyncExternalStore(
    subscribe,
    () => preference,
    () => initialPreference,
  );
  const [locale, theme, timeZone] = current.split('|') as [Locale, Theme, string];
  // Saved choices and the time zone are read after mount, so the first client
  // render matches the server one.
  useEffect(() => {
    let locale: Locale = 'ru';
    let theme: Theme = 'dark';
    try {
      if (localStorage.getItem('warden.locale') === 'en') locale = 'en';
      if (localStorage.getItem('warden.theme') === 'light') theme = 'light';
    } catch {}
    publish(locale, theme, Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC', false);
  }, []);
  const value = useMemo<I18n>(
    () => ({
      locale,
      theme,
      timeZone,
      setLocale: (next) => publish(next, theme, timeZone, true),
      setTheme: (next) => publish(locale, next, timeZone, true),
      t: (key) => (locale === 'ru' ? ru : en)[key],
      date: (value) => {
        const d = validDate(value);
        return d
          ? new Intl.DateTimeFormat(locale === 'ru' ? 'ru-RU' : 'en-GB', {
              dateStyle: 'medium',
              timeStyle: 'medium',
              timeZone,
            }).format(d)
          : '—';
      },
      relative: (value) => {
        const d = validDate(value);
        if (!d) return '—';
        const seconds = (d.getTime() - Date.now()) / 1000;
        const [scale, unit]: [number, Intl.RelativeTimeFormatUnit] =
          Math.abs(seconds) >= 86400
            ? [86400, 'day']
            : Math.abs(seconds) >= 3600
              ? [3600, 'hour']
              : Math.abs(seconds) >= 60
                ? [60, 'minute']
                : [1, 'second'];
        return new Intl.RelativeTimeFormat(locale, { numeric: 'auto' }).format(
          Math.round(seconds / scale),
          unit,
        );
      },
    }),
    [locale, theme, timeZone],
  );
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}
/** useI18n returns translations, theme and date formatting. */
export function useI18n(): I18n {
  const value = useContext(I18nContext);
  if (!value) throw new Error('useI18n requires I18nProvider');
  return value;
}
