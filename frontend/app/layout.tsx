import type { Metadata } from 'next';
import './globals.css';
import { Providers } from './providers';

export const metadata: Metadata = {
  title: {
    default: 'archadiLM — Chat with your content',
    template: '%s | archadiLM',
  },
  description:
    'Connect documents, web research, and notes into a unified knowledge graph. Chat with your context instantly with AI-powered citations.',
  keywords: ['AI', 'learning', 'RAG', 'study', 'chat', 'PDF', 'video', 'archadiLM'],
  openGraph: {
    type: 'website',
    title: 'archadiLM',
    description: 'Your Second Brain, Supercharged by AI.',
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="dark">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        <link
          href="https://fonts.googleapis.com/css2?family=Geist:wght@100..900&family=Inter:wght@100..900&family=JetBrains+Mono:wght@100..800&display=swap"
          rel="stylesheet"
        />
        <link
          href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&display=swap"
          rel="stylesheet"
        />
      </head>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
