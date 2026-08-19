import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SheetPanel } from '../SheetPanel';
import type { SheetItem } from '../../hooks/useCharacterSheetItems';

const items: SheetItem[] = [
  { id: 's1', name: 'Stealth', type: 'skill', metadata: 'Combat · Level 2' },
  { id: 'i1', name: 'Elvish Longbow', type: 'item', description: 'A fine bow', metadata: 'Weapon' },
  { id: 's2', name: 'Fire Bolt', type: 'skill', description: 'Deals fire damage', metadata: 'Evocation' },
];

describe('SheetPanel', () => {
  it('renders both group headings when items exist', () => {
    render(<SheetPanel items={items} onInsert={vi.fn()} />);
    expect(screen.getByText('Skills')).toBeInTheDocument();
    expect(screen.getByText('Inventory')).toBeInTheDocument();
  });

  it('renders item names', () => {
    render(<SheetPanel items={items} onInsert={vi.fn()} />);
    expect(screen.getByText('Fire Bolt')).toBeInTheDocument();
    expect(screen.getByText('Stealth')).toBeInTheDocument();
    expect(screen.getByText('Elvish Longbow')).toBeInTheDocument();
  });

  it('shows empty state when no items', () => {
    render(<SheetPanel items={[]} onInsert={vi.fn()} />);
    expect(screen.getByText(/no items on this character/i)).toBeInTheDocument();
  });

  it('calls onInsert with the clicked item', async () => {
    const user = userEvent.setup();
    const onInsert = vi.fn();
    render(<SheetPanel items={items} onInsert={onInsert} />);
    await user.click(screen.getByText('Stealth'));
    expect(onInsert).toHaveBeenCalledWith(items[0]);
  });

  it('filters items by name', async () => {
    const user = userEvent.setup();
    render(<SheetPanel items={items} onInsert={vi.fn()} />);
    const filterInput = screen.getByRole('textbox', { name: /filter/i });
    await user.type(filterInput, 'bolt');
    expect(screen.getByText('Fire Bolt')).toBeInTheDocument();
    expect(screen.queryByText('Stealth')).not.toBeInTheDocument();
  });

  it('shows no-match message when filter returns nothing', async () => {
    const user = userEvent.setup();
    render(<SheetPanel items={items} onInsert={vi.fn()} />);
    await user.type(screen.getByRole('textbox', { name: /filter/i }), 'xyzzy');
    expect(screen.getByText(/no items match/i)).toBeInTheDocument();
  });

  it('shows character name when provided', () => {
    render(<SheetPanel items={items} onInsert={vi.fn()} characterName="Aria the Swift" />);
    expect(screen.getByText('Aria the Swift')).toBeInTheDocument();
  });

  it('hides groups that have no items', () => {
    const skillsOnly = items.filter((i) => i.type === 'skill');
    render(<SheetPanel items={skillsOnly} onInsert={vi.fn()} />);
    expect(screen.getByText('Skills')).toBeInTheDocument();
    expect(screen.queryByText('Inventory')).not.toBeInTheDocument();
  });
});
