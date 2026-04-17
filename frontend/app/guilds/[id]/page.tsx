'use client';

import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import Shell from '@/components/Shell';
import GuildNav from '@/components/GuildNav';
import { api, Guild, GuildStats } from '@/lib/api';

export default function GuildOverview() {
  const { id } = useParams<{ id: string }>();
  const [guild, setGuild] = useState<Guild | null>(null);
  const [stats, setStats] = useState<GuildStats | null>(null);
  const [err, setErr] = useState('');

  useEffect(() => {
    if (!id) return;
    api<Guild>(`/api/guilds/${id}`).then(setGuild).catch((e) => setErr(e.message));
    api<GuildStats>(`/api/guilds/${id}/stats?period=30d`)
      .then(setStats)
      .catch(() => {});
  }, [id]);

  return (
    <Shell>
      <GuildNav id={id} />
      {err && <div className="text-red-400 mb-4">{err}</div>}
      {guild && (
        <>
          <h1 className="text-2xl font-semibold mb-1">{guild.name}</h1>
          <div className="text-sm text-slate-400 mb-6">ID: {guild.id}</div>
          <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
            <Stat label="Bans (30d)" value={stats?.total_bans ?? '-'} />
            <Stat label="Kicks (30d)" value={stats?.total_kicks ?? '-'} />
            <Stat label="Warns (30d)" value={stats?.total_warns ?? '-'} />
            <Stat label="Mutes (30d)" value={stats?.total_mutes ?? '-'} />
            <Stat label="Active rules" value={stats?.active_rules ?? '-'} />
          </div>
        </>
      )}
    </Shell>
  );
}

function Stat({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="card">
      <div className="text-xs text-slate-400">{label}</div>
      <div className="text-2xl font-semibold mt-1">{value}</div>
    </div>
  );
}
