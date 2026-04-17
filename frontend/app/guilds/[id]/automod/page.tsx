'use client';

import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import Shell from '@/components/Shell';
import GuildNav from '@/components/GuildNav';
import { api, AutoModRule } from '@/lib/api';

const RULE_TYPES = [
  { value: 'word_filter', label: 'Word filter' },
  { value: 'spam_detection', label: 'Spam detection' },
  { value: 'raid_protection', label: 'Raid protection' },
  { value: 'custom', label: 'Custom' },
];

export default function AutomodPage() {
  const { id } = useParams<{ id: string }>();
  const [rules, setRules] = useState<AutoModRule[]>([]);
  const [creating, setCreating] = useState(false);
  const [err, setErr] = useState('');

  async function reload() {
    const r = await api<AutoModRule[]>(`/api/guilds/${id}/automod`);
    setRules(r || []);
  }

  useEffect(() => {
    if (!id) return;
    reload().catch((e) => setErr(e.message));
  }, [id]);

  async function toggle(r: AutoModRule) {
    await api(`/api/guilds/${id}/automod/${r.id}`, {
      method: 'PUT',
      body: JSON.stringify({ enabled: !r.enabled }),
    });
    reload();
  }

  async function remove(r: AutoModRule) {
    if (!confirm(`Delete rule "${r.rule_type}"?`)) return;
    await api(`/api/guilds/${id}/automod/${r.id}`, { method: 'DELETE' });
    reload();
  }

  return (
    <Shell>
      <GuildNav id={id} />
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xl font-semibold">Auto-moderation rules</h2>
        <button className="btn-primary" onClick={() => setCreating(true)}>
          New rule
        </button>
      </div>
      {err && <div className="text-red-400 mb-4">{err}</div>}

      {creating && (
        <CreateRule
          guildId={id}
          onClose={() => setCreating(false)}
          onCreated={() => {
            setCreating(false);
            reload();
          }}
        />
      )}

      <div className="space-y-3">
        {rules.map((r) => (
          <div key={r.id} className="card flex items-start justify-between gap-4">
            <div className="flex-1">
              <div className="flex items-center gap-3">
                <span className="font-medium">
                  {RULE_TYPES.find((t) => t.value === r.rule_type)?.label ?? r.rule_type}
                </span>
                <span
                  className={`text-xs px-2 py-0.5 rounded ${
                    r.enabled ? 'bg-green-900 text-green-300' : 'bg-slate-800 text-slate-400'
                  }`}
                >
                  {r.enabled ? 'enabled' : 'disabled'}
                </span>
              </div>
              <pre className="mt-2 text-xs text-slate-400 whitespace-pre-wrap break-all">
                {JSON.stringify(r.config, null, 2)}
              </pre>
            </div>
            <div className="flex flex-col gap-2">
              <button className="btn-ghost text-xs" onClick={() => toggle(r)}>
                {r.enabled ? 'Disable' : 'Enable'}
              </button>
              <button className="btn-danger text-xs" onClick={() => remove(r)}>
                Delete
              </button>
            </div>
          </div>
        ))}
        {rules.length === 0 && (
          <div className="card text-slate-400 text-sm">No rules configured.</div>
        )}
      </div>
    </Shell>
  );
}

function CreateRule({
  guildId,
  onClose,
  onCreated,
}: {
  guildId: string;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [ruleType, setRuleType] = useState('word_filter');
  const [configText, setConfigText] = useState(configTemplate('word_filter'));
  const [err, setErr] = useState('');

  function onTypeChange(v: string) {
    setRuleType(v);
    setConfigText(configTemplate(v));
  }

  async function save() {
    setErr('');
    let config: any;
    try {
      config = JSON.parse(configText);
    } catch (e: any) {
      setErr('Invalid JSON: ' + e.message);
      return;
    }
    try {
      await api(`/api/guilds/${guildId}/automod`, {
        method: 'POST',
        body: JSON.stringify({ rule_type: ruleType, config, enabled: true }),
      });
      onCreated();
    } catch (e: any) {
      setErr(e.message);
    }
  }

  return (
    <div className="card mb-4 space-y-3">
      <h3 className="font-medium">New rule</h3>
      {err && <div className="text-red-400 text-sm">{err}</div>}
      <div>
        <label className="text-sm text-slate-400">Type</label>
        <select
          className="input mt-1"
          value={ruleType}
          onChange={(e) => onTypeChange(e.target.value)}
        >
          {RULE_TYPES.map((t) => (
            <option key={t.value} value={t.value}>
              {t.label}
            </option>
          ))}
        </select>
      </div>
      <div>
        <label className="text-sm text-slate-400">Config (JSON)</label>
        <textarea
          className="input mt-1 font-mono min-h-[180px]"
          value={configText}
          onChange={(e) => setConfigText(e.target.value)}
        />
      </div>
      <div className="flex gap-2 justify-end">
        <button className="btn-ghost" onClick={onClose}>
          Cancel
        </button>
        <button className="btn-primary" onClick={save}>
          Create
        </button>
      </div>
    </div>
  );
}

function configTemplate(type: string): string {
  switch (type) {
    case 'word_filter':
      return JSON.stringify(
        { words: ['badword'], patterns: [], action: 'delete' },
        null,
        2
      );
    case 'spam_detection':
      return JSON.stringify(
        { limit: 5, window_seconds: 10, action: 'mute' },
        null,
        2
      );
    case 'raid_protection':
      return JSON.stringify(
        { limit: 10, window_seconds: 30, alert_channel_id: '' },
        null,
        2
      );
    default:
      return JSON.stringify({ action: 'delete' }, null, 2);
  }
}
