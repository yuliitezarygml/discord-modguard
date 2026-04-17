const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return window.localStorage.getItem('token');
}

export function setToken(t: string | null) {
  if (typeof window === 'undefined') return;
  if (t) window.localStorage.setItem('token', t);
  else window.localStorage.removeItem('token');
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, msg: string) {
    super(msg);
    this.status = status;
  }
}

export async function api<T = any>(
  path: string,
  opts: RequestInit = {}
): Promise<T> {
  const token = getToken();
  const headers = new Headers(opts.headers);
  headers.set('Content-Type', 'application/json');
  if (token) headers.set('Authorization', `Bearer ${token}`);

  const res = await fetch(`${API_URL}${path}`, { ...opts, headers });
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) {
    throw new ApiError(res.status, data?.error || `HTTP ${res.status}`);
  }
  return data as T;
}

export type User = {
  id: string;
  email: string;
  role: 'admin' | 'moderator';
};

export type Guild = {
  id: string;
  name: string;
  settings: Record<string, any>;
  member_count?: number;
  icon_url?: string;
  banner_url?: string;
};

export type ModerationLog = {
  id: string;
  guild_id: string;
  moderator_id: string;
  moderator_name: string;
  action_type: string;
  target_user_id: string;
  target_username: string;
  reason: string | null;
  duration_seconds: number | null;
  created_at: string;
};

export type AutoModRule = {
  id: string;
  guild_id: string;
  rule_type: 'word_filter' | 'spam_detection' | 'raid_protection' | 'custom';
  config: Record<string, any>;
  enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type GuildStats = {
  total_bans: number;
  total_kicks: number;
  total_warns: number;
  total_mutes: number;
  active_rules: number;
  timeline: Array<{
    date: string;
    bans: number;
    kicks: number;
    warns: number;
    mutes: number;
  }>;
};
