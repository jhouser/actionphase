import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, render, fireEvent, within } from '@testing-library/react';
import { FilterBar } from './FilterBar';
import type { GameState, ParticipationFilter, SortBy } from '../types/games';

describe('FilterBar', () => {
  const mockOnStatesChange = vi.fn();
  const mockOnParticipationChange = vi.fn();
  const mockOnHasOpenSpotsChange = vi.fn();
  const mockOnSortByChange = vi.fn();
  const mockOnClearFilters = vi.fn();

  const defaultProps = {
    selectedStates: [] as GameState[],
    participation: undefined as ParticipationFilter | undefined,
    hasOpenSpots: undefined as boolean | undefined,
    sortBy: 'recent_activity' as SortBy,
    availableStates: ['recruitment', 'in_progress', 'completed'] as GameState[],
    onStatesChange: mockOnStatesChange,
    onParticipationChange: mockOnParticipationChange,
    onHasOpenSpotsChange: mockOnHasOpenSpotsChange,
    onSortByChange: mockOnSortByChange,
    onClearFilters: mockOnClearFilters,
    filteredCount: 5,
    totalCount: 10,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Initial Rendering', () => {
    it('renders all participation filter buttons', () => {
      render(<FilterBar {...defaultProps} />);

      expect(screen.getByRole('button', { name: 'All Games' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'My Games' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Applied' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Not Joined' })).toBeInTheDocument();
    });

    it('renders state dropdown', () => {
      render(<FilterBar {...defaultProps} />);

      expect(screen.getByRole('button', { name: /State/i })).toBeInTheDocument();
    });

    it('renders has open spots toggle', () => {
      render(<FilterBar {...defaultProps} />);

      expect(screen.getByRole('button', { name: 'Has Open Spots' })).toBeInTheDocument();
    });

    it('renders sort dropdown', () => {
      render(<FilterBar {...defaultProps} />);

      const sortDropdown = screen.getByDisplayValue(/Sort: Recent Activity/i);
      expect(sortDropdown).toBeInTheDocument();
    });

    it('displays results count', () => {
      render(<FilterBar {...defaultProps} />);

      expect(screen.getByText('5')).toBeInTheDocument();
      expect(screen.getByText('10')).toBeInTheDocument();
      expect(screen.getByText(/Showing/i)).toBeInTheDocument();
    });

    it('does not show clear filters button when no filters active', () => {
      render(<FilterBar {...defaultProps} />);

      expect(screen.queryByRole('button', { name: 'Clear Filters' })).not.toBeInTheDocument();
    });
  });

  describe('Participation Filters', () => {
    it('highlights "All Games" button when no participation filter selected', () => {
      render(<FilterBar {...defaultProps} />);

      const allGamesButton = screen.getByRole('button', { name: 'All Games' });
      expect(allGamesButton).toHaveClass('bg-interactive-primary', 'text-content-inverse');
    });

    it('highlights "My Games" button when my_games filter selected', () => {
      render(<FilterBar {...defaultProps} participation="my_games" />);

      const myGamesButton = screen.getByRole('button', { name: 'My Games' });
      expect(myGamesButton).toHaveClass('bg-interactive-primary', 'text-content-inverse');
    });

    it('calls onParticipationChange with undefined when "All Games" clicked', () => {
      render(<FilterBar {...defaultProps} participation="my_games" />);

      fireEvent.click(screen.getByRole('button', { name: 'All Games' }));

      expect(mockOnParticipationChange).toHaveBeenCalledWith(undefined);
    });

    it('calls onParticipationChange with "my_games" when "My Games" clicked', () => {
      render(<FilterBar {...defaultProps} />);

      fireEvent.click(screen.getByRole('button', { name: 'My Games' }));

      expect(mockOnParticipationChange).toHaveBeenCalledWith('my_games');
    });

    it('calls onParticipationChange with "applied" when "Applied" clicked', () => {
      render(<FilterBar {...defaultProps} />);

      fireEvent.click(screen.getByRole('button', { name: 'Applied' }));

      expect(mockOnParticipationChange).toHaveBeenCalledWith('applied');
    });

    it('calls onParticipationChange with "not_joined" when "Not Joined" clicked', () => {
      render(<FilterBar {...defaultProps} />);

      fireEvent.click(screen.getByRole('button', { name: 'Not Joined' }));

      expect(mockOnParticipationChange).toHaveBeenCalledWith('not_joined');
    });
  });

  describe('State Dropdown', () => {
    it('shows badge with count when states are selected', () => {
      render(<FilterBar {...defaultProps} selectedStates={['recruitment', 'in_progress']} />);

      const stateButton = screen.getByRole('button', { name: /State/i });
      expect(within(stateButton).getByText('2')).toBeInTheDocument();
    });

    it('opens dropdown when state button clicked', () => {
      render(<FilterBar {...defaultProps} />);

      const stateButton = screen.getByRole('button', { name: /State/i });
      fireEvent.click(stateButton);

      // Check for state checkboxes
      expect(screen.getByLabelText('Recruiting Players')).toBeInTheDocument();
      expect(screen.getByLabelText('In Progress')).toBeInTheDocument();
      expect(screen.getByLabelText('Completed')).toBeInTheDocument();
    });

    it('checks selected states in dropdown', () => {
      render(<FilterBar {...defaultProps} selectedStates={['recruitment']} />);

      const stateButton = screen.getByRole('button', { name: /State/i });
      fireEvent.click(stateButton);

      const recruitmentCheckbox = screen.getByLabelText('Recruiting Players') as HTMLInputElement;
      expect(recruitmentCheckbox.checked).toBe(true);
    });

    it('calls onStatesChange when state checkbox toggled', () => {
      render(<FilterBar {...defaultProps} />);

      const stateButton = screen.getByRole('button', { name: /State/i });
      fireEvent.click(stateButton);

      const recruitmentCheckbox = screen.getByLabelText('Recruiting Players');
      fireEvent.click(recruitmentCheckbox);

      expect(mockOnStatesChange).toHaveBeenCalledWith(['recruitment']);
    });

    it('adds state when unchecked state is clicked', () => {
      render(<FilterBar {...defaultProps} selectedStates={['recruitment']} />);

      const stateButton = screen.getByRole('button', { name: /State/i });
      fireEvent.click(stateButton);

      const inProgressCheckbox = screen.getByLabelText('In Progress');
      fireEvent.click(inProgressCheckbox);

      expect(mockOnStatesChange).toHaveBeenCalledWith(['recruitment', 'in_progress']);
    });

    it('removes state when checked state is clicked', () => {
      render(<FilterBar {...defaultProps} selectedStates={['recruitment', 'in_progress']} />);

      const stateButton = screen.getByRole('button', { name: /State/i });
      fireEvent.click(stateButton);

      const recruitmentCheckbox = screen.getByLabelText('Recruiting Players');
      fireEvent.click(recruitmentCheckbox);

      expect(mockOnStatesChange).toHaveBeenCalledWith(['in_progress']);
    });
  });

  describe('Has Open Spots Toggle', () => {
    it('shows inactive state when hasOpenSpots is undefined', () => {
      render(<FilterBar {...defaultProps} />);

      const toggleButton = screen.getByRole('button', { name: 'Has Open Spots' });
      expect(toggleButton).not.toHaveClass('bg-indigo-50', 'border-indigo-300');
    });

    it('shows active state when hasOpenSpots is true', () => {
      render(<FilterBar {...defaultProps} hasOpenSpots={true} />);

      const toggleButton = screen.getByRole('button', { name: 'Has Open Spots' });
      expect(toggleButton).toHaveClass('bg-interactive-primary', 'text-content-inverse');
    });

    it('shows checkmark icon when hasOpenSpots is true', () => {
      render(<FilterBar {...defaultProps} hasOpenSpots={true} />);

      const toggleButton = screen.getByRole('button', { name: 'Has Open Spots' });
      const svg = toggleButton.querySelector('svg');
      expect(svg).toBeInTheDocument();
    });

    it('does not show checkmark icon when hasOpenSpots is false/undefined', () => {
      render(<FilterBar {...defaultProps} />);

      const toggleButton = screen.getByRole('button', { name: 'Has Open Spots' });
      const svg = toggleButton.querySelector('svg');
      expect(svg).not.toBeInTheDocument();
    });

    it('calls onHasOpenSpotsChange with true when clicked from undefined', () => {
      render(<FilterBar {...defaultProps} />);

      const toggleButton = screen.getByRole('button', { name: 'Has Open Spots' });
      fireEvent.click(toggleButton);

      expect(mockOnHasOpenSpotsChange).toHaveBeenCalledWith(true);
    });

    it('calls onHasOpenSpotsChange with undefined when clicked from true', () => {
      render(<FilterBar {...defaultProps} hasOpenSpots={true} />);

      const toggleButton = screen.getByRole('button', { name: 'Has Open Spots' });
      fireEvent.click(toggleButton);

      expect(mockOnHasOpenSpotsChange).toHaveBeenCalledWith(undefined);
    });
  });

  describe('Sort Dropdown', () => {
    it('displays current sort option', () => {
      render(<FilterBar {...defaultProps} sortBy="recent_activity" />);

      const sortSelect = screen.getByDisplayValue('Sort: Recent Activity');
      expect(sortSelect).toBeInTheDocument();
    });

    it('shows all sort options', () => {
      render(<FilterBar {...defaultProps} />);

      const sortSelect = screen.getByRole('combobox');
      const options = within(sortSelect as HTMLElement).getAllByRole('option');

      expect(options).toHaveLength(4);
      expect(options[0]).toHaveTextContent('Recent Activity');
      expect(options[1]).toHaveTextContent('Recently Created');
      expect(options[2]).toHaveTextContent('Starting Soon');
      expect(options[3]).toHaveTextContent('Alphabetical');
    });

    it('calls onSortByChange when option selected', () => {
      render(<FilterBar {...defaultProps} />);

      const sortSelect = screen.getByRole('combobox');
      fireEvent.change(sortSelect, { target: { value: 'alphabetical' } });

      expect(mockOnSortByChange).toHaveBeenCalledWith('alphabetical');
    });

    it('displays selected sort option correctly', () => {
      render(<FilterBar {...defaultProps} sortBy="start_date" />);

      const sortSelect = screen.getByDisplayValue('Sort: Starting Soon');
      expect(sortSelect).toBeInTheDocument();
    });
  });

  describe('Clear Filters Button', () => {
    it('shows clear button when states are selected', () => {
      render(<FilterBar {...defaultProps} selectedStates={['recruitment']} />);

      expect(screen.getByRole('button', { name: 'Clear Filters' })).toBeInTheDocument();
    });

    it('shows clear button when participation is selected', () => {
      render(<FilterBar {...defaultProps} participation="my_games" />);

      expect(screen.getByRole('button', { name: 'Clear Filters' })).toBeInTheDocument();
    });

    it('shows clear button when hasOpenSpots is selected', () => {
      render(<FilterBar {...defaultProps} hasOpenSpots={true} />);

      expect(screen.getByRole('button', { name: 'Clear Filters' })).toBeInTheDocument();
    });

    it('hides clear button when no filters are active', () => {
      render(<FilterBar {...defaultProps} />);

      expect(screen.queryByRole('button', { name: 'Clear Filters' })).not.toBeInTheDocument();
    });

    it('calls onClearFilters when clicked', () => {
      render(<FilterBar {...defaultProps} selectedStates={['recruitment']} />);

      const clearButton = screen.getByRole('button', { name: 'Clear Filters' });
      fireEvent.click(clearButton);

      expect(mockOnClearFilters).toHaveBeenCalledTimes(1);
    });
  });

  describe('Results Count', () => {
    it('displays filtered and total counts', () => {
      render(<FilterBar {...defaultProps} filteredCount={7} totalCount={20} />);

      expect(screen.getByText('7')).toBeInTheDocument();
      expect(screen.getByText('20')).toBeInTheDocument();
    });

    it('updates when counts change', () => {
      const { rerender } = render(<FilterBar {...defaultProps} filteredCount={5} totalCount={10} />);

      expect(screen.getByText('5')).toBeInTheDocument();

      rerender(<FilterBar {...defaultProps} filteredCount={3} totalCount={10} />);

      expect(screen.getByText('3')).toBeInTheDocument();
      expect(screen.queryByText('5')).not.toBeInTheDocument();
    });

    it('displays zero counts correctly', () => {
      render(<FilterBar {...defaultProps} filteredCount={0} totalCount={0} />);

      const zeros = screen.getAllByText('0');
      expect(zeros).toHaveLength(2);
    });
  });

  describe('Edge Cases', () => {
    it('handles empty available states', () => {
      render(<FilterBar {...defaultProps} availableStates={[]} />);

      const stateButton = screen.getByRole('button', { name: /State/i });
      fireEvent.click(stateButton);

      // Dropdown should still open but be empty
      const dropdown = document.querySelector('.absolute.z-10');
      expect(dropdown).toBeInTheDocument();
    });

    it('handles toggling dropdown multiple times', () => {
      render(<FilterBar {...defaultProps} />);

      const stateButton = screen.getByRole('button', { name: /State/i });

      // Open
      fireEvent.click(stateButton);
      expect(screen.getByLabelText('Recruiting Players')).toBeInTheDocument();

      // Close
      fireEvent.click(stateButton);
      expect(screen.queryByLabelText('Recruiting Players')).not.toBeInTheDocument();

      // Open again
      fireEvent.click(stateButton);
      expect(screen.getByLabelText('Recruiting Players')).toBeInTheDocument();
    });

  });

  describe('Integration', () => {
    it('handles multiple filter changes in sequence', () => {
      render(<FilterBar {...defaultProps} />);

      // Change participation
      fireEvent.click(screen.getByRole('button', { name: 'My Games' }));
      expect(mockOnParticipationChange).toHaveBeenCalledWith('my_games');

      // Toggle open spots
      fireEvent.click(screen.getByRole('button', { name: 'Has Open Spots' }));
      expect(mockOnHasOpenSpotsChange).toHaveBeenCalledWith(true);

      // Change sort
      const sortSelect = screen.getByRole('combobox');
      fireEvent.change(sortSelect, { target: { value: 'alphabetical' } });
      expect(mockOnSortByChange).toHaveBeenCalledWith('alphabetical');
    });

    it('shows clear button after applying multiple filters', () => {
      const { rerender } = render(<FilterBar {...defaultProps} />);

      expect(screen.queryByRole('button', { name: 'Clear Filters' })).not.toBeInTheDocument();

      rerender(
        <FilterBar
          {...defaultProps}
          selectedStates={['recruitment']}
          participation="my_games"
          hasOpenSpots={true}
        />
      );

      expect(screen.getByRole('button', { name: 'Clear Filters' })).toBeInTheDocument();
    });
  });

  describe('Community filter', () => {
    const communities = [
      { id: 1, name: 'Midnight Ravens' },
      { id: 2, name: 'Dawn Chorus' },
    ];
    const mockOnCommunityChange = vi.fn();

    // Absent by default so the community games page -- already scoped to one
    // community -- does not offer a control that navigates off itself.
    it('is hidden when no communities are supplied', () => {
      render(<FilterBar {...defaultProps} />);
      expect(screen.queryByTestId('community-filter')).not.toBeInTheDocument();
    });

    it('lists the communities plus an all-communities option', () => {
      render(
        <FilterBar
          {...defaultProps}
          communities={communities}
          onCommunityChange={mockOnCommunityChange}
        />
      );

      const select = screen.getByTestId('community-filter');
      expect(within(select).getByRole('option', { name: 'All communities' })).toBeInTheDocument();
      expect(within(select).getByRole('option', { name: 'Midnight Ravens' })).toBeInTheDocument();
    });

    it('reports a chosen community as a number, not a string', () => {
      render(
        <FilterBar
          {...defaultProps}
          communities={communities}
          onCommunityChange={mockOnCommunityChange}
        />
      );

      fireEvent.change(screen.getByTestId('community-filter'), { target: { value: '2' } });
      expect(mockOnCommunityChange).toHaveBeenCalledWith(2);
    });

    it('reports clearing as undefined so the filter is dropped', () => {
      render(
        <FilterBar
          {...defaultProps}
          communityId={2}
          communities={communities}
          onCommunityChange={mockOnCommunityChange}
        />
      );

      fireEvent.change(screen.getByTestId('community-filter'), { target: { value: '' } });
      expect(mockOnCommunityChange).toHaveBeenCalledWith(undefined);
    });

    // Without this the Clear Filters button stays hidden while a community
    // filter is active, leaving no way to undo it except editing the URL.
    it('counts as an active filter', () => {
      render(
        <FilterBar
          {...defaultProps}
          communityId={1}
          communities={communities}
          onCommunityChange={mockOnCommunityChange}
        />
      );

      expect(screen.getByRole('button', { name: 'Clear Filters' })).toBeInTheDocument();
    });
  });
});
