import type { Metadata } from 'next';
import './globals.css';
import { Providers } from './providers';

export const metadata: Metadata = {
  title: {
    default: 'Course Assistant — Chat with your course material',
    template: '%s | Course Assistant',
  },
  description:
    'Upload course transcripts, slides, or videos and have a natural conversation with your study material. Get cited, grounded answers with timestamp links.',
  keywords: ['AI', 'learning', 'course', 'study', 'RAG', 'chat'],
  openGraph: {
    type: 'website',
    title: 'Course Assistant',
    description: 'Chat with your course material — grounded answers with citations.',
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
