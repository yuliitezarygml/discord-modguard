'use client';

import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  Legend,
} from 'recharts';
import Shell from '@/components/Shell';
import GuildNav from '@/components/GuildNav';
import { api, GuildStats } from '@/lib/api';

const PERIODS = ['7d', '30d', '90d', 'all'];

export default function StatsPage() {
  const { id } = useParams<{ id: string }>();
  const [period, setPeriod] = useState('30d');
  const [stats, setStats] = useState<GuildStats | null>(null);

  useEffect(() => {
    if (!id) return;
    api<GuildStats>(`/api/guilds/${id}/stats?period=${period}`).then(setStats);
  }, [id, period]);

  return (
    <Shell>
      <GuildNav id={id} />
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xl font-semibold">Statistics</h2>
        <select
          className="input max-w-xs"
          value={period}
          onChange={(e) => setPeriod(e.target.value)}
        >
          {PERIODS.map((p) => (
            <option key={p} value={p}>
              {p}
            </option>
          ))}
        </select>
      </div>

      {stats && (
        <>
          <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
            <Stat label="Bans" value={stats.total_bans} />
            <Stat label="Kicks" value={stats.total_kicks} />
            <Stat label="Warns" value={stats.total_warns} />
            <Stat label="Mutes" value={stats.total_mutes} />
            <Stat label="Active rules" value={stats.active_rules} />
          </div>
          <div className="card h-80">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={stats.timeline || []}>
                <CartesianGrid stroke="#1e293b" />
                <XAxis dataKey="date" stroke="#64748b" />
                <YAxis stroke="#64748b" allowDecimals={false} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#0f172a',
                    border: '1px solid #1e293b',
                  }}
                />
                <Legend />
                <Line type="monotone" dataKey="bans" stroke="#ef4444" />
                <Line type="monotone" dataKey="kicks" stroke="#f59e0b" />
                <Line type="monotone" dataKey="warns" stroke="#6366f1" />
                <Line type="monotone" dataKey="mutes" stroke="#10b981" />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </>
      )}
    </Shell>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="card">
      <div className="text-xs text-slate-400">{label}</div>
      <div className="text-2xl font-semibold mt-1">{value}</div>
    </div>
  );
}
