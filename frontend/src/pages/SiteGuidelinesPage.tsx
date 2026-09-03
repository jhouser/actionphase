import { useState, useEffect } from 'react';
import { MarkdownPreview } from '@/components/MarkdownPreview';

export const SiteGuidelinesPage = () => {
  const [content, setContent] = useState<string | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    fetch('/site-guidelines.md')
      .then((res) => {
        if (!res.ok) throw new Error('Failed to load');
        return res.text();
      })
      .then(setContent)
      .catch(() => setError(true));
  }, []);

  return (
    <div className="max-w-3xl mx-auto py-8 px-4 sm:px-6 lg:px-8">
      <div className="bg-surface-base shadow rounded-lg px-8 py-10">
        {error && (
          <p className="text-content-secondary">Could not load the site guidelines. Please try again later.</p>
        )}
        {!error && content === null && (
          <p className="text-content-secondary">Loading...</p>
        )}
        {!error && content !== null && (
          <MarkdownPreview content={content} fullWidth />
        )}
      </div>
    </div>
  );
};
