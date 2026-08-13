import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import React from 'react';
import { CreateActionResultForm } from './CreateActionResultForm';

const mockCreateResult = vi.fn();
const mockCreateChain = vi.fn();

vi.mock('../hooks/useActionResults', () => ({
  useCreateActionResult: () => ({
    mutateAsync: mockCreateResult,
    isPending: false,
    isError: false,
    isSuccess: false,
  }),
  useCreateStagedResultChain: () => ({
    mutateAsync: mockCreateChain,
    isPending: false,
    isError: false,
    isSuccess: false,
  }),
}));

const mockShowWarning = vi.fn();
vi.mock('../contexts/ToastContext', () => ({
  useToast: () => ({ showWarning: mockShowWarning }),
}));

// CommentEditor is a rich editor; a plain textarea keeps these tests about the
// payload the form builds rather than the editor's internals.
vi.mock('./CommentEditor', () => ({
  CommentEditor: ({
    id,
    value,
    onChange,
    placeholder,
  }: {
    id?: string;
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
  }) => (
    <textarea
      data-testid={id ?? 'content'}
      aria-label={placeholder}
      value={value}
      onChange={e => onChange(e.target.value)}
    />
  ),
}));

const GAME_ID = 164;

function renderForm() {
  return render(
    <CreateActionResultForm gameId={GAME_ID} userId={7} userName="TestPlayer" />
  );
}

describe('CreateActionResultForm — staged chains', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCreateResult.mockResolvedValue({});
    mockCreateChain.mockResolvedValue({});
  });

  it('posts a single ordinary result when no follow-up is added', async () => {
    const user = userEvent.setup();
    renderForm();

    await user.type(screen.getByTestId('content'), 'A normal result.');
    await user.click(screen.getByRole('button', { name: /Create Draft Result/ }));

    await waitFor(() => expect(mockCreateResult).toHaveBeenCalledTimes(1));
    // The staged endpoint must not be touched by a GM who never opted in.
    expect(mockCreateChain).not.toHaveBeenCalled();
    expect(mockCreateResult).toHaveBeenCalledWith(
      expect.objectContaining({ content: 'A normal result.', is_published: false })
    );
  });

  it('builds a chain payload with a zero-delay head', async () => {
    const user = userEvent.setup();
    renderForm();

    await user.type(screen.getByTestId('content'), 'The sword whooshes...');
    await user.click(screen.getByTestId('add-staged-part'));
    await user.type(screen.getByTestId('staged-part-content-2'), '...and misses!');

    await user.click(screen.getByRole('button', { name: /Create Draft Result/ }));

    await waitFor(() => expect(mockCreateChain).toHaveBeenCalledTimes(1));
    expect(mockCreateResult).not.toHaveBeenCalled();

    const payload = mockCreateChain.mock.calls[0][0];
    expect(payload.parts).toEqual([
      { content: 'The sword whooshes...', delay_minutes: 0 },
      { content: '...and misses!', delay_minutes: 15 },
    ]);
    expect(payload.is_published).toBe(false);
  });

  it('sends the delay the GM picked', async () => {
    const user = userEvent.setup();
    renderForm();

    await user.type(screen.getByTestId('content'), 'Part one.');
    await user.click(screen.getByTestId('add-staged-part'));
    await user.type(screen.getByTestId('staged-part-content-2'), 'Part two.');
    await user.selectOptions(screen.getByTestId('staged-part-delay-2'), '30');

    await user.click(screen.getByRole('button', { name: /Create Draft Result/ }));

    await waitFor(() => expect(mockCreateChain).toHaveBeenCalledTimes(1));
    expect(mockCreateChain.mock.calls[0][0].parts[1].delay_minutes).toBe(30);
  });

  it('supports a three-part chain in order', async () => {
    const user = userEvent.setup();
    renderForm();

    await user.type(screen.getByTestId('content'), 'One.');
    await user.click(screen.getByTestId('add-staged-part'));
    await user.type(screen.getByTestId('staged-part-content-2'), 'Two.');
    await user.click(screen.getByTestId('add-staged-part'));
    await user.type(screen.getByTestId('staged-part-content-3'), 'Three.');

    await user.click(screen.getByRole('button', { name: /Create Draft Result/ }));

    await waitFor(() => expect(mockCreateChain).toHaveBeenCalledTimes(1));
    expect(mockCreateChain.mock.calls[0][0].parts.map((p: { content: string }) => p.content)).toEqual([
      'One.',
      'Two.',
      'Three.',
    ]);
  });

  it('removes a follow-up part, reverting to the single-result path', async () => {
    const user = userEvent.setup();
    renderForm();

    await user.type(screen.getByTestId('content'), 'Only this.');
    await user.click(screen.getByTestId('add-staged-part'));
    expect(screen.getByTestId('staged-part-2')).toBeInTheDocument();

    await user.click(screen.getByTestId('remove-staged-part-2'));
    expect(screen.queryByTestId('staged-part-2')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /Create Draft Result/ }));

    await waitFor(() => expect(mockCreateResult).toHaveBeenCalledTimes(1));
    expect(mockCreateChain).not.toHaveBeenCalled();
  });

  it('refuses to submit a chain with an empty part', async () => {
    const user = userEvent.setup();
    renderForm();

    await user.type(screen.getByTestId('content'), 'Part one.');
    await user.click(screen.getByTestId('add-staged-part'));
    // Part 2 left blank.

    await user.click(screen.getByRole('button', { name: /Create Draft Result/ }));

    expect(mockCreateChain).not.toHaveBeenCalled();
    expect(mockShowWarning).toHaveBeenCalledWith(
      'Every part needs content before you can create the chain'
    );
  });
});
