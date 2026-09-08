'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from './ui';

const LINKS = [
  { href: '/', label: '一覧' },
  { href: '/calendar', label: 'カレンダー' },
  { href: '/volume', label: 'ボリューム' },
  { href: '/routine', label: 'プリセット' },
  { href: '/settings', label: '設定' },
];

export function Nav() {
  const pathname = usePathname();
  const isActive = (href: string) =>
    href === '/' ? pathname === '/' : pathname.startsWith(href);

  return (
    <header className="border-b border-gray-200 bg-white">
      <div className="mx-auto flex max-w-5xl flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3">
        <Link href="/" className="text-base font-bold text-brand-700">
          トレーニング記録
        </Link>
        <nav className="flex flex-wrap gap-1">
          {LINKS.map((l) => (
            <Link
              key={l.href}
              href={l.href}
              className={cn(
                'rounded-md px-3 py-1.5 text-sm font-medium transition',
                isActive(l.href)
                  ? 'bg-brand-50 text-brand-700'
                  : 'text-gray-600 hover:bg-gray-100',
              )}
            >
              {l.label}
            </Link>
          ))}
        </nav>
        <div className="ml-auto flex gap-2">
          <Link href="/sessions/new" className="btn-primary">
            ＋筋トレ
          </Link>
          <Link href="/spin/new" className="btn-secondary">
            ＋スピン
          </Link>
        </div>
      </div>
    </header>
  );
}
