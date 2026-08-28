import { useState, useEffect, useRef } from 'react';
import { createPortal } from 'react-dom';
import { Button, Input } from './ui';
import { Modal } from './Modal';
import { useAddParticipant } from '../hooks/usePlayerManagement';
import { apiClient } from '../lib/api';
import { logger } from '@/services/LoggingService';

interface AddParticipantModalProps {
  gameId: number;
  role: 'player' | 'audience';
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
  excludeUserIds?: number[];
}

interface SearchResult {
  id: number;
  username: string;
  created_at: string;
}

const CONFIG = {
  player: {
    title: 'Add Player Directly',
    description: 'Adding a player directly bypasses the application process and grants them immediate access to the game.',
    buttonLabel: 'Add Player',
    errorMessage: 'Failed to add player. They may already be in the game, or the user may be invalid.',
  },
  audience: {
    title: 'Add Audience Member Directly',
    description: 'Adding an audience member directly bypasses the application process and grants them immediate audience access.',
    buttonLabel: 'Add Audience Member',
    errorMessage: 'Failed to add audience member. They may already be in the game, or the user may be invalid.',
  },
} as const;

interface DropdownPosition {
  top: number;
  left: number;
  width: number;
}

const EMPTY_ARRAY: number[] = [];

export function AddParticipantModal({ gameId, role, isOpen, onClose, onSuccess, excludeUserIds = EMPTY_ARRAY }: AddParticipantModalProps) {
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [selectedUser, setSelectedUser] = useState<SearchResult | null>(null);
  const [isSearching, setIsSearching] = useState(false);
  const [showDropdown, setShowDropdown] = useState(false);
  const [dropdownPos, setDropdownPos] = useState<DropdownPosition | null>(null);
  const inputWrapperRef = useRef<HTMLDivElement>(null);
  const addParticipant = useAddParticipant(gameId, role);
  const config = CONFIG[role];

  const updateDropdownPos = () => {
    if (!inputWrapperRef.current) return;
    const rect = inputWrapperRef.current.getBoundingClientRect();
    setDropdownPos({ top: rect.bottom + 4, left: rect.left, width: rect.width });
  };

  useEffect(() => {
    if (!searchQuery.trim() || selectedUser) {
      setSearchResults([]);
      setShowDropdown(false);
      return;
    }

    const timeoutId = setTimeout(async () => {
      setIsSearching(true);
      updateDropdownPos();
      try {
        const response = await apiClient.auth.searchUsers(searchQuery);
        const filtered = excludeUserIds.length > 0
          ? response.data.users.filter((u: SearchResult) => !excludeUserIds.includes(u.id))
          : response.data.users;
        setSearchResults(filtered);
        setShowDropdown(true);
      } catch (error) {
        logger.error('Failed to search users', { error, searchQuery });
        setSearchResults([]);
      } finally {
        setIsSearching(false);
      }
    }, 300);

    return () => clearTimeout(timeoutId);
  }, [searchQuery, selectedUser, excludeUserIds]);

  // Keep dropdown position in sync if the window is resized while open
  useEffect(() => {
    if (!showDropdown) return;
    const handler = () => updateDropdownPos();
    window.addEventListener('resize', handler);
    window.addEventListener('scroll', handler, true);
    return () => {
      window.removeEventListener('resize', handler);
      window.removeEventListener('scroll', handler, true);
    };
  }, [showDropdown]);

  useEffect(() => {
    if (!showDropdown) return;
    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target as Node;
      const insideInput = inputWrapperRef.current?.contains(target);
      const insideDropdown = document.getElementById('participant-search-dropdown')?.contains(target);
      if (!insideInput && !insideDropdown) {
        setShowDropdown(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [showDropdown]);

  const handleSelectUser = (user: SearchResult) => {
    setSelectedUser(user);
    setSearchQuery(user.username);
    setShowDropdown(false);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedUser) return;

    try {
      await addParticipant.mutateAsync(selectedUser.id);
      setSearchQuery('');
      setSelectedUser(null);
      onClose();
      onSuccess?.();
    } catch (error) {
      logger.error(`Failed to add ${role}`, { error, gameId, userId: selectedUser.id, username: selectedUser.username });
    }
  };

  const handleClose = () => {
    if (!addParticipant.isPending) {
      setSearchQuery('');
      setSelectedUser(null);
      setSearchResults([]);
      setShowDropdown(false);
      addParticipant.reset();
      onClose();
    }
  };

  const dropdown = showDropdown && dropdownPos && (
    <div
      id="participant-search-dropdown"
      style={{
        position: 'fixed',
        top: dropdownPos.top,
        left: dropdownPos.left,
        width: dropdownPos.width,
        zIndex: 9999,
      }}
      className="bg-surface-overlay border border-theme-default rounded-lg shadow-xl max-h-56 overflow-y-auto"
    >
      {isSearching && (
        <p className="px-4 py-3 text-sm text-content-secondary">Searching...</p>
      )}
      {!isSearching && searchResults.length === 0 && searchQuery.trim() && (
        <p className="px-4 py-3 text-sm text-content-secondary">No users found matching "{searchQuery}"</p>
      )}
      {!isSearching && searchResults.map((user) => (
        <button
          key={user.id}
          type="button"
          onMouseDown={(e) => {
            // Use onMouseDown + preventDefault so the input doesn't lose focus before we register the click
            e.preventDefault();
            handleSelectUser(user);
          }}
          className="w-full px-4 py-3 text-left bg-surface-overlay hover:surface-raised transition-colors border-b border-theme-default last:border-b-0"
        >
          <div className="font-medium text-content-primary">{user.username}</div>
          <div className="text-sm text-content-secondary">
            Joined {new Date(user.created_at).toLocaleDateString()}
          </div>
        </button>
      ))}
    </div>
  );

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title={config.title}>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="p-4 rounded-lg bg-semantic-info-subtle border border-theme-default">
          <p className="text-sm text-content-primary">{config.description}</p>
        </div>

        <div ref={inputWrapperRef}>
          <Input
            label="Search Users"
            type="text"
            placeholder="Type username to search..."
            value={searchQuery}
            onChange={(e) => {
              setSearchQuery(e.target.value);
              setSelectedUser(null);
            }}
            onFocus={() => {
              if (searchResults.length > 0) {
                updateDropdownPos();
                setShowDropdown(true);
              }
            }}
            helperText={selectedUser ? `Selected: ${selectedUser.username}` : 'Start typing to search for users'}
            required
            disabled={addParticipant.isPending}
          />
        </div>

        {createPortal(dropdown, document.body)}

        {addParticipant.isError && (
          <div className="p-3 rounded-lg bg-semantic-danger-subtle border border-semantic-danger">
            <p className="text-sm text-semantic-danger">{config.errorMessage}</p>
          </div>
        )}

        <div className="flex justify-end gap-3">
          <Button type="button" variant="secondary" onClick={handleClose} disabled={addParticipant.isPending}>
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            loading={addParticipant.isPending}
            disabled={!selectedUser || addParticipant.isPending}
          >
            {config.buttonLabel}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
