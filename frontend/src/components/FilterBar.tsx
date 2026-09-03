import { useState } from 'react';
import {
  GAME_STATE_LABELS,
  SORT_BY_LABELS,
  type GameState,
  type ParticipationFilter,
  type SortBy,
} from '../types/games';
import { Button, Select, Checkbox } from './ui';

interface FilterBarProps {
  // Current filter values
  selectedStates: GameState[];
  participation?: ParticipationFilter;
  hasOpenSpots?: boolean;
  communityId?: number;
  sortBy: SortBy;

  // Available options
  availableStates: GameState[];
  /**
   * Communities to offer. Omit to hide the picker entirely -- the community
   * games page is already scoped to one, so a picker there would be a way to
   * navigate off the page you are on.
   */
  communities?: { id: number; name: string }[];

  // Callbacks
  onStatesChange: (states: GameState[]) => void;
  onParticipationChange: (participation?: ParticipationFilter) => void;
  onHasOpenSpotsChange: (hasOpenSpots?: boolean) => void;
  onCommunityChange?: (communityId?: number) => void;
  onSortByChange: (sortBy: SortBy) => void;
  onClearFilters: () => void;

  // Metadata
  filteredCount: number;
  totalCount: number;
}

export function FilterBar({
  selectedStates,
  participation,
  hasOpenSpots,
  communityId,
  sortBy,
  availableStates,
  communities,
  onStatesChange,
  onParticipationChange,
  onHasOpenSpotsChange,
  onCommunityChange,
  onSortByChange,
  onClearFilters,
  filteredCount,
  totalCount,
}: FilterBarProps) {
  const [showStateDropdown, setShowStateDropdown] = useState(false);

  const hasActiveFilters =
    selectedStates.length > 0 ||
    participation !== undefined ||
    hasOpenSpots !== undefined ||
    communityId !== undefined;

  const handleStateToggle = (state: GameState) => {
    if (selectedStates.includes(state)) {
      onStatesChange(selectedStates.filter((s) => s !== state));
    } else {
      onStatesChange([...selectedStates, state]);
    }
  };

  return (
    <div className="surface-base border-b border-theme-default">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
        {/* Top row: Quick filters */}
        <div className="flex flex-wrap items-center gap-2 mb-4">
          {/* Participation quick filters */}
          <div className="flex gap-2">
            <Button
              variant={!participation || participation === null ? "primary" : "outline"}
              onClick={() => onParticipationChange(undefined)}
              size="sm"
            >
              All Games
            </Button>
            <Button
              variant={participation === 'my_games' ? "primary" : "outline"}
              onClick={() => onParticipationChange('my_games')}
              size="sm"
            >
              My Games
            </Button>
            <Button
              variant={participation === 'applied' ? "primary" : "outline"}
              onClick={() => onParticipationChange('applied')}
              size="sm"
            >
              Applied
            </Button>
            <Button
              variant={participation === 'not_joined' ? "primary" : "outline"}
              onClick={() => onParticipationChange('not_joined')}
              size="sm"
            >
              Not Joined
            </Button>
          </div>

          {/* Spacer */}
          <div className="flex-1" />

          {/* Results count */}
          <div className="text-sm text-content-secondary">
            Showing <span className="font-medium text-content-primary">{filteredCount}</span> of{' '}
            <span className="font-medium text-content-primary">{totalCount}</span> games
          </div>
        </div>

        {/* Bottom row: Advanced filters */}
        <div className="flex flex-wrap items-center gap-3">
          {/* State filter dropdown */}
          <div className="relative">
            <Button
              variant="outline"
              onClick={() => setShowStateDropdown(!showStateDropdown)}
              size="sm"
              className="flex items-center gap-2"
            >
              <span>State</span>
              {selectedStates.length > 0 && (
                <span className="bg-interactive-primary-subtle text-interactive-primary px-2 py-0.5 rounded-full text-xs">
                  {selectedStates.length}
                </span>
              )}
              <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
              </svg>
            </Button>
            {showStateDropdown && (
              <div className="absolute z-10 mt-2 w-56 surface-base border border-theme-default rounded-md shadow-lg">
                <div className="py-1 max-h-60 overflow-auto">
                  {availableStates.map((state) => (
                    <div
                      key={state}
                      className="px-4 py-2 hover:surface-raised cursor-pointer"
                    >
                      <Checkbox
                        id={`state-filter-${state}`}
                        checked={selectedStates.includes(state)}
                        onChange={() => handleStateToggle(state)}
                        label={GAME_STATE_LABELS[state]}
                      />
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Has open spots toggle */}
          <Button
            variant={hasOpenSpots ? "primary" : "outline"}
            onClick={() => onHasOpenSpotsChange(hasOpenSpots ? undefined : true)}
            size="sm"
          >
            {hasOpenSpots && (
              <svg
                className="inline h-4 w-4 mr-1"
                fill="currentColor"
                viewBox="0 0 20 20"
              >
                <path
                  fillRule="evenodd"
                  d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                  clipRule="evenodd"
                />
              </svg>
            )}
            Has Open Spots
          </Button>

          {/* Community filter. Absent when the caller did not supply a list. */}
          {communities && communities.length > 0 && onCommunityChange && (
            <Select
              value={communityId ?? ''}
              onChange={(e) =>
                onCommunityChange(e.target.value === '' ? undefined : Number(e.target.value))
              }
              data-testid="community-filter"
              aria-label="Filter by community"
            >
              <option value="">All communities</option>
              {communities.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </Select>
          )}

          {/* Sort dropdown */}
          <Select
            value={sortBy}
            onChange={(e) => onSortByChange(e.target.value as SortBy)}
          >
            {(Object.keys(SORT_BY_LABELS) as SortBy[]).map((key) => (
              <option key={key} value={key}>
                Sort: {SORT_BY_LABELS[key]}
              </option>
            ))}
          </Select>

          {/* Clear filters button */}
          {hasActiveFilters && (
            <Button
              variant="ghost"
              onClick={onClearFilters}
              size="sm"
            >
              Clear Filters
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
