import { useState, useMemo } from 'react';
import { useToast } from '../../contexts/ToastContext';
import { useCommunities } from '../../hooks/useCommunities';
import { Button, Input, Badge } from '../../components/ui';
import { UserSearchSelect, type SelectedUser } from '../../components/UserSearchSelect';
import type { Community } from '../../types/communities';
import { extractApiErrorMessage } from '@/lib/errors';

/**
 * Derive a slug candidate from a community name.
 *
 * This only PRE-FILLS the field. The server re-validates on submit and is the
 * authority, so a slug this produces that the server dislikes costs a rejected
 * submit, not a bad slug -- and the admin can always type their own.
 *
 * There is deliberately no backend counterpart. One existed and went unused
 * once the form derived its own suggestion; keeping a second copy in Go meant
 * two implementations that could drift with nothing calling one of them.
 * core.ValidateCommunitySlug is the shared rule that actually matters.
 */
function suggestSlug(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 100)
    .replace(/-+$/g, '');
}

export function CommunitiesTab() {
  const { showSuccess, showError } = useToast();
  const { communities, isLoading, createCommunity, updateCommunity } = useCommunities();

  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  // Tracks whether the admin has typed their own slug. Until they do, the slug
  // follows the name; once they edit it, we stop overwriting their choice.
  const [slugTouched, setSlugTouched] = useState(false);
  const [description, setDescription] = useState('');
  const [owner, setOwner] = useState<SelectedUser | null>(null);

  const effectiveSlug = useMemo(
    () => (slugTouched ? slug : suggestSlug(name)),
    [slug, slugTouched, name]
  );

  const canSubmit = name.trim().length >= 2 && effectiveSlug.length >= 2 && owner !== null;

  const resetForm = () => {
    setName('');
    setSlug('');
    setSlugTouched(false);
    setDescription('');
    setOwner(null);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit || owner === null) return;

    createCommunity.mutate(
      {
        name: name.trim(),
        slug: effectiveSlug,
        description: description.trim() || undefined,
        owner_user_id: owner.id,
      },
      {
        onSuccess: () => {
          showSuccess('Community created');
          resetForm();
        },
        // The server explains slug collisions and unknown owners precisely;
        // surfacing its message beats a generic failure string.
        onError: (err: unknown) => {
          const detail =
            extractApiErrorMessage(err);
          showError(detail || 'Failed to create community');
        },
      }
    );
  };

  const toggleActive = (community: Community) => {
    updateCommunity.mutate(
      { id: community.id, data: { is_active: !community.is_active } },
      {
        onSuccess: () =>
          showSuccess(community.is_active ? 'Community deactivated' : 'Community reactivated'),
        onError: () => showError('Failed to update community'),
      }
    );
  };

  return (
    <div className="bg-surface-base rounded-lg shadow">
      <div className="px-6 py-4 border-b border-theme-default">
        <h2 className="text-xl font-semibold text-content-primary">Communities</h2>
        <p className="text-sm text-content-tertiary mt-1">
          Create communities and assign their owners. Owners manage their own moderators.
        </p>
      </div>

      <form onSubmit={handleSubmit} className="px-6 py-4 border-b border-theme-default space-y-3">
        <div className="flex gap-3 flex-wrap">
          <div className="flex-1 min-w-48">
            <Input
              label="Name"
              placeholder="Midnight Ravens"
              value={name}
              onChange={(e) => setName(e.target.value)}
              data-testid="community-name-input"
            />
          </div>
          <div className="flex-1 min-w-48">
            <Input
              label="Slug"
              placeholder="midnight-ravens"
              value={effectiveSlug}
              onChange={(e) => {
                setSlugTouched(true);
                setSlug(e.target.value);
              }}
              helperText="Appears in the community URL. Cannot be changed later."
              data-testid="community-slug-input"
            />
          </div>
        </div>

        <Input
          label="Description (optional)"
          placeholder="What this community is about"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />

        <UserSearchSelect
          label="Owner"
          value={owner}
          onChange={setOwner}
          helperText="The user who will own this community and manage its moderators"
          dropdownId="community-owner-dropdown"
          disabled={createCommunity.isPending}
          data-testid="community-owner-search"
        />

        <Button
          variant="primary"
          type="submit"
          disabled={!canSubmit || createCommunity.isPending}
          data-testid="create-community-button"
        >
          Create Community
        </Button>
      </form>

      {isLoading ? (
        <div className="px-6 py-8 text-center text-content-tertiary">Loading...</div>
      ) : !communities.length ? (
        <div className="px-6 py-8 text-center text-content-tertiary">No communities yet</div>
      ) : (
        <div className="divide-y divide-theme-default" data-testid="communities-list">
          {communities.map((community) => (
            <div
              key={community.id}
              className="px-6 py-4 hover:bg-surface-raised flex items-center justify-between gap-4"
            >
              <div className="min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="font-medium text-content-primary">{community.name}</span>
                  <Badge variant={community.is_active ? 'success' : 'neutral'}>
                    {community.is_active ? 'Active' : 'Inactive'}
                  </Badge>
                </div>
                <div className="text-sm text-content-secondary font-mono">/{community.slug}</div>
                <div className="text-xs text-content-tertiary">
                  Owner: {community.owner_username ?? `user #${community.owner_user_id}`}
                  {' · '}
                  Created {new Date(community.created_at).toLocaleDateString()}
                </div>
              </div>

              <Button
                variant="secondary"
                onClick={() => toggleActive(community)}
                disabled={updateCommunity.isPending}
              >
                {community.is_active ? 'Deactivate' : 'Reactivate'}
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
