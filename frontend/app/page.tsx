'use client';

import { useAuth } from '@/lib/auth-context';
import LandingPage from '@/features/landing/LandingPage';
import DashboardPage from '@/app/(dashboard)/page';
import DashboardLayout from '@/app/(dashboard)/layout';
import { Spinner } from '@/design-system';

export default function RootPage() {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: '#0b1326',
      }}>
        <Spinner size={32} color="var(--color-primary)" />
      </div>
    );
  }

  if (isAuthenticated) {
    return (
      <DashboardLayout>
        <DashboardPage />
      </DashboardLayout>
    );
  }

  return <LandingPage />;
}
