'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { api, getToken, setToken, User } from '@/lib/api';

export default function Shell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!getToken()) {
      router.replace('/login');
      return;
    }
    api<User>('/api/me')
      .then(setUser)
      .catch(() => {
        setToken(null);
        router.replace('/login');
      })
      .finally(() => setLoading(false));
  }, [router]);

  function logout() {
    setToken(null);
    router.push('/login');
  }

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center text-slate-500">
        Loading...
      </div>
    );
  }
  if (!user) return null;

  return (
    <div className="min-h-screen">
      <header className="border-b border-slate-800 bg-slate-900/50 backdrop-blur sticky top-0 z-10">
        <div className="max-w-7xl mx-auto px-4 py-3 flex items-center justify-between">
          <Link href="/dashboard" className="font-semibold">
            Discord Mod Admin
          </Link>
          <div className="flex items-center gap-4 text-sm">
            <span className="text-slate-400">
              {user.email} · <span className="text-brand-500">{user.role}</span>
            </span>
            <button onClick={logout} className="btn-ghost py-1 px-3 text-xs">
              Logout
            </button>
          </div>
        </div>
      </header>
      <main className="max-w-7xl mx-auto px-4 py-6">{children}</main>
    </div>
  );
}
