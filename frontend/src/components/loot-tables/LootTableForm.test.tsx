import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { LootTableForm } from './LootTableForm';

// The form only needs the game id from context; the real provider pulls in the
// whole game fetch chain.
vi.mock('@/contexts/GameContext', () => ({
  useOptionalGameContext: () => ({ gameId: 1 }),
}));

vi.mock('@/lib/api', () => ({
  apiClient: {
    games: {
      getLootTableContents: vi.fn().mockResolvedValue({ data: [] }),
    },
  },
}));

const renderForm = (props: Partial<React.ComponentProps<typeof LootTableForm>> = {}) => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const onSubmit = vi.fn();
  render(
    <QueryClientProvider client={queryClient}>
      <LootTableForm
        onClose={vi.fn()}
        onSubmit={onSubmit}
        isSubmitting={false}
        {...props}
      />
    </QueryClientProvider>
  );
  return { onSubmit };
};

const submitButton = () => screen.getByRole('button', { name: /create loot table/i });

describe('LootTableForm validation', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('blocks submission when the table has no name', () => {
    const { onSubmit } = renderForm();

    expect(screen.getByText(/give the loot table a name/i)).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();

    fireEvent.click(submitButton());
    expect(onSubmit).not.toHaveBeenCalled();
  });

  // Regression: an empty loot table could be saved, and rolling on it made the
  // backend panic on rand.Intn(0). The API now rejects it with a 400; the form
  // should stop the GM from creating one in the first place.
  it('blocks submission when the table is named but has no items', () => {
    const { onSubmit } = renderForm();

    fireEvent.change(screen.getByLabelText(/table name/i), {
      target: { value: 'Treasure Chest' },
    });

    expect(
      screen.getByText(/add at least one item — an empty loot table cannot be rolled on/i)
    ).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();

    fireEvent.click(submitButton());
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('enables submission once the table has a name and an item', async () => {
    const { onSubmit } = renderForm();

    fireEvent.change(screen.getByLabelText(/table name/i), {
      target: { value: 'Treasure Chest' },
    });

    fireEvent.click(screen.getByRole('button', { name: /add loot table content/i }));

    fireEvent.change(await screen.findByLabelText(/item name/i), {
      target: { value: 'Gold Coins' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^add item$/i }));

    expect(submitButton()).toBeEnabled();

    fireEvent.click(submitButton());
    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'Treasure Chest',
        items: expect.arrayContaining([
          expect.objectContaining({ name: 'Gold Coins' }),
        ]),
      })
    );
  });
});
