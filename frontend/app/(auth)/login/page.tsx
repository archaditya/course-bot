'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation } from '@tanstack/react-query';
import { apiLogin, ApiError } from '@/lib/api';
import { useAuth } from '@/lib/auth-context';
import { Button, Input } from '@/design-system';

export default function LoginPage() {
  const router = useRouter();
  const { login } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const { mutate, isPending, error } = useMutation({
    mutationFn: () => apiLogin(email, password),
    onSuccess: (data) => {
      login(data);
      router.push('/');
    },
  });

  const errMsg = error instanceof ApiError ? error.message : error ? 'Login failed. Please try again.' : '';

  return (
    <form
      onSubmit={(e) => { e.preventDefault(); mutate(); }}
      style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-5)' }}
    >
      <div>
        <h1 style={{
          fontFamily: 'var(--font-display)',
          fontSize: 'var(--text-2xl)',
          fontWeight: 700,
          marginBottom: 'var(--space-1)',
        }}>
          Welcome back
        </h1>
        <p style={{ color: 'var(--color-ink-muted)', fontSize: 'var(--text-sm)' }}>
          Sign in to your study room
        </p>
      </div>

      <Input
        id="email"
        type="email"
        label="Email"
        autoComplete="email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        required
        placeholder="you@example.com"
      />

      <Input
        id="password"
        type="password"
        label="Password"
        autoComplete="current-password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        required
        placeholder="••••••••"
        error={errMsg}
      />

      <Button type="submit" id="btn-login" loading={isPending} style={{ width: '100%' }}>
        Log in
      </Button>

      <p style={{
        textAlign: 'center',
        fontSize: 'var(--text-sm)',
        color: 'var(--color-ink-muted)',
      }}>
        Don't have an account?{' '}
        <Link href="/signup" style={{ color: 'var(--color-accent)', fontWeight: 500 }}>
          Sign up
        </Link>
      </p>
    </form>
  );
}
