import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import { DocumentsTab } from '../DocumentsTab'
import type { Community, CommunityDocument } from '../../../types/communities'

const listAllDocuments = vi.fn()
const createDocument = vi.fn()
const updateDocument = vi.fn()
const deleteDocument = vi.fn()

vi.mock('../../../lib/api', () => ({
  apiClient: {
    communities: {
      listAllDocuments: (slug: string) => listAllDocuments(slug),
      createDocument: (slug: string, data: unknown) => createDocument(slug, data),
      updateDocument: (slug: string, id: number, data: unknown) =>
        updateDocument(slug, id, data),
      deleteDocument: (slug: string, id: number) => deleteDocument(slug, id),
    },
  },
}))

const showSuccess = vi.fn()
const showError = vi.fn()
vi.mock('../../../contexts/ToastContext', async () => {
  const actual = await vi.importActual('../../../contexts/ToastContext')
  return { ...actual, useToast: () => ({ showSuccess, showError }) }
})

const community: Community = {
  id: 1,
  name: 'Midnight Ravens',
  slug: 'midnight-ravens',
  description: null,
  banner_url: null,
  owner_user_id: 7,
  owner_username: 'corvid',
  is_active: true,
  your_role: 'moderator',
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

const publishedDoc: CommunityDocument = {
  id: 20,
  community_id: 1,
  title: 'House rules',
  content: '# Be excellent',
  status: 'published',
  sort_order: 10,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

const draftDoc: CommunityDocument = {
  id: 21,
  community_id: 1,
  title: 'Work in progress',
  content: 'half written',
  status: 'draft',
  sort_order: 20,
  created_at: '2026-08-02T00:00:00Z',
  updated_at: '2026-08-02T00:00:00Z',
}

beforeEach(() => {
  vi.clearAllMocks()
  listAllDocuments.mockResolvedValue({ data: [publishedDoc, draftDoc] })
  createDocument.mockResolvedValue({ data: draftDoc })
  updateDocument.mockResolvedValue({ data: publishedDoc })
  deleteDocument.mockResolvedValue({ data: undefined })
})

describe('DocumentsTab', () => {
  it('lists documents with their status', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)

    await screen.findByTestId('document-list')
    expect(screen.getByTestId('document-status-20')).toHaveTextContent('Published')
    expect(screen.getByTestId('document-status-21')).toHaveTextContent('Draft')
  })

  // This is the MANAGE list, so drafts belong here. The distinction matters:
  // the public community page uses a different hook and a different cache key
  // precisely so a draft cannot reach it.
  it('reads the moderator list, not the public one', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)

    await screen.findByTestId('document-list')
    expect(listAllDocuments).toHaveBeenCalledWith('midnight-ravens')
  })

  it('renders nothing manageable for a non-moderator', () => {
    renderWithProviders(<DocumentsTab community={community} canModerate={false} />)

    expect(screen.getByText('Moderators only')).toBeInTheDocument()
    expect(screen.queryByTestId('new-document')).not.toBeInTheDocument()
    expect(listAllDocuments).not.toHaveBeenCalled()
  })

  it('shows an empty state when the community has no documents', async () => {
    listAllDocuments.mockResolvedValue({ data: [] })
    renderWithProviders(<DocumentsTab community={community} canModerate />)

    expect(await screen.findByTestId('documents-empty')).toBeInTheDocument()
  })

  // Creating always yields a draft: a half-written page must bind nobody until
  // its author publishes it deliberately.
  // Unpublished stays the DEFAULT -- the publish box is opt-in, so a moderator
  // who just fills in a title and submits still gets a draft.
  it('creates a draft when the publish box is left alone', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('new-document'))
    fireEvent.change(screen.getByTestId('document-title'), {
      target: { value: 'New rules' },
    })
    expect(screen.getByTestId('document-publish-now')).not.toBeChecked()
    fireEvent.click(screen.getByTestId('document-submit'))

    await waitFor(() => expect(createDocument).toHaveBeenCalled())
    const [, payload] = createDocument.mock.calls[0]
    expect(payload).toMatchObject({ title: 'New rules', status: 'draft' })
  })

  // The API has always accepted status on create; the form used to omit it, so
  // a finished page still needed a second Publish click.
  it('creates a published document when the publish box is ticked', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('new-document'))
    fireEvent.change(screen.getByTestId('document-title'), {
      target: { value: 'New rules' },
    })
    fireEvent.click(screen.getByTestId('document-publish-now'))
    expect(screen.getByTestId('document-submit')).toHaveTextContent('Create and publish')

    fireEvent.click(screen.getByTestId('document-submit'))

    await waitFor(() => expect(createDocument).toHaveBeenCalled())
    const [, payload] = createDocument.mock.calls[0]
    expect(payload).toMatchObject({ title: 'New rules', status: 'published' })
  })

  // Status on the edit path belongs to the row's Publish/Unpublish toggle. If
  // the editor also sent it, opening a published document and saving a typo fix
  // would carry whatever the (hidden) box happened to hold.
  it('omits status when editing so the row toggle stays the only owner', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('edit-document-20'))
    expect(screen.queryByTestId('document-publish-now')).not.toBeInTheDocument()

    fireEvent.click(screen.getByTestId('document-submit'))

    await waitFor(() => expect(updateDocument).toHaveBeenCalled())
    const [, , payload] = updateDocument.mock.calls[0]
    expect(payload).not.toHaveProperty('status')
  })

  it('trims the title before sending it', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('new-document'))
    fireEvent.change(screen.getByTestId('document-title'), {
      target: { value: '  Padded  ' },
    })
    fireEvent.click(screen.getByTestId('document-submit'))

    await waitFor(() => expect(createDocument).toHaveBeenCalled())
    expect(createDocument.mock.calls[0][1].title).toBe('Padded')
  })

  it('will not submit a blank title', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('new-document'))
    fireEvent.change(screen.getByTestId('document-title'), {
      target: { value: '   ' },
    })

    expect(screen.getByTestId('document-submit')).toBeDisabled()
    expect(createDocument).not.toHaveBeenCalled()
  })

  // Publishing is an edit of status alone -- the title and body must not be
  // resent, or a stale form would overwrite whatever the row currently holds.
  it('publishes by sending only the status', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('toggle-publish-21'))

    await waitFor(() => expect(updateDocument).toHaveBeenCalled())
    expect(updateDocument).toHaveBeenCalledWith('midnight-ravens', 21, {
      status: 'published',
    })
  })

  it('unpublishes a published document', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('toggle-publish-20'))

    await waitFor(() => expect(updateDocument).toHaveBeenCalled())
    expect(updateDocument).toHaveBeenCalledWith('midnight-ravens', 20, {
      status: 'draft',
    })
  })

  // The list is what a moderator came to this tab to manage, so it must be
  // visible without opening anything. The first version put an always-open
  // twelve-row editor above it and pushed the list below the fold.
  it('shows the document list without opening the editor', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)

    await screen.findByTestId('document-list')
    expect(screen.queryByTestId('document-form')).not.toBeInTheDocument()
  })

  it('opens the editor in a dialog and closes it again', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('new-document'))
    expect(screen.getByTestId('document-form')).toBeInTheDocument()
    // The list stays mounted behind the dialog rather than being replaced.
    expect(screen.getByTestId('document-list')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByTestId('document-form')).not.toBeInTheDocument()
  })

  // The content field must be the SITE'S markdown editor, not a bare textarea.
  // Without this, swapping CommentEditor out for a plain <textarea> passes
  // every other test in this file -- which is how the wrong control shipped
  // here in the first place. The Write/Preview toggle is what distinguishes it.
  it('writes content through the shared markdown editor', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('new-document'))

    expect(screen.getByTestId('document-content')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /preview/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /write/i })).toBeInTheDocument()
  })

  // The editor carries its own preview, so a second preview pane below the form
  // would render the same markdown twice on one screen.
  it('renders the markdown preview through the editor rather than a second pane', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('new-document'))
    fireEvent.change(screen.getByTestId('document-content'), {
      target: { value: '# Heading' },
    })

    expect(screen.queryByRole('heading', { name: 'Preview' })).not.toBeInTheDocument()
  })

  it('loads an existing document into the editor', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('edit-document-20'))

    expect(screen.getByTestId('document-title')).toHaveValue('House rules')
    expect(screen.getByTestId('document-content')).toHaveValue('# Be excellent')
  })

  // Deleting is irreversible with no undo path, so it goes behind ConfirmModal
  // like every other destructive action on the site. The button alone must NOT
  // reach the server.
  it('does not delete until the moderator confirms', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('delete-document-20'))
    expect(deleteDocument).not.toHaveBeenCalled()

    // The prompt names the document -- "Delete this document?" is not a
    // question a moderator can answer.
    expect(await screen.findByTestId('confirm-modal-message')).toHaveTextContent(
      'Delete "House rules"? This cannot be undone.'
    )

    // By testid, not role+name: the dialog's confirm button and the row button
    // that opened it are BOTH labelled "Delete", so a name lookup matches two
    // elements and cannot say which one the test meant.
    fireEvent.click(screen.getByTestId('confirm-modal-confirm'))

    await waitFor(() => expect(deleteDocument).toHaveBeenCalledWith('midnight-ravens', 20))
  })

  it('deletes nothing when the confirmation is dismissed', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('delete-document-20'))
    await screen.findByTestId('confirm-modal')
    fireEvent.click(screen.getByTestId('confirm-modal-cancel'))

    await waitFor(() =>
      expect(screen.queryByTestId('confirm-modal')).not.toBeInTheDocument()
    )
    expect(deleteDocument).not.toHaveBeenCalled()
  })

  // The server's message names the actual problem; the generic fallback does
  // not. Surfacing it is what tells a moderator why a save was refused.
  it('surfaces the server error message', async () => {
    createDocument.mockRejectedValue({
      response: { data: { error: 'status must be draft or published' } },
    })
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('new-document'))
    fireEvent.change(screen.getByTestId('document-title'), {
      target: { value: 'Rules' },
    })
    fireEvent.click(screen.getByTestId('document-submit'))

    await waitFor(() =>
      expect(showError).toHaveBeenCalledWith('status must be draft or published')
    )
  })

  // An unparseable number must not become NaN, which serialises to null and
  // reads server-side as "leave this field unchanged".
  it('falls back to order 0 rather than sending NaN', async () => {
    renderWithProviders(<DocumentsTab community={community} canModerate />)
    await screen.findByTestId('document-list')

    fireEvent.click(screen.getByTestId('new-document'))
    fireEvent.change(screen.getByTestId('document-title'), {
      target: { value: 'Rules' },
    })
    fireEvent.change(screen.getByTestId('document-sort-order'), {
      target: { value: 'abc' },
    })
    fireEvent.click(screen.getByTestId('document-submit'))

    await waitFor(() => expect(createDocument).toHaveBeenCalled())
    expect(createDocument.mock.calls[0][1].sort_order).toBe(0)
  })
})
