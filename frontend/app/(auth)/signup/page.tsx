'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation } from '@tanstack/react-query';
import { apiSignup, apiLogin, ApiError } from '@/lib/api';
import { useAuth } from '@/lib/auth-context';
import { Button, Input } from '@/design-system';

export default function SignupPage() {
  const router = useRouter();
  const { login } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const { mutate, isPending, error } = useMutation({
    mutationFn: async () => {
      await apiSignup(email, password);
      const tokens = await apiLogin(email, password);
      return tokens;
    },
    onSuccess: (tokens) => {
      login(tokens);
      router.push('/');
    },
  });

  const errMsg = error instanceof ApiError ? error.message : error ? 'Signup failed. Please try again.' : '';

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
          Create your account
        </h1>
        <p style={{ color: 'var(--color-ink-muted)', fontSize: 'var(--text-sm)' }}>
          Set up your study room in 30 seconds
        </p>
      </div>

      <Input
        id="signup-email"
        type="email"
        label="Email"
        autoComplete="email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        required
        placeholder="you@example.com"
      />

      <Input
        id="signup-password"
        type="password"
        label="Password"
        autoComplete="new-password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        required
        placeholder="At least 8 characters"
        minLength={8}
        error={errMsg}
      />

      <Button type="submit" id="btn-signup" loading={isPending} style={{ width: '100%' }}>
        Create account
      </Button>

      <p style={{
        textAlign: 'center',
        fontSize: 'var(--text-sm)',
        color: 'var(--color-ink-muted)',
      }}>
        Already have an account?{' '}
        <Link href="/login" style={{ color: 'var(--color-accent)', fontWeight: 500 }}>
          Log in
        </Link>
      </p>
    </form>
  );
}
