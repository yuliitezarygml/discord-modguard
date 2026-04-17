'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { api, setToken } from '@/lib/api';

export default function RegisterPage() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [err, setErr] = useState('');
  const [loading, setLoading] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setErr('');
    if (password.length < 8) {
      setErr('Password must be at least 8 characters');
      return;
    }
    setLoading(true);
    try {
      const res = await api<{ token: string }>('/api/auth/register', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      });
      setToken(res.token);
      router.push('/dashboard');
    } catch (e: any) {
      setErr(e.message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <form onSubmit={submit} className="card w-full max-w-md space-y-4">
        <h1 className="text-2xl font-semibold">Create account</h1>
        <p className="text-sm text-slate-400">
          The first registered user becomes admin.
        </p>
        {err && <div className="text-red-400 text-sm">{err}</div>}
        <div>
          <label className="text-sm text-slate-400">Email</label>
          <input
            className="input mt-1"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </div>
        <div>
          <label className="text-sm text-slate-400">Password (min 8)</label>
          <input
            className="input mt-1"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </div>
        <button className="btn-primary w-full" disabled={loading}>
          {loading ? '...' : 'Register'}
        </button>
        <p className="text-sm text-slate-400 text-center">
          Have an account?{' '}
          <Link href="/login" className="text-brand-500 hover:underline">
            Sign in
          </Link>
        </p>
      </form>
    </div>
  );
}
