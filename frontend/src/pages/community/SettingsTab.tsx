import { useEffect, useState } from 'react';
import { Card, CardBody, CardHeader, Button, Input, Alert } from '../../components/ui';
import { CommentEditor } from '../../components/CommentEditor';
import { useToast } from '../../contexts/ToastContext';
import { useUpdateCommunityProfile } from '../../hooks/useCommunities';
import type { Community } from '../../types/communities';

interface SettingsTabProps {
  community: Community;
  /**
   * Whether the viewer may edit the profile. Moderator-level, unlike the
   * roster: keeping the name and blurb current is ordinary upkeep. Read-only
   * viewers still see the current values so the tab is not a dead end.
   */
  canEdit: boolean;
}

/**
 * A community's name and description.
 *
 * The slug is deliberately absent: it is immutable after creation because it
 * appears in URLs the community has shared externally. Showing it as a
 * disabled field would only invite the question; the tab explains it instead.
 *
 * Ownership and active status are not here either -- those are site-admin acts
 * on the admin surface, and the moderator endpoint rejects them outright.
 */
export function SettingsTab({ community, canEdit }: SettingsTabProps) {
  const { showSuccess, showError } = useToast();
  const updateProfile = useUpdateCommunityProfile(community.slug);

  const [name, setName] = useState(community.name);
  const [description, setDescription] = useState(community.description ?? '');

  // Re-sync when the community changes underneath the form -- after a save the
  // query refetches, and a moderator switching communities reuses this
  // component. Without this the fields would keep the previous community's text.
  useEffect(() => {
    setName(community.name);
    setDescription(community.description ?? '');
  }, [community.name, community.description]);

  const trimmedName = name.trim();
  const isDirty =
    trimmedName !== community.name || description !== (community.description ?? '');
  // The server rejects a blank name; matching that here keeps the button honest
  // rather than letting the GM discover it through a 400.
  const canSubmit = canEdit && isDirty && trimmedName.length >= 2 && !updateProfile.isPending;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;

    // Send only what changed. The endpoint is preserve-on-absent, so omitting
    // an untouched field is both smaller and safer than resending it.
    //
    // An empty description is sent as '' rather than omitted: that is how the
    // blurb gets CLEARED. Omission would silently mean "leave it alone" and
    // make a description impossible to remove once written.
    const payload: { name?: string; description?: string } = {};
    if (trimmedName !== community.name) payload.name = trimmedName;
    if (description !== (community.description ?? '')) payload.description = description;

    updateProfile.mutate(payload, {
      onSuccess: () => showSuccess('Community profile updated'),
      onError: (err: unknown) => {
        const detail =
          (err as { response?: { data?: { detail?: string } } })?.response?.data?.detail;
        showError(detail ?? 'Could not save those changes');
      },
    });
  };

  if (!canEdit) {
    return (
      <Alert variant="info" title="Read-only">
        Only this community&apos;s moderators and owner can edit its profile.
      </Alert>
    );
  }

  return (
    <Card variant="default" padding="md">
      <CardHeader>
        <h2 className="text-lg font-semibold text-content-primary">Profile</h2>
        <p className="text-sm text-content-tertiary mt-1">
          The name and description shown on this community&apos;s page. The address (
          <code className="text-content-secondary">/communities/{community.slug}</code>) cannot
          be changed, so existing links keep working.
        </p>
      </CardHeader>
      <CardBody>
        <form onSubmit={handleSubmit} data-testid="community-settings-form">
          <Input
            label="Name"
            id="community-settings-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            maxLength={255}
            required
            data-testid="community-settings-name"
          />

          <div className="mt-4">
            <label
              htmlFor="community-settings-description"
              className="block text-sm font-medium text-content-primary mb-1"
            >
              Description
            </label>
            <CommentEditor
              id="community-settings-description"
              value={description}
              onChange={setDescription}
              placeholder="Describe this community... (Markdown supported)"
              rows={10}
              textareaTestId="community-settings-description"
            />
          </div>

          <div className="flex items-center gap-3 mt-4">
            <Button
              type="submit"
              variant="primary"
              disabled={!canSubmit}
              loading={updateProfile.isPending}
              data-testid="community-settings-save"
            >
              Save changes
            </Button>
            {isDirty && !updateProfile.isPending && (
              <span className="text-sm text-content-tertiary">Unsaved changes</span>
            )}
          </div>
        </form>
      </CardBody>
    </Card>
  );
}
