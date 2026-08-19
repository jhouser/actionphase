import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useGameForm, gameToFormData } from '../useGameForm';
import type { GameWithDetails } from '../../types/games';

// The banner hooks are React Query mutations and irrelevant to label handling;
// stubbing them keeps this test off the network and out of a QueryClientProvider.
vi.mock('../useGameBanner', () => ({
  useUploadGameBanner: () => ({ mutateAsync: vi.fn() }),
  useDeleteGameBanner: () => ({ mutateAsync: vi.fn() }),
}));

function baseGame(overrides: Partial<GameWithDetails> = {}): GameWithDetails {
  return {
    id: 1,
    title: 'A Game',
    description: 'A description long enough to pass validation.',
    gm_user_id: 1,
    state: 'in_progress',
    current_players: 2,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  } as GameWithDetails;
}

/** Fills the fields buildApiPayload requires, leaving labels to the test. */
function fillRequired(result: { current: ReturnType<typeof useGameForm> }) {
  act(() => {
    result.current.handleChange('title', 'A Game');
    result.current.handleChange('description', 'A description long enough to pass.');
  });
}

describe('gameToFormData — character sheet labels', () => {
  it('hydrates each overridden label into its own box', () => {
    const form = gameToFormData(
      baseGame({ character_sheet: { labels: { skills: 'Playbook', numbers: 'Stress' } } })
    );
    expect(form.sheet_label_skills).toBe('Playbook');
    expect(form.sheet_label_numbers).toBe('Stress');
  });

  it('leaves a box empty when that label was never overridden', () => {
    // Empty, NOT prefilled with the default: a prefilled box would send the
    // default back as though the GM had chosen it, turning every edit of an
    // unrelated field into a permanent label override.
    const form = gameToFormData(
      baseGame({ character_sheet: { labels: { skills: 'Playbook' } } })
    );
    expect(form.sheet_label_inventory).toBe('');
    expect(form.sheet_label_numbers).toBe('');
  });

  it('leaves all boxes empty for a game with no config', () => {
    const form = gameToFormData(baseGame());
    expect(form.sheet_label_skills).toBe('');
    expect(form.sheet_label_inventory).toBe('');
    expect(form.sheet_label_numbers).toBe('');
  });
});

describe('useGameForm buildApiPayload — character sheet labels', () => {
  it('omits character_sheet entirely when no label is set', () => {
    const { result } = renderHook(() => useGameForm());
    fillRequired(result);
    const { payload, error } = result.current.buildApiPayload();
    expect(error).toBeNull();
    expect(payload?.character_sheet).toBeUndefined();
  });

  it('sends only the labels that were actually filled in', () => {
    const { result } = renderHook(() => useGameForm());
    fillRequired(result);
    act(() => {
      result.current.handleChange('sheet_label_numbers', 'Stress');
    });
    const { payload } = result.current.buildApiPayload();
    // Sparse on the wire: the untouched tabs must not appear at all, so the
    // backend stores an override only where the GM made one.
    expect(payload?.character_sheet).toEqual({ labels: { numbers: 'Stress' } });
  });

  it('drops a whitespace-only label rather than sending ""', () => {
    // The backend would also tolerate "" — it trims and treats whitespace-only
    // as "no override" rather than an error (verified against the API: it
    // stores null and returns 201). Omitting it here anyway keeps one spelling
    // of "no override" on the wire instead of two that happen to coincide, so
    // a stored config never contains a key the GM did not actually set.
    const { result } = renderHook(() => useGameForm());
    fillRequired(result);
    act(() => {
      result.current.handleChange('sheet_label_skills', '   ');
    });
    expect(result.current.buildApiPayload().payload?.character_sheet).toBeUndefined();
  });

  it('trims a label before sending it', () => {
    const { result } = renderHook(() => useGameForm());
    fillRequired(result);
    act(() => {
      result.current.handleChange('sheet_label_inventory', '  Load  ');
    });
    expect(result.current.buildApiPayload().payload?.character_sheet).toEqual({
      labels: { inventory: 'Load' },
    });
  });

  it('round-trips an existing game unchanged', () => {
    // Opening the edit form and saving without touching anything must send back
    // exactly what was stored — not the defaults, and not an empty object.
    const { result } = renderHook(() =>
      useGameForm(
        baseGame({
          character_sheet: { labels: { skills: 'Playbook', inventory: 'Load', numbers: 'Stress' } },
        })
      )
    );
    expect(result.current.buildApiPayload().payload?.character_sheet).toEqual({
      labels: { skills: 'Playbook', inventory: 'Load', numbers: 'Stress' },
    });
  });

  it('clears an override when the GM empties the box', () => {
    // Emptying a box means "go back to the default", which on the wire is the
    // key being absent. Sending "" would land in the same place server-side, but
    // only because the backend trims it away; not relying on that keeps the
    // clearing path independent of server-side leniency.
    const { result } = renderHook(() =>
      useGameForm(baseGame({ character_sheet: { labels: { skills: 'Playbook' } } }))
    );
    act(() => {
      result.current.handleChange('sheet_label_skills', '');
    });
    expect(result.current.buildApiPayload().payload?.character_sheet).toBeUndefined();
  });
});
