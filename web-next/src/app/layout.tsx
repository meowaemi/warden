import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { Providers } from './providers';
import './globals.css';
/** metadata is the title until i18n sets it in the viewer's language. */
export const metadata: Metadata = {
  title: 'Warden — управление площадками',
  description: 'Мониторинг удалённых площадок и отложенные операции Kubernetes.',
};
/** RootLayout wraps every page in the app providers. */
export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="ru" data-theme="dark" suppressHydrationWarning>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
