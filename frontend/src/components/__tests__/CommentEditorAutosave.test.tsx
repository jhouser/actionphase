import { useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';
import { CommentEditor } from '../CommentEditor';
import { postCachingService } from '@/services/PostCachingService';

/**
 * Controlled harness mirroring how every real caller drives CommentEditor:
 * the parent owns the value and CommentEditor reports changes upward.
 */
function Harness({ autosaveRefId, initialValue = '' }: { autosaveRefId?: string; initialValue?: string }) {
  const [value, setValue] = useState(initialValue);
  return (
    <MemoryRouter>
      <CommentEditor
        value={value}
        onChange={setValue}
        autosaveRefId={autosaveRefId}
        textareaTestId="editor"
      />
    </MemoryRouter>
  );
}

describe('CommentEditor autosave', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('persists typed content under the supplied autosave id', async () => {
    const user = userEvent.setup();
    render(<Harness autosaveRefId="post-reply-42" />);

    await user.type(screen.getByTestId('editor'), 'draft text');

    expect(postCachingService.get('post-reply-42')).toBe('draft text');
  });

  it('restores a cached draft when mounted empty', () => {
    postCachingService.save('post-reply-42', 'recovered draft');

    render(<Harness autosaveRefId="post-reply-42" />);

    expect(screen.getByTestId('editor')).toHaveValue('recovered draft');
  });

  it('does not overwrite existing content with a cached draft', () => {
    // Edit forms mount with real content; a stale cache must never clobber it.
    postCachingService.save('post-reply-42', 'stale cached draft');

    render(<Harness autosaveRefId="post-reply-42" initialValue="the real content" />);

    expect(screen.getByTestId('editor')).toHaveValue('the real content');
  });

  it('does not persist anything when no autosave id is supplied', async () => {
    const user = userEvent.setup();
    render(<Harness />);

    await user.type(screen.getByTestId('editor'), 'ephemeral');

    expect(localStorage.length).toBe(0);
  });

  it('clears the cached draft when the editor is emptied', async () => {
    const user = userEvent.setup();
    postCachingService.save('post-reply-42', 'abc');

    render(<Harness autosaveRefId="post-reply-42" />);
    await user.clear(screen.getByTestId('editor'));

    expect(postCachingService.get('post-reply-42')).toBeUndefined();
  });
});
