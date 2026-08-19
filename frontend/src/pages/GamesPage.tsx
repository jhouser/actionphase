import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { GamesList } from '../components/GamesList';
import { CreateGameForm } from '../components/CreateGameForm';
import { ApplyToGameModal } from '../components/ApplyToGameModal';
import { Modal } from '../components/Modal';
import { FilterBar } from '../components/FilterBar';
import { Pagination } from '../components/Pagination';
import { Input } from '../components/ui';
import { useGameListing } from '../hooks/useGameListing';
import { useToast } from '../contexts/ToastContext';
import type { EnrichedGameListItem } from '../types/games';

export const GamesPage = () => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { showSuccess } = useToast();
  const [showCreateModal, setShowCreateModal] = useState(false);
  // Mirrored up from CreateGameForm so the modal can withdraw backdrop dismissal
  // while the form holds unsaved edits.
  const [createFormDirty, setCreateFormDirty] = useState(false);
  const [createCloseRequested, setCreateCloseRequested] = useState(false);
  const [showApplyModal, setShowApplyModal] = useState(false);
  const [selectedGame, setSelectedGame] = useState<EnrichedGameListItem | null>(null);
  const [searchInput, setSearchInput] = useState('');

  // Use the new game listing hook with URL-synced filters
  const {
    games,
    metadata,
    filters,
    setSearch,
    setStates,
    setParticipation,
    setHasOpenSpots,
    setSortBy,
    setPage,
    setPageSize,
    clearFilters,
    isLoading,
    isError,
    error,
  } = useGameListing();

  // Debounced search effect
  useEffect(() => {
    const timeoutId = setTimeout(() => {
      setSearch(searchInput);
    }, 300);

    return () => clearTimeout(timeoutId);
  }, [searchInput, setSearch]);

  const handleCreateGame = () => {
    setShowCreateModal(true);
  };

  // The X asks the form to close rather than closing outright: the form owns the
  // confirmation, since it is the thing holding the unsaved edits.
  const handleCreateModalClose = () => {
    if (createFormDirty) {
      setCreateCloseRequested(true);
      return;
    }
    setShowCreateModal(false);
  };

  const closeCreateModal = () => {
    setCreateCloseRequested(false);
    setCreateFormDirty(false);
    setShowCreateModal(false);
  };

  const handleCreateSuccess = (gameId: number) => {
    setShowCreateModal(false);
    navigate(`/games/${gameId}`);
  };

  const handleOpenApplyModal = (game: EnrichedGameListItem) => {
    setSelectedGame(game);
    setShowApplyModal(true);
  };

  const handleApplicationSubmitted = () => {
    setShowApplyModal(false);
    showSuccess('Successfully applied to game!');
    queryClient.invalidateQueries({ queryKey: ['games', 'filtered'] });
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
      {/* Page Header */}
      <div className="mb-6">
        <div className="flex justify-between items-center">
          <div>
            <h1 className="text-3xl font-bold text-content-primary">Browse Games</h1>
            <p className="text-content-secondary mt-2 text-sm">
              Discover and join role-playing games in the ActionPhase community
            </p>
          </div>
          <button
            onClick={handleCreateGame}
            className="bg-interactive-primary hover:bg-interactive-primary text-white px-4 py-2 rounded-lg transition-colors font-medium"
          >
            Create Game
          </button>
        </div>
      </div>

      {/* Search Bar */}
      <div className="mb-4">
        <Input
          id="game-search"
          type="text"
          placeholder="Search games by title or description..."
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          className="max-w-2xl"
        />
      </div>

      {/* Filter Bar */}
      <FilterBar
        selectedStates={filters.states || []}
        participation={filters.participation}
        hasOpenSpots={filters.has_open_spots}
        sortBy={filters.sort_by || 'recent_activity'}
        availableStates={metadata.available_states}
        onStatesChange={setStates}
        onParticipationChange={setParticipation}
        onHasOpenSpotsChange={setHasOpenSpots}
        onSortByChange={setSortBy}
        onClearFilters={clearFilters}
        filteredCount={metadata.filtered_count}
        totalCount={metadata.total_count}
      />

      {/* Games List */}
      <div className="mt-6">
        <GamesList
          games={games}
          loading={isLoading}
          error={isError ? (error?.message ?? null) : null}
          onApplyToGame={handleOpenApplyModal}
        />
      </div>

      {/* Pagination - always in layout flow to prevent CLS; invisible when not applicable */}
      <div className={`mt-6 ${isLoading || isError || games.length === 0 || metadata.total_pages <= 1 ? 'invisible' : ''}`}>
        <Pagination
          currentPage={metadata.page || 1}
          totalPages={metadata.total_pages || 1}
          pageSize={metadata.page_size || 10}
          hasNextPage={metadata.has_next_page || false}
          hasPreviousPage={metadata.has_previous_page || false}
          onPageChange={setPage}
          onPageSizeChange={setPageSize}
          isLoading={isLoading}
        />
      </div>

      {/* Create Game Modal */}
      {/* The backdrop is withdrawn while the form holds unsaved edits: a stray
          click out is not a decision worth discarding a half-filled form over.
          Cancel and the header X still close, and both confirm first. */}
      <Modal
        isOpen={showCreateModal}
        onClose={handleCreateModalClose}
        title="Create New Game"
        size="5xl"
        dismissOnBackdrop={!createFormDirty}
      >
        <CreateGameForm
          onSuccess={handleCreateSuccess}
          onCancel={closeCreateModal}
          onDirtyChange={setCreateFormDirty}
          closeRequested={createCloseRequested}
          onCloseRequestHandled={() => setCreateCloseRequested(false)}
        />
      </Modal>

      {/* Apply to Game Modal */}
      {selectedGame && (
        <ApplyToGameModal
          gameId={selectedGame.id}
          gameTitle={selectedGame.title}
          autoAcceptAudience={selectedGame.auto_accept_audience}
          audienceOnly={selectedGame.state !== 'recruitment'} // Only audience can join after recruitment
          isOpen={showApplyModal}
          onClose={() => setShowApplyModal(false)}
          onApplicationSubmitted={handleApplicationSubmitted}
        />
      )}
    </div>
  );
};
