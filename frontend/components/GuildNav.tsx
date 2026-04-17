'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

export default function GuildNav({ id }: { id: string }) {
  const pathname = usePathname();
  const items = [
    { href: `/guilds/${id}`, label: 'Overview' },
    { href: `/guilds/${id}/logs`, label: 'Logs' },
    { href: `/guilds/${id}/automod`, label: 'Auto-mod' },
    { href: `/guilds/${id}/stats`, label: 'Stats' },
    { href: `/guilds/${id}/settings`, label: 'Settings' },
  ];
  return (
    <nav className="flex gap-2 border-b border-slate-800 mb-6 overflow-x-auto">
      {items.map((it) => {
        const active = pathname === it.href;
        return (
          <Link
            key={it.href}
            href={it.href}
            className={`px-3 py-2 text-sm whitespace-nowrap border-b-2 ${
              active
                ? 'border-brand-500 text-white'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            {it.label}
          </Link>
        );
      })}
    </nav>
  );
}
