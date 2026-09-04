import { useState } from 'react';
import {
  Card,
  CardBody,
  CardHeader,
  Button,
  Alert,
  Spinner,
  Badge,
  Input,
  Checkbox,
} from '../../components/ui';
import { Modal } from '../../components/Modal';
import { ConfirmModal } from '../../components/ConfirmModal';
import { useToast } from '../../contexts/ToastContext';
import { useCommunityWebhooks } from '../../hooks/useCommunities';
import type {
  Community,
  CommunityWebhook,
  WebhookEvent,
} from '../../types/communities';
import { WEBHOOK_EVENTS, WEBHOOK_EVENT_LABELS } from '../../types/communities';
import { extractApiErrorMessage } from '@/lib/errors';

interface WebhooksTabProps {
  community: Community;
  /** Whether the viewer may configure webhooks. Moderator tier. */
  canModerate: boolean;
}

/** Form state, shared by the create and edit paths. */
interface WebhookDraft {
  /**
   * The Discord webhook URL.
   *
   * 🔴 On the EDIT path this starts EMPTY, never pre-filled with the row's
   * `url`. That value is a mask (`.../••••ab12`), and submitting it would
   * overwrite the stored credential with bullet characters — silently breaking
   * delivery while the UI reported success. Empty means "keep the current URL";
   * the moderator types one only to rotate it.
   */
  url: string;
  label: string;
  isEnabled: boolean;
  events: WebhookEvent[];
}

const EMPTY_DRAFT: WebhookDraft = {
  url: '',
  label: '',
  isEnabled: true,
  events: [],
};

/** Renders a timestamp as a readable local string. */
function formatWhen(iso?: string): string | null {
  if (!iso) return null;
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? null : d.toLocaleString();
}

/**
 * The delivery-status line for one webhook.
 *
 * Three genuinely different states, and collapsing any two of them would
 * mislead:
 *
 *   - never used   — no attempt yet. NOT a failure, and must not look like one.
 *   - working      — delivered, no outstanding error.
 *   - failing      — an error since the last success. This is the state the
 *                    whole feature's observability exists to surface.
 *
 * A row can have BOTH a last success and a current error; that is the important
 * case ("worked this morning, broken since lunch"), so both are shown.
 */
function WebhookStatus({ webhook }: { webhook: CommunityWebhook }) {
  const succeededAt = formatWhen(webhook.last_success_at);
  const failedAt = formatWhen(webhook.last_error_at);

  if (!succeededAt && !webhook.last_error) {
    return (
      <p className="text-sm text-content-secondary" data-testid="webhook-status-unused">
        Not used yet — no message has been sent through this webhook.
      </p>
    );
  }

  return (
    <div className="space-y-1" data-testid="webhook-status">
      {webhook.last_error ? (
        <Alert variant="danger" title="Delivery failing">
          <p className="break-words" data-testid="webhook-last-error">
            {webhook.last_error}
          </p>
          {failedAt && (
            <p className="mt-1 text-sm">Last failed {failedAt}</p>
          )}
          {succeededAt && (
            <p className="mt-1 text-sm">
              Last delivered successfully {succeededAt}
            </p>
          )}
        </Alert>
      ) : (
        <p className="text-sm text-content-secondary" data-testid="webhook-last-success">
          Last delivered {succeededAt}
        </p>
      )}
    </div>
  );
}

/**
 * A community's Discord webhooks (req 9).
 *
 * 🔴 THE URL IS A CREDENTIAL. The server returns it masked and never in full,
 * so this screen can display it but can never re-send it. Every save omits
 * `url` unless the moderator typed a new one — see WebhookDraft.url.
 *
 * Delivery is best-effort: there is no queue and no redelivery, so the
 * per-row status below is the entire answer to "is my webhook working?"
 */
export function WebhooksTab({ community, canModerate }: WebhooksTabProps) {
  const { showSuccess, showError } = useToast();
  const {
    webhooks,
    isLoading,
    isError,
    createWebhook,
    updateWebhook,
    deleteWebhook,
    testWebhook,
  } = useCommunityWebhooks(community.slug, canModerate);

  // null = not editing; a number is the id being edited; 'new' is the create
  // form. One piece of state so the two forms cannot both be open.
  const [editing, setEditing] = useState<number | 'new' | null>(null);
  const [draft, setDraft] = useState<WebhookDraft>(EMPTY_DRAFT);

  // Holds the whole record rather than an id, so the prompt can name what is
  // about to be disconnected.
  const [pendingDelete, setPendingDelete] = useState<CommunityWebhook | null>(null);

  // Which row's test is in flight, so only that button shows a spinner.
  const [testingId, setTestingId] = useState<number | null>(null);

  const startCreate = () => {
    setDraft(EMPTY_DRAFT);
    setEditing('new');
  };

  const startEdit = (hook: CommunityWebhook) => {
    setDraft({
      // Deliberately blank -- see WebhookDraft.url. Pre-filling with the masked
      // value is the bug this comment exists to prevent.
      url: '',
      label: hook.label ?? '',
      isEnabled: hook.is_enabled,
      events: hook.events,
    });
    setEditing(hook.id);
  };

  const cancel = () => {
    setEditing(null);
    setDraft(EMPTY_DRAFT);
  };

  /** Reads the server's message, which names the actual problem. */
  const errorDetail = (err: unknown, fallback: string) =>
    extractApiErrorMessage(err) ?? fallback;

  const toggleEvent = (event: WebhookEvent) => {
    setDraft((d) => ({
      ...d,
      events: d.events.includes(event)
        ? d.events.filter((e) => e !== event)
        : [...d.events, event],
    }));
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    const label = draft.label.trim();
    const url = draft.url.trim();

    if (editing === 'new') {
      if (!url) return;
      createWebhook.mutate(
        {
          url,
          label: label || undefined,
          is_enabled: draft.isEnabled,
          events: draft.events,
        },
        {
          onSuccess: () => {
            showSuccess('Webhook added');
            cancel();
          },
          onError: (err) => showError(errorDetail(err, 'Could not add that webhook')),
        }
      );
      return;
    }

    if (typeof editing !== 'number') return;

    updateWebhook.mutate(
      {
        id: editing,
        data: {
          // Sent ONLY when the moderator typed a replacement. Omitting it keeps
          // the stored credential; sending the masked value would destroy it.
          ...(url ? { url } : {}),
          label,
          is_enabled: draft.isEnabled,
          events: draft.events,
        },
      },
      {
        onSuccess: () => {
          showSuccess(url ? 'Webhook updated and URL rotated' : 'Webhook updated');
          cancel();
        },
        onError: (err) => showError(errorDetail(err, 'Could not update that webhook')),
      }
    );
  };

  const confirmDelete = () => {
    if (!pendingDelete) return;
    const name = pendingDelete.label || 'this webhook';

    deleteWebhook.mutate(pendingDelete.id, {
      onSuccess: () => {
        showSuccess(`Disconnected ${name}`);
        setPendingDelete(null);
      },
      // The prompt stays open on failure: closing it would suggest the webhook
      // was removed when it is still live.
      onError: (err) => showError(errorDetail(err, 'Could not delete that webhook')),
    });
  };

  const runTest = (hook: CommunityWebhook) => {
    setTestingId(hook.id);
    testWebhook.mutate(hook.id, {
      onSuccess: () => showSuccess('Test message delivered to Discord'),
      // The server's message carries Discord's own reason, which is the point
      // of pressing this button.
      onError: (err) => showError(errorDetail(err, 'Discord rejected the test message')),
      onSettled: () => setTestingId(null),
    });
  };

  const toggleEnabled = (hook: CommunityWebhook) => {
    updateWebhook.mutate(
      // No `url` -- this is a status flip, not a rotation.
      { id: hook.id, data: { is_enabled: !hook.is_enabled } },
      {
        onSuccess: () =>
          showSuccess(hook.is_enabled ? 'Webhook disabled' : 'Webhook enabled'),
        onError: (err) => showError(errorDetail(err, 'Could not change that webhook')),
      }
    );
  };

  if (!canModerate) {
    return (
      <Alert variant="info">
        Only this community's moderators can configure Discord webhooks.
      </Alert>
    );
  }

  if (isLoading) {
    return (
      <div className="flex justify-center py-8">
        <Spinner size="lg" />
      </div>
    );
  }

  if (isError) {
    return <Alert variant="danger">Could not load this community's webhooks.</Alert>;
  }

  const saving = createWebhook.isPending || updateWebhook.isPending;

  return (
    <div className="space-y-4" data-testid="webhooks-tab">
      <div className="flex items-start justify-between gap-4">
        <p className="text-sm text-content-secondary">
          Announce this community's game state changes in a Discord channel.
          Messages are best-effort — if Discord is unavailable the game still
          changes state, and nothing is re-sent later.
        </p>
        <Button variant="primary" onClick={startCreate} data-testid="add-webhook">
          Add webhook
        </Button>
      </div>

      {webhooks.length === 0 ? (
        <Card variant="bordered" padding="md">
          <CardBody>
            <p className="text-content-secondary" data-testid="webhooks-empty">
              No webhooks yet. Add one to post game announcements to Discord.
            </p>
          </CardBody>
        </Card>
      ) : (
        <div className="space-y-3">
          {webhooks.map((hook) => (
            <Card key={hook.id} variant="default" padding="md" data-testid="webhook-row">
              <CardHeader>
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-semibold text-content-primary">
                    {hook.label || 'Discord webhook'}
                  </span>
                  {hook.is_enabled ? (
                    <Badge variant="success">Enabled</Badge>
                  ) : (
                    <Badge variant="neutral">Disabled</Badge>
                  )}
                  {hook.last_error && <Badge variant="danger">Failing</Badge>}
                </div>
              </CardHeader>

              <CardBody>
                <div className="space-y-3">
                  {/*
                    The masked URL. Shown so a moderator can tell two webhooks
                    apart; it is not a working URL and cannot be copied into
                    Discord.
                  */}
                  <p
                    className="break-all font-mono text-sm text-content-secondary"
                    data-testid="webhook-url"
                  >
                    {hook.url}
                  </p>

                  <div className="flex flex-wrap gap-1" data-testid="webhook-events">
                    {hook.events.length === 0 ? (
                      <span className="text-sm text-content-secondary">
                        No events selected — this webhook will never fire.
                      </span>
                    ) : (
                      hook.events.map((event) => (
                        <Badge key={event} variant="secondary">
                          {WEBHOOK_EVENT_LABELS[event] ?? event}
                        </Badge>
                      ))
                    )}
                  </div>

                  <WebhookStatus webhook={hook} />

                  <div className="flex flex-wrap gap-2">
                    <Button
                      variant="secondary"
                      onClick={() => runTest(hook)}
                      loading={testingId === hook.id}
                      disabled={testingId !== null}
                      data-testid="webhook-test"
                    >
                      Send test
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => startEdit(hook)}
                      data-testid="webhook-edit"
                    >
                      Edit
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => toggleEnabled(hook)}
                      data-testid="webhook-toggle"
                    >
                      {hook.is_enabled ? 'Disable' : 'Enable'}
                    </Button>
                    <Button
                      variant="danger"
                      onClick={() => setPendingDelete(hook)}
                      data-testid="webhook-delete"
                    >
                      Delete
                    </Button>
                  </div>
                </div>
              </CardBody>
            </Card>
          ))}
        </div>
      )}

      <Modal
        isOpen={editing !== null}
        onClose={cancel}
        title={editing === 'new' ? 'Add a Discord webhook' : 'Edit webhook'}
        dismissOnBackdrop={false}
        size="2xl"
        testId="webhook-editor"
      >
        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            label="Discord webhook URL"
            type="url"
            value={draft.url}
            onChange={(e) => setDraft((d) => ({ ...d, url: e.target.value }))}
            placeholder="https://discord.com/api/webhooks/..."
            required={editing === 'new'}
            data-testid="webhook-url-input"
            /*
              On edit this is blank and optional: the client never holds the real
              URL, so "unchanged" has to mean "send nothing" rather than
              "re-send what is displayed".
            */
            helperText={
              editing === 'new'
                ? 'In Discord: Channel Settings → Integrations → Webhooks → Copy URL.'
                : 'Leave blank to keep the current URL. Enter a new one only to replace it.'
            }
          />

          <Input
            label="Label"
            value={draft.label}
            onChange={(e) => setDraft((d) => ({ ...d, label: e.target.value }))}
            placeholder="#game-announcements"
            maxLength={100}
            data-testid="webhook-label-input"
            helperText="Optional. Helps tell several webhooks apart."
          />

          <fieldset className="space-y-2">
            <legend className="text-sm font-medium text-content-primary">
              Announce these changes
            </legend>
            <div className="grid gap-2 sm:grid-cols-2">
              {WEBHOOK_EVENTS.map((event) => (
                <Checkbox
                  key={event}
                  label={WEBHOOK_EVENT_LABELS[event]}
                  checked={draft.events.includes(event)}
                  onChange={() => toggleEvent(event)}
                  data-testid={`webhook-event-${event}`}
                />
              ))}
            </div>
          </fieldset>

          <Checkbox
            label="Enabled"
            checked={draft.isEnabled}
            onChange={(e) => setDraft((d) => ({ ...d, isEnabled: e.target.checked }))}
            data-testid="webhook-enabled-input"
          />

          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={cancel} type="button">
              Cancel
            </Button>
            <Button
              variant="primary"
              type="submit"
              loading={saving}
              data-testid="webhook-save"
            >
              {editing === 'new' ? 'Add webhook' : 'Save changes'}
            </Button>
          </div>
        </form>
      </Modal>

      <ConfirmModal
        isOpen={pendingDelete !== null}
        onClose={() => setPendingDelete(null)}
        onConfirm={confirmDelete}
        title="Delete webhook"
        message={
          pendingDelete
            ? `Delete ${pendingDelete.label || 'this webhook'}? Announcements will stop being posted to that channel. This cannot be undone.`
            : ''
        }
        confirmText="Delete"
        variant="danger"
        isLoading={deleteWebhook.isPending}
      />
    </div>
  );
}
