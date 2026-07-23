'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation } from '@tanstack/react-query';
import { motion, AnimatePresence } from 'framer-motion';
import { apiSignup, apiLogin, ApiError } from '@/lib/api';
import { useAuth } from '@/lib/auth-context';

export default function SignupPage() {
  const router = useRouter();
  const { login } = useAuth();
  const [fullName, setFullName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const { mutate, isPending, error } = useMutation({
    mutationFn: async () => {
      await apiSignup(fullName, email, password);
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
    <div className="glass-card" style={{ borderRadius: '16px', padding: '24px' }}>
      {/* Tab Header */}
      <div
        style={{
          display: 'flex',
          gap: '4px',
          padding: '4px',
          background: 'var(--color-surface-container-low)',
          borderRadius: '10px',
          marginBottom: '24px',
        }}
      >
        <button
          onClick={() => router.push('/login')}
          style={{
            flex: 1,
            padding: '8px',
            borderRadius: '8px',
            border: 'none',
            cursor: 'pointer',
            fontFamily: 'var(--font-geist)',
            fontSize: '12px',
            fontWeight: 500,
            letterSpacing: '0.05em',
            background: 'transparent',
            color: 'var(--color-on-surface-variant)',
          }}
        >
          Sign In
        </button>
        <button
          style={{
            flex: 1,
            padding: '8px',
            borderRadius: '8px',
            border: 'none',
            fontFamily: 'var(--font-geist)',
            fontSize: '12px',
            fontWeight: 600,
            letterSpacing: '0.05em',
            background: 'var(--color-surface-container-high)',
            color: 'var(--color-primary)',
          }}
        >
          Create Account
        </button>
      </div>

      {/* Form */}
      <motion.form
        onSubmit={(e) => { e.preventDefault(); mutate(); }}
        style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3 }}
      >
        {/* Full Name */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
          <label
            htmlFor="fullname"
            style={{
              fontFamily: 'var(--font-geist)',
              fontSize: '11px',
              fontWeight: 500,
              color: 'var(--color-on-surface-variant)',
              letterSpacing: '0.05em',
              textTransform: 'uppercase',
              marginLeft: '4px',
            }}
          >
            Full Name
          </label>
          <div style={{ position: 'relative' }}>
            <span
              className="material-symbols-outlined"
              style={{
                position: 'absolute',
                left: '14px',
                top: '50%',
                transform: 'translateY(-50%)',
                color: 'var(--color-on-surface-variant)',
                fontSize: '18px',
                pointerEvents: 'none',
              }}
            >
              person
            </span>
            <input
              id="fullname"
              type="text"
              autoComplete="name"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              required
              placeholder="Aditya Sharma"
              className="input-glow"
              style={{
                width: '100%',
                background: 'var(--color-surface-container-lowest)',
                border: '1px solid var(--color-outline-variant)',
                borderRadius: '10px',
                padding: '12px 14px 12px 44px',
                fontFamily: 'var(--font-inter)',
                fontSize: '13px',
                color: 'var(--color-on-surface)',
                outline: 'none',
              }}
            />
          </div>
        </div>

        {/* Email */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
          <label
            htmlFor="signup-email"
            style={{
              fontFamily: 'var(--font-geist)',
              fontSize: '11px',
              fontWeight: 500,
              color: 'var(--color-on-surface-variant)',
              letterSpacing: '0.05em',
              textTransform: 'uppercase',
              marginLeft: '4px',
            }}
          >
            Email Address
          </label>
          <div style={{ position: 'relative' }}>
            <span
              className="material-symbols-outlined"
              style={{
                position: 'absolute',
                left: '14px',
                top: '50%',
                transform: 'translateY(-50%)',
                color: 'var(--color-on-surface-variant)',
                fontSize: '18px',
                pointerEvents: 'none',
              }}
            >
              alternate_email
            </span>
            <input
              id="signup-email"
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              placeholder="name@company.com"
              className="input-glow"
              style={{
                width: '100%',
                background: 'var(--color-surface-container-lowest)',
                border: '1px solid var(--color-outline-variant)',
                borderRadius: '10px',
                padding: '12px 14px 12px 44px',
                fontFamily: 'var(--font-inter)',
                fontSize: '13px',
                color: 'var(--color-on-surface)',
                outline: 'none',
              }}
            />
          </div>
        </div>

        {/* Password */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
          <label
            htmlFor="signup-password"
            style={{
              fontFamily: 'var(--font-geist)',
              fontSize: '11px',
              fontWeight: 500,
              color: 'var(--color-on-surface-variant)',
              letterSpacing: '0.05em',
              textTransform: 'uppercase',
              marginLeft: '4px',
            }}
          >
            Password
          </label>
          <div style={{ position: 'relative' }}>
            <span
              className="material-symbols-outlined"
              style={{
                position: 'absolute',
                left: '14px',
                top: '50%',
                transform: 'translateY(-50%)',
                color: 'var(--color-on-surface-variant)',
                fontSize: '18px',
                pointerEvents: 'none',
              }}
            >
              lock
            </span>
            <input
              id="signup-password"
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              placeholder="At least 8 characters"
              minLength={8}
              className="input-glow"
              style={{
                width: '100%',
                background: 'var(--color-surface-container-lowest)',
                border: `1px solid ${errMsg ? 'var(--color-error)' : 'var(--color-outline-variant)'}`,
                borderRadius: '10px',
                padding: '12px 14px 12px 44px',
                fontFamily: 'var(--font-inter)',
                fontSize: '13px',
                color: 'var(--color-on-surface)',
                outline: 'none',
              }}
            />
          </div>
          <AnimatePresence>
            {errMsg && (
              <motion.p
                initial={{ opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -4 }}
                style={{ color: 'var(--color-error)', fontSize: '12px', marginLeft: '4px' }}
              >
                {errMsg}
              </motion.p>
            )}
          </AnimatePresence>
        </div>

        {/* Submit */}
        <motion.button
          type="submit"
          id="btn-signup"
          disabled={isPending}
          whileHover={{ scale: 1.01 }}
          whileTap={{ scale: 0.97 }}
          style={{
            width: '100%',
            padding: '14px',
            marginTop: '8px',
            background: 'var(--color-primary)',
            color: 'var(--color-on-primary)',
            border: 'none',
            borderRadius: '10px',
            fontFamily: 'var(--font-geist)',
            fontSize: '16px',
            fontWeight: 600,
            cursor: isPending ? 'not-allowed' : 'pointer',
            opacity: isPending ? 0.7 : 1,
            boxShadow: '0 4px 20px rgba(192,193,255,0.2)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '8px',
          }}
        >
          {isPending ? 'Creating Account...' : 'Create Account'}
        </motion.button>
      </motion.form>

      <p style={{ textAlign: 'center', marginTop: '20px', fontFamily: 'var(--font-inter)', fontSize: '12px', color: 'var(--color-on-surface-variant)' }}>
        Already registered?{' '}
        <Link href="/login" style={{ color: 'var(--color-primary)', fontWeight: 600 }}>
          Sign in
        </Link>
      </p>
    </div>
  );
}
