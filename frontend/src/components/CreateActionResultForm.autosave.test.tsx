import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { CreateActionResultForm } from './CreateActionResultForm';
import { postCachingService } from '@/services/PostCachingService';

// CommentEditor calls useBlocker for its unsaved-changes guard, which requires
// a data router in context.
function renderInRouter(ui: React.ReactElement) {
  const router = createMemoryRouter([{ path: '/', element: ui }], { initialEntries: ['/'] });
  return render(<RouterProvider router={router} />);
}

vi.mock('../hooks/useActionResults', () => ({
  useCreateActionResult: () => ({ mutateAsync: vi.fn(), isPending: false, isError: false, isSuccess: false }),
  useCreateStagedResultChain: () => ({ mutateAsync: vi.fn(), isPending: false, isError: false, isSuccess: false }),
}));
vi.mock('../contexts/ToastContext', () => ({ useToast: () => ({ showWarning: vi.fn() }) }));

const ACTION_SUBMISSION_ID = 99;
const HEAD_KEY = `action-result-${ACTION_SUBMISSION_ID}`;

describe('CreateActionResultForm — cancel discards cached drafts', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('clears the head draft when cancelled', async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    postCachingService.save(HEAD_KEY, 'half-written result');

    renderInRouter(
      <CreateActionResultForm
        gameId={164}
        userId={7}
        userName="TestPlayer"
        actionSubmissionId={ACTION_SUBMISSION_ID}
        onCancel={onCancel}
      />
    );

    await user.click(screen.getByTestId('cancel-action-result'));

    expect(postCachingService.get(HEAD_KEY)).toBeUndefined();
    expect(onCancel).toHaveBeenCalled();
  });

  it('clears staged follow-up drafts too, not just the head', async () => {
    const user = userEvent.setup();
    postCachingService.save(HEAD_KEY, 'head');

    renderInRouter(
      <CreateActionResultForm
        gameId={164}
        userId={7}
        userName="TestPlayer"
        actionSubmissionId={ACTION_SUBMISSION_ID}
        onCancel={vi.fn()}
      />
    );

    // Add two follow-up parts, then cache content against their suffixed keys
    await user.click(screen.getByTestId('add-staged-part'));
    await user.click(screen.getByTestId('add-staged-part'));
    postCachingService.save(`${HEAD_KEY}-part-2`, 'part two');
    postCachingService.save(`${HEAD_KEY}-part-3`, 'part three');

    await user.click(screen.getByTestId('cancel-action-result'));

    // Orphaned part keys would resurface the next time this composer opens.
    expect(postCachingService.get(HEAD_KEY)).toBeUndefined();
    expect(postCachingService.get(`${HEAD_KEY}-part-2`)).toBeUndefined();
    expect(postCachingService.get(`${HEAD_KEY}-part-3`)).toBeUndefined();
  });

  it('renders no cancel control when the parent supplies no handler', () => {
    renderInRouter(<CreateActionResultForm gameId={164} userId={7} userName="TestPlayer" />);

    expect(screen.queryByTestId('cancel-action-result')).toBeNull();
  });
});
