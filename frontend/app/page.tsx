'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth-context';
import LandingPage from '@/features/landing/LandingPage';
import { Spinner } from '@/design-system';

// This route is the public marketing page. It used to also secretly render
// the dashboard by manually importing the (dashboard) layout + page — which
// collided with app/(dashboard)/page.tsx also resolving to "/" and broke the
// standalone production build. The dashboard now lives at its own real route
// (/chat), so this page just has to redirect signed-in users there.
export default function RootPage() {
  const { isAuthenticated, isLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      router.replace('/chat');
    }
  }, [isLoading, isAuthenticated, router]);

  if (isLoading || isAuthenticated) {
    return (
      <div
        style={{
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: 'var(--color-background)',
        }}
      >
        <Spinner size={32} color="var(--color-primary)" />
      </div>
    );
  }

  return <LandingPage />;
}
