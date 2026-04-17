'use client';

import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import Shell from '@/components/Shell';
import GuildNav from '@/components/GuildNav';
import { api, Guild } from '@/lib/api';

export default function SettingsPage() {
  const { id } = useParams<{ id: string }>();
  const [guild, setGuild] = useState<Guild | null>(null);
  const [text, setText] = useState('{}');
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');

  useEffect(() => {
    if (!id) return;
    api<Guild>(`/api/guilds/${id}`).then((g) => {
      setGuild(g);
      setText(JSON.stringify(g.settings || {}, null, 2));
    });
  }, [id]);

  async function save() {
    setMsg('');
    setErr('');
    let settings: any;
    try {
      settings = JSON.parse(text);
    } catch (e: any) {
      setErr('Invalid JSON: ' + e.message);
      return;
    }
    try {
      await api(`/api/guilds/${id}/settings`, {
        method: 'PUT',
        body: JSON.stringify({ settings }),
      });
      setMsg('Saved');
    } catch (e: any) {
      setErr(e.message);
    }
  }

  return (
    <Shell>
      <GuildNav id={id} />
      <h2 className="text-xl font-semibold mb-4">Settings</h2>
      {guild && (
        <div className="card space-y-3 max-w-3xl">
          <div className="text-sm text-slate-400">
            Known keys:{' '}
            <code>warn_mute_threshold</code>, <code>warn_ban_threshold</code>
          </div>
          <textarea
            className="input font-mono min-h-[300px]"
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
          {err && <div className="text-red-400 text-sm">{err}</div>}
          {msg && <div className="text-green-400 text-sm">{msg}</div>}
          <div className="flex justify-end">
            <button className="btn-primary" onClick={save}>
              Save
            </button>
          </div>
        </div>
      )}
    </Shell>
  );
}
