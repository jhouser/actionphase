import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils';
import { CharacterSheet } from '../CharacterSheet';
import type { CharacterData } from '../../types/characters';

/**
 * The point of this file is that it exists at all.
 *
 * The real CharacterSheet used to be unmountable under jsdom: an effect copied the
 * character-data query into state, the query's `= []` default produced a new array
 * every render so the effect's dependency never compared equal, and the effect set a
 * freshly-built object so React's identical-state bail-out never fired either. The
 * result was render -> effect -> setState -> render until React's 50-update cap, and
 * it spun hardest while the query was unresolved — exactly where a component test
 * starts. Four integration tests mock this component with a props probe and cite the
 * hang as the reason.
 *
 * So the regression guard is not an assertion, it is the mount itself: if the loop
 * comes back, these tests hang or trip `Maximum update depth exceeded` rather than
 * failing on a specific expectation. The assertions below cover the behaviour the
 * fix had to preserve while removing the state copy — that saved values still
 * display, and that an in-progress edit is buffered separately from them.
 */

const CHARACTER_ID = 42;

function characterDataRow(overrides: Partial<CharacterData> = {}): CharacterData {
  return {
    id: 1,
    character_id: CHARACTER_ID,
    module_type: 'bio',
    field_name: 'background',
    field_value: 'A saved background.',
    field_type: 'text',
    is_public: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  } as CharacterData;
}

function mockCharacter(name = 'Test Character') {
  return {
    id: CHARACTER_ID,
    game_id: 1,
    name,
    character_type: 'player_character',
    status: 'approved',
    avatar_url: null,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  };
}

/** Installs the two queries the sheet issues on mount. */
function setupSheet(data: CharacterData[] = [characterDataRow()]) {
  server.use(
    http.get(`/api/v1/characters/${CHARACTER_ID}`, () =>
      HttpResponse.json(mockCharacter())
    ),
    http.get(`/api/v1/characters/${CHARACTER_ID}/data`, () =>
      HttpResponse.json(data)
    )
  );
}

describe('CharacterSheet', () => {
  let updateDepthError: string | null;
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    updateDepthError = null;
    // React reports the runaway loop by logging rather than throwing, so a test
    // that renders "correctly" would otherwise pass with the bug present.
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation((...args) => {
      const message = String(args[0] ?? '');
      if (message.includes('Maximum update depth exceeded')) {
        updateDepthError = message;
      }
    });
  });

  afterEach(() => {
    consoleErrorSpy.mockRestore();
  });

  it('mounts without a runaway render loop while the data query is unresolved', async () => {
    // Never resolves: the sheet holds `characterData` at its `= []` default, which
    // is the state that used to spin. Nothing is awaited on the response here.
    server.use(
      http.get(`/api/v1/characters/${CHARACTER_ID}`, () =>
        HttpResponse.json(mockCharacter())
      ),
      http.get(`/api/v1/characters/${CHARACTER_ID}/data`, () => new Promise(() => {}))
    );

    const { container } = renderWithProviders(
      <CharacterSheet characterId={CHARACTER_ID} canEdit />,
      { gameId: 1 }
    );

    // `isLoading` gates the sheet, so an unresolved data query shows the skeleton and
    // nothing else. That is the expected render; what is under test is that it holds
    // there quietly instead of re-rendering itself to React's update cap.
    await waitFor(() => {
      expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
    });
    // Give the loop time to run away if it is going to. Under the old code the cap
    // was tripped well inside this window.
    await new Promise(resolve => setTimeout(resolve, 100));

    expect(updateDepthError).toBeNull();
  });

  // The sheet used to print a module header above every tab, including the three
  // that render their own heading — so a stat tab read its name twice, separated
  // by a description that only restated it ("Skills" / "Character skills").
  it('does not repeat the tab name above a manager that heads itself', async () => {
    setupSheet();

    renderWithProviders(<CharacterSheet characterId={CHARACTER_ID} canEdit />, {
      gameId: 1,
    });

    const user = userEvent.setup();
    await user.click(await screen.findByRole('tab', { name: 'Skills' }));

    // The manager's own <h3> is the one that survives; the description that used
    // to sit under the duplicate goes with it.
    expect(await screen.findByTestId('skills-section')).toBeInTheDocument();
    expect(screen.getAllByRole('heading', { name: 'Skills' })).toHaveLength(1);
    expect(screen.queryByText('Character skills')).not.toBeInTheDocument();
    expect(updateDepthError).toBeNull();
  });

  it('renders saved field values from the character data query', async () => {
    setupSheet();

    renderWithProviders(<CharacterSheet characterId={CHARACTER_ID} canEdit />, {
      gameId: 1,
    });

    expect(await screen.findByText('A saved background.')).toBeInTheDocument();
    expect(updateDepthError).toBeNull();
  });

  it('buffers an in-progress edit without mutating the saved values', async () => {
    const user = userEvent.setup();
    setupSheet();

    renderWithProviders(<CharacterSheet characterId={CHARACTER_ID} canEdit />, {
      gameId: 1,
    });

    expect(await screen.findByText('A saved background.')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /edit/i }));

    // The editor opens seeded with the saved text, not empty — the old code got this
    // from the state copy, the new code seeds the draft explicitly.
    const editor = await screen.findByDisplayValue('A saved background.');
    await user.clear(editor);
    await user.type(editor, 'Edited but not saved.');

    // Cancelling discards the draft. This is what would regress if the draft were
    // written back into the derived map instead of held beside it.
    await user.click(screen.getByRole('button', { name: /cancel/i }));

    expect(await screen.findByText('A saved background.')).toBeInTheDocument();
    expect(screen.queryByText('Edited but not saved.')).not.toBeInTheDocument();
    expect(updateDepthError).toBeNull();
  });

  it('sends the buffered draft when the edit is saved', async () => {
    const user = userEvent.setup();
    setupSheet();

    let savedBody: Record<string, unknown> | null = null;
    server.use(
      http.post(`/api/v1/characters/${CHARACTER_ID}/data`, async ({ request }) => {
        savedBody = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(<CharacterSheet characterId={CHARACTER_ID} canEdit />, {
      gameId: 1,
    });

    expect(await screen.findByText('A saved background.')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /edit/i }));
    const editor = await screen.findByDisplayValue('A saved background.');
    await user.clear(editor);
    await user.type(editor, 'A rewritten background.');
    await user.click(screen.getByRole('button', { name: /save changes/i }));

    await waitFor(() => {
      expect(savedBody).not.toBeNull();
    });
    expect(savedBody).toMatchObject({
      module_type: 'bio',
      field_name: 'background',
      field_value: 'A rewritten background.',
    });
    expect(updateDepthError).toBeNull();
  });
});
