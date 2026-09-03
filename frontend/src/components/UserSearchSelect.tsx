import { useState, useEffect, useRef } from 'react';
import { createPortal } from 'react-dom';
import { Input } from './ui';
import { apiClient } from '../lib/api';
import { logger } from '@/services/LoggingService';

/**
 * A user picked from the search results.
 *
 * Email is deliberately absent: the search endpoint returns it, but no picker
 * surface displays another user's address. Username plus join date is enough to
 * disambiguate two similar names.
 */
export interface SelectedUser {
  id: number;
  username: string;
  created_at: string;
}

interface UserSearchSelectProps {
  label?: string;
  placeholder?: string;
  /** The currently selected user, or null. Controlled by the parent. */
  value: SelectedUser | null;
  onChange: (user: SelectedUser | null) => void;
  /** User IDs to hide from results (already added, already an owner, ...). */
  excludeUserIds?: number[];
  disabled?: boolean;
  required?: boolean;
  helperText?: string;
  /** Distinguishes this instance's portal when several are on one page. */
  dropdownId?: string;
  'data-testid'?: string;
}

const EMPTY_ARRAY: number[] = [];

interface DropdownPosition {
  top: number;
  left: number;
  width: number;
}

/**
 * Typeahead for choosing a user by username.
 *
 * Extracted from AddParticipantModal so every "pick a user" surface behaves the
 * same way: a 300ms debounce, a portalled dropdown that escapes overflow
 * clipping, dismissal on outside click, and no email addresses on screen.
 */
export function UserSearchSelect({
  label = 'Search Users',
  placeholder = 'Type username to search...',
  value,
  onChange,
  excludeUserIds = EMPTY_ARRAY,
  disabled = false,
  required = false,
  helperText,
  dropdownId = 'user-search-dropdown',
  'data-testid': testId,
}: UserSearchSelectProps) {
  const [searchQuery, setSearchQuery] = useState(value?.username ?? '');
  const [searchResults, setSearchResults] = useState<SelectedUser[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [showDropdown, setShowDropdown] = useState(false);
  const [dropdownPos, setDropdownPos] = useState<DropdownPosition | null>(null);
  const inputWrapperRef = useRef<HTMLDivElement>(null);
  const prevValueRef = useRef<SelectedUser | null>(value);

  // The parent owns the selection but not the text in the box, so a parent-side
  // reset (clearing the form after a successful submit) would otherwise leave a
  // stale username on screen for a user who is no longer selected.
  //
  // Only reacts to the parent CHANGING the value out from under us -- comparing
  // against the previous value keeps this from fighting the user's own typing,
  // which sets value to null on every keystroke after a selection.
  useEffect(() => {
    const prev = prevValueRef.current;
    prevValueRef.current = value;

    if (value === null && prev !== null && searchQuery === prev.username) {
      setSearchQuery('');
    } else if (value !== null && value.id !== prev?.id) {
      setSearchQuery(value.username);
    }
  }, [value, searchQuery]);

  const updateDropdownPos = () => {
    if (!inputWrapperRef.current) return;
    const rect = inputWrapperRef.current.getBoundingClientRect();
    setDropdownPos({ top: rect.bottom + 4, left: rect.left, width: rect.width });
  };

  useEffect(() => {
    // A chosen user means the query is just their name echoed back, not a
    // search term -- re-querying would reopen the dropdown over their choice.
    if (!searchQuery.trim() || value) {
      setSearchResults([]);
      setShowDropdown(false);
      return;
    }

    const timeoutId = setTimeout(async () => {
      setIsSearching(true);
      updateDropdownPos();
      try {
        const response = await apiClient.auth.searchUsers(searchQuery);
        const filtered =
          excludeUserIds.length > 0
            ? response.data.users.filter((u) => !excludeUserIds.includes(u.id))
            : response.data.users;
        setSearchResults(
          filtered.map((u) => ({ id: u.id, username: u.username, created_at: u.created_at }))
        );
        setShowDropdown(true);
      } catch (error) {
        logger.error('Failed to search users', { error, searchQuery });
        setSearchResults([]);
      } finally {
        setIsSearching(false);
      }
    }, 300);

    return () => clearTimeout(timeoutId);
  }, [searchQuery, value, excludeUserIds]);

  // Keep the portalled dropdown glued to the input when the page moves.
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
      const insideDropdown = document.getElementById(dropdownId)?.contains(target);
      if (!insideInput && !insideDropdown) {
        setShowDropdown(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [showDropdown, dropdownId]);

  const handleSelect = (user: SelectedUser) => {
    onChange(user);
    setSearchQuery(user.username);
    setShowDropdown(false);
  };

  const dropdown = showDropdown && dropdownPos && (
    <div
      id={dropdownId}
      style={{
        position: 'fixed',
        top: dropdownPos.top,
        left: dropdownPos.left,
        width: dropdownPos.width,
        zIndex: 9999,
      }}
      className="bg-surface-overlay border border-theme-default rounded-lg shadow-xl max-h-56 overflow-y-auto"
    >
      {isSearching && <p className="px-4 py-3 text-sm text-content-secondary">Searching...</p>}
      {!isSearching && searchResults.length === 0 && searchQuery.trim() && (
        <p className="px-4 py-3 text-sm text-content-secondary">
          No users found matching "{searchQuery}"
        </p>
      )}
      {!isSearching &&
        searchResults.map((user) => (
          <button
            key={user.id}
            type="button"
            onMouseDown={(e) => {
              // onMouseDown + preventDefault so the input keeps focus long
              // enough for the click to register.
              e.preventDefault();
              handleSelect(user);
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
    <>
      <div ref={inputWrapperRef}>
        <Input
          label={label}
          type="text"
          placeholder={placeholder}
          value={searchQuery}
          onChange={(e) => {
            setSearchQuery(e.target.value);
            // Typing after a selection clears it, so the parent can never
            // submit a stale user that no longer matches what is on screen.
            if (value) onChange(null);
          }}
          onFocus={() => {
            if (searchResults.length > 0) {
              updateDropdownPos();
              setShowDropdown(true);
            }
          }}
          helperText={
            value ? `Selected: ${value.username}` : helperText ?? 'Start typing to search for users'
          }
          required={required}
          disabled={disabled}
          data-testid={testId}
        />
      </div>
      {createPortal(dropdown, document.body)}
    </>
  );
}
