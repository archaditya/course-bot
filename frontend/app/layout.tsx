import type { Metadata } from 'next';
import './globals.css';
import { Providers } from './providers';

export const metadata: Metadata = {
  title: {
    default: 'archadiLM — Chat with your learning material',
    template: '%s | archadiLM',
  },
  description:
    'Upload PDFs, videos, web pages, or text and have a natural conversation with your study material. Get cited, grounded answers with source links.',
  keywords: ['AI', 'learning', 'RAG', 'study', 'chat', 'PDF', 'video', 'NotebookLM'],
  openGraph: {
    type: 'website',
    title: 'archadiLM',
    description: 'Chat with your learning material — grounded answers with citations.',
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
      </head>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
