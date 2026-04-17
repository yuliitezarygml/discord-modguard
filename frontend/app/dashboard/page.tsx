'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { api, Guild } from '@/lib/api';
import Shell from '@/components/Shell';

export default function Dashboard() {
  const [guilds, setGuilds] = useState<Guild[]>([]);
  const [q, setQ] = useState('');
  const [err, setErr] = useState('');

  useEffect(() => {
    api<Guild[]>('/api/guilds')
      .then((gs) => setGuilds(gs || []))
      .catch((e) => setErr(e.message));
  }, []);

  const filtered = guilds.filter((g) =>
    g.name.toLowerCase().includes(q.toLowerCase())
  );

  return (
    <Shell>
      <div className="mb-6 flex items-center justify-between gap-4">
        <h1 className="text-2xl font-semibold">Guilds</h1>
        <input
          className="input max-w-xs"
          placeholder="Search..."
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
      </div>
      {err && <div className="text-red-400 mb-4">{err}</div>}
      {filtered.length === 0 ? (
        <div className="card text-slate-400 text-sm">
          No guilds yet — invite the bot to a server.
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map((g) => (
            <GuildCard key={g.id} guild={g} />
          ))}
        </div>
      )}
    </Shell>
  );
}

function GuildCard({ guild: g }: { guild: Guild }) {
  const initials = g.name
    .split(/\s+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase())
    .join('');

  return (
    <Link
      href={`/guilds/${g.id}`}
      className="group rounded-xl overflow-hidden border border-slate-800 hover:border-brand-500 transition bg-slate-900"
    >
      <div
        className="relative h-24 bg-gradient-to-br from-brand-600/40 to-slate-800"
        style={
          g.banner_url
            ? {
                backgroundImage: `url(${g.banner_url})`,
                backgroundSize: 'cover',
                backgroundPosition: 'center',
              }
            : undefined
        }
      >
        <div className="absolute inset-0 bg-gradient-to-t from-slate-900 via-slate-900/50 to-transparent" />
      </div>
      <div className="p-4 pt-0 -mt-8 relative">
        {g.icon_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={g.icon_url}
            alt={g.name}
            className="w-16 h-16 rounded-xl border-4 border-slate-900 bg-slate-800 object-cover"
          />
        ) : (
          <div className="w-16 h-16 rounded-xl border-4 border-slate-900 bg-slate-800 flex items-center justify-center text-lg font-semibold text-slate-300">
            {initials}
          </div>
        )}
        <div className="mt-3 text-lg font-medium">{g.name}</div>
        <div className="mt-1 text-xs text-slate-400">ID: {g.id}</div>
        {g.member_count !== undefined && (
          <div className="mt-0.5 text-xs text-slate-400">
            {g.member_count} members
          </div>
        )}
      </div>
    </Link>
  );
}
