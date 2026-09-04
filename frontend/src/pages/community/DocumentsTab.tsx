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
import { CommentEditor } from '../../components/CommentEditor';
import { Modal } from '../../components/Modal';
import { ConfirmModal } from '../../components/ConfirmModal';
import { useToast } from '../../contexts/ToastContext';
import { useManageCommunityDocuments } from '../../hooks/useCommunities';
import type { Community, CommunityDocument } from '../../types/communities';

interface DocumentsTabProps {
  community: Community;
  /**
   * Whether the viewer may edit documents. Writing a community's rules is the
   * moderator tier, not the owner tier -- it is ordinary upkeep.
   */
  canModerate: boolean;
}

/** Form state for the editor, shared by the create and edit paths. */
interface DocumentDraft {
  title: string;
  content: string;
  sortOrder: string;
  /**
   * Create path only: whether to publish immediately.
   *
   * The API has always accepted `status` on create; the form used to omit it,
   * so every document was born a draft and needed a second Publish click even
   * when the moderator had just written a finished page. Defaults to false --
   * unpublished remains the safe default, this only makes the other option
   * reachable.
   *
   * The EDIT path ignores this: status there is owned by the row's
   * Publish/Unpublish toggle, and a second control for one piece of state is
   * how the two disagree.
   */
  publishNow: boolean;
}

const EMPTY_DRAFT: DocumentDraft = {
  title: '',
  content: '',
  sortOrder: '0',
  publishNow: false,
};

/**
 * A community's documents (req 7).
 *
 * The list here is the PRIVILEGED one: it includes drafts, which the public
 * community page never shows. That is why it uses `useManageCommunityDocuments`
 * rather than the public hook -- the two are separate cache keys precisely so a
 * draft cannot leak into a public view.
 *
 * Publishing is an edit, not a separate action: status sits on the same form as
 * the body, so a moderator who fixes a typo and publishes does it in one
 * request. The status toggle below is a convenience over that same PATCH.
 */
export function DocumentsTab({ community, canModerate }: DocumentsTabProps) {
  const { showSuccess, showError } = useToast();
  const {
    documents,
    isLoading,
    isError,
    createDocument,
    updateDocument,
    deleteDocument,
  } = useManageCommunityDocuments(community.slug, canModerate);

  // null = not editing. A number is the id being edited; 'new' is the create
  // form. One piece of state rather than two booleans, so the two forms cannot
  // both be open at once.
  const [editing, setEditing] = useState<number | 'new' | null>(null);
  const [draft, setDraft] = useState<DocumentDraft>(EMPTY_DRAFT);

  // The document awaiting delete confirmation, or null. Holds the whole record
  // rather than an id so the prompt can name what is about to be destroyed --
  // "Delete House Rules?" is answerable, "Delete this document?" is not.
  const [pendingDelete, setPendingDelete] = useState<CommunityDocument | null>(null);

  const startCreate = () => {
    setDraft(EMPTY_DRAFT);
    setEditing('new');
  };

  const startEdit = (doc: CommunityDocument) => {
    setDraft({
      title: doc.title,
      content: doc.content,
      sortOrder: String(doc.sort_order),
      // Not surfaced on the edit form -- the row's toggle owns status there.
      publishNow: doc.status === 'published',
    });
    setEditing(doc.id);
  };

  const cancel = () => {
    setEditing(null);
    setDraft(EMPTY_DRAFT);
  };

  /** Reads the server's message, which names the actual problem. */
  const errorDetail = (err: unknown, fallback: string) => {
    const detail = (err as { response?: { data?: { error?: string } } })?.response?.data
      ?.error;
    return detail ?? fallback;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!draft.title.trim()) return;

    // An unparseable sort order falls back to 0 rather than sending NaN, which
    // would serialise to null and read as "leave unchanged".
    const sortOrder = Number.parseInt(draft.sortOrder, 10);
    const payload = {
      title: draft.title.trim(),
      content: draft.content,
      sort_order: Number.isNaN(sortOrder) ? 0 : sortOrder,
    };

    if (editing === 'new') {
      // status is sent ONLY here. On the edit path it is omitted, so the PATCH
      // leaves the column alone (COALESCE) and the row's toggle stays the one
      // owner of published state.
      const status = draft.publishNow ? 'published' : 'draft';
      createDocument.mutate(
        { ...payload, status },
        {
          onSuccess: () => {
            showSuccess(
              draft.publishNow
                ? `"${payload.title}" published`
                : `"${payload.title}" created as a draft`
            );
            cancel();
          },
          onError: (err) => showError(errorDetail(err, 'Could not create that document')),
        }
      );
      return;
    }

    if (typeof editing === 'number') {
      updateDocument.mutate(
        { id: editing, data: payload },
        {
          onSuccess: () => {
            showSuccess(`"${payload.title}" saved`);
            cancel();
          },
          onError: (err) => showError(errorDetail(err, 'Could not save that document')),
        }
      );
    }
  };

  const togglePublished = (doc: CommunityDocument) => {
    const next = doc.status === 'published' ? 'draft' : 'published';
    updateDocument.mutate(
      { id: doc.id, data: { status: next } },
      {
        onSuccess: () =>
          showSuccess(
            next === 'published'
              ? `"${doc.title}" is now visible to everyone`
              : `"${doc.title}" is back to a draft`
          ),
        onError: (err) => showError(errorDetail(err, 'Could not change that document')),
      }
    );
  };

  /**
   * Runs after the moderator confirms. Deleting a document is irreversible and
   * destroys writing someone may have spent real effort on, so it goes behind
   * ConfirmModal like every other destructive action on the site -- the service
   * deliberately refuses a missing document rather than reporting a silent
   * success for the same reason, and an unguarded button here undercut that.
   */
  const confirmDelete = () => {
    const doc = pendingDelete;
    if (!doc) return;

    deleteDocument.mutate(doc.id, {
      onSuccess: () => {
        showSuccess(`"${doc.title}" deleted`);
        setPendingDelete(null);
      },
      onError: (err) => {
        showError(errorDetail(err, 'Could not delete that document'));
        // Left open: the prompt still names the document, so retrying does not
        // mean finding it in the list again.
      },
    });
  };

  if (!canModerate) {
    return (
      <Alert variant="info" title="Moderators only">
        Only this community's moderators can manage its documents.
      </Alert>
    );
  }

  const isSaving = createDocument.isPending || updateDocument.isPending;

  return (
    <div className="space-y-6">
      {/* The LIST is the page, with the editor in a modal -- the same shape as
          handouts, which these documents otherwise mirror.

          The first version put an always-open create form above the list,
          copied from BansTab. That works there because banning is a one-line
          form: pick a user, submit. A twelve-row markdown editor is not that,
          and it pushed the list of documents -- the thing a moderator came to
          this tab to manage -- below the fold. */}
      <Card variant="default" padding="md">
        <CardHeader>
          <div className="flex items-start justify-between gap-3">
            <div>
              <h2 className="text-lg font-semibold text-content-primary">Documents</h2>
              <p className="text-sm text-content-tertiary mt-1">
                Rules, etiquette, and reference pages for {community.name}. Published
                documents appear on the community page and on the Info tab of its games.
              </p>
            </div>
            <Button variant="primary" onClick={startCreate} data-testid="new-document">
              New document
            </Button>
          </div>
        </CardHeader>

        <CardBody>
          {isLoading && (
            <div className="py-6 flex justify-center" data-testid="documents-loading">
              <Spinner size="md" />
            </div>
          )}

          {isError && (
            <Alert variant="danger" title="Could not load the documents">
              Try reloading the page.
            </Alert>
          )}

          {!isLoading && !isError && documents.length === 0 && (
            <p className="text-sm text-content-tertiary" data-testid="documents-empty">
              This community has no documents yet.
            </p>
          )}

          {!isLoading && !isError && documents.length > 0 && (
            <ul className="divide-y divide-theme-default" data-testid="document-list">
              {documents.map((doc) => (
                <li
                  key={doc.id}
                  className="flex items-start justify-between gap-3 py-3 first:pt-0 last:pb-0"
                  data-testid={`document-row-${doc.id}`}
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-content-primary font-medium">{doc.title}</span>
                      {/* Drafts are moderator-only, so the badge is the one
                          signal telling a moderator whether players can see
                          this page at all. */}
                      {doc.status === 'published' ? (
                        <Badge variant="success" data-testid={`document-status-${doc.id}`}>
                          Published
                        </Badge>
                      ) : (
                        <Badge variant="neutral" data-testid={`document-status-${doc.id}`}>
                          Draft
                        </Badge>
                      )}
                    </div>
                    <p className="text-xs text-content-tertiary mt-1">
                      Order {doc.sort_order}
                    </p>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => togglePublished(doc)}
                      // Scoped to this row: the mutation state is shared, so an
                      // unscoped isPending would spin every button at once.
                      loading={
                        updateDocument.isPending && updateDocument.variables?.id === doc.id
                      }
                      data-testid={`toggle-publish-${doc.id}`}
                    >
                      {doc.status === 'published' ? 'Unpublish' : 'Publish'}
                    </Button>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => startEdit(doc)}
                      data-testid={`edit-document-${doc.id}`}
                    >
                      Edit
                    </Button>
                    <Button
                      variant="danger"
                      size="sm"
                      onClick={() => setPendingDelete(doc)}
                      loading={
                        deleteDocument.isPending && deleteDocument.variables === doc.id
                      }
                      data-testid={`delete-document-${doc.id}`}
                    >
                      Delete
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardBody>
      </Card>

      {/* dismissOnBackdrop=false: this holds a markdown draft that lives only in
          component state, and a stray backdrop click would discard it. The X
          and Cancel remain, so closing stays deliberate. */}
      <Modal
        isOpen={editing !== null}
        onClose={cancel}
        title={editing === 'new' ? 'New document' : 'Edit document'}
        dismissOnBackdrop={false}
      >
        <form onSubmit={handleSubmit} className="space-y-3" data-testid="document-form">
          <Input
            label="Title"
            value={draft.title}
            onChange={(e) => setDraft({ ...draft, title: e.target.value })}
            placeholder="House rules"
            data-testid="document-title"
          />

          {/* The site-wide markdown editor, same as handouts and comments:
              it carries the preview toggle, the formatting hotkeys, and the
              markdown help reference. A bare Textarea here would make this
              the one place on the site where markdown is written blind. */}
          <div>
            <label className="block text-sm font-medium text-content-primary mb-1">
              Content
            </label>
            <CommentEditor
              value={draft.content}
              onChange={(content) => setDraft({ ...draft, content })}
              placeholder="Write the document here... (Markdown supported)"
              rows={14}
              textareaTestId="document-content"
            />
            <p className="mt-1 text-sm text-content-tertiary">
              You can save an empty document and write it later.
            </p>
          </div>

          <Input
            label="Display order"
            type="number"
            value={draft.sortOrder}
            onChange={(e) => setDraft({ ...draft, sortOrder: e.target.value })}
            helperText="Lowest first. Documents sharing a number fall back to creation order."
            data-testid="document-sort-order"
          />

          {/* Create path only. On the edit form the row's Publish/Unpublish
              toggle already owns this, and two controls for one piece of state
              is how they come to disagree. */}
          {editing === 'new' && (
            <Checkbox
              label="Publish immediately"
              helperText="Otherwise it is saved as a draft that only moderators can see."
              checked={draft.publishNow}
              onChange={(e) => setDraft({ ...draft, publishNow: e.target.checked })}
              data-testid="document-publish-now"
            />
          )}

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="secondary" onClick={cancel}>
              Cancel
            </Button>
            <Button
              type="submit"
              variant="primary"
              disabled={!draft.title.trim()}
              loading={isSaving}
              data-testid="document-submit"
            >
              {editing === 'new'
                ? draft.publishNow
                  ? 'Create and publish'
                  : 'Create draft'
                : 'Save changes'}
            </Button>
          </div>
        </form>
      </Modal>

      {/* Deleting a document is irreversible and there is no undo path -- the
          service refuses a missing document rather than silently succeeding,
          which only helps if the moderator meant to delete this one. */}
      <ConfirmModal
        isOpen={pendingDelete !== null}
        onClose={() => setPendingDelete(null)}
        onConfirm={confirmDelete}
        title="Delete document"
        message={
          pendingDelete
            ? `Delete "${pendingDelete.title}"? This cannot be undone.`
            : ''
        }
        confirmText="Delete"
        variant="danger"
        isLoading={deleteDocument.isPending}
      />
    </div>
  );
}
