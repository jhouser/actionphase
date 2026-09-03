import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import { CommunitiesTab } from '../CommunitiesTab'

const listCommunities = vi.fn()
const createCommunity = vi.fn()
const updateCommunity = vi.fn()
const searchUsers = vi.fn()

vi.mock('../../../lib/api', () => ({
  apiClient: {
    communities: {
      listCommunities: () => listCommunities(),
      createCommunity: (payload: unknown) => createCommunity(payload),
      updateCommunity: (id: number, data: unknown) => updateCommunity(id, data),
    },
    auth: {
      searchUsers: (query: string) => searchUsers(query),
    },
  },
}))

const showSuccess = vi.fn()
const showError = vi.fn()
vi.mock('../../../contexts/ToastContext', async () => {
  const actual = await vi.importActual('../../../contexts/ToastContext')
  return {
    ...actual,
    useToast: () => ({ showSuccess, showError }),
  }
})

const community = {
  id: 1,
  name: 'Midnight Ravens',
  slug: 'midnight-ravens',
  description: 'Rules live here',
  banner_url: null,
  owner_user_id: 7,
  owner_username: 'corvid',
  is_active: true,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

beforeEach(() => {
  vi.clearAllMocks()
  listCommunities.mockResolvedValue({ data: [community] })
  createCommunity.mockResolvedValue({ data: community })
  updateCommunity.mockResolvedValue({ data: { ...community, is_active: false } })
  searchUsers.mockResolvedValue({ data: { users: [] } })
})

/** The picker debounces by 300ms before hitting the search endpoint. */
const OWNER = { id: 7, username: 'corvid', email: 'corvid@example.com', created_at: '2026-01-01T00:00:00Z' }

async function pickOwner(user = OWNER) {
  searchUsers.mockResolvedValue({ data: { users: [user] } })

  fireEvent.change(screen.getByTestId('community-owner-search'), {
    target: { value: user.username },
  })

  const option = await screen.findByRole('button', { name: new RegExp(user.username) }, { timeout: 2000 })
  fireEvent.mouseDown(option)
}

describe('CommunitiesTab', () => {
  it('lists existing communities with owner and status', async () => {
    renderWithProviders(<CommunitiesTab />)

    expect(await screen.findByText('Midnight Ravens')).toBeInTheDocument()
    expect(screen.getByText('/midnight-ravens')).toBeInTheDocument()
    expect(screen.getByText(/corvid/)).toBeInTheDocument()
    expect(screen.getByText('Active')).toBeInTheDocument()
  })

  it('shows an empty state when no communities exist', async () => {
    listCommunities.mockResolvedValue({ data: [] })
    renderWithProviders(<CommunitiesTab />)

    expect(await screen.findByText('No communities yet')).toBeInTheDocument()
  })

  // The slug follows the name until the admin edits it, so the common case
  // needs no thought and the deliberate case is still possible.
  it('derives the slug from the name until the slug is edited', async () => {
    renderWithProviders(<CommunitiesTab />)

    const nameInput = await screen.findByTestId('community-name-input')
    fireEvent.change(nameInput, { target: { value: 'The Midnight Ravens!' } })

    const slugInput = screen.getByTestId('community-slug-input') as HTMLInputElement
    expect(slugInput.value).toBe('the-midnight-ravens')

    fireEvent.change(slugInput, { target: { value: 'custom-slug' } })
    fireEvent.change(nameInput, { target: { value: 'A Different Name' } })

    expect((screen.getByTestId('community-slug-input') as HTMLInputElement).value).toBe('custom-slug')
  })

  // An owner is required, so the form must not submit without one -- the
  // backend would reject it anyway, and a 400 is a worse way to learn.
  it('keeps submit disabled until a name and an owner are chosen', async () => {
    renderWithProviders(<CommunitiesTab />)

    const submit = await screen.findByTestId('create-community-button')
    expect(submit).toBeDisabled()

    fireEvent.change(screen.getByTestId('community-name-input'), {
      target: { value: 'Midnight Ravens' },
    })
    expect(submit).toBeDisabled()
  })

  it('creates a community once an owner is selected', async () => {
    renderWithProviders(<CommunitiesTab />)

    fireEvent.change(await screen.findByTestId('community-name-input'), {
      target: { value: 'Midnight Ravens' },
    })
    await pickOwner()

    const submit = screen.getByTestId('create-community-button')
    await waitFor(() => expect(submit).not.toBeDisabled())
    fireEvent.click(submit)

    await waitFor(() =>
      expect(createCommunity).toHaveBeenCalledWith({
        name: 'Midnight Ravens',
        slug: 'midnight-ravens',
        description: undefined,
        owner_user_id: 7,
      })
    )
  })

  // The server explains slug collisions precisely; a generic failure string
  // would hide the one thing the admin needs to know.
  it('surfaces the server error message on failure', async () => {
    createCommunity.mockRejectedValue({
      response: { data: { error: 'that slug is already taken' } },
    })

    renderWithProviders(<CommunitiesTab />)

    fireEvent.change(await screen.findByTestId('community-name-input'), {
      target: { value: 'Midnight Ravens' },
    })
    await pickOwner()

    const submit = screen.getByTestId('create-community-button')
    await waitFor(() => expect(submit).not.toBeDisabled())
    fireEvent.click(submit)

    await waitFor(() => expect(showError).toHaveBeenCalledWith('that slug is already taken'))
  })

  // Regression: the picker holds its own text state, so clearing the form after
  // a successful create must also clear the username showing in the box --
  // otherwise the field reads like a user is still selected when none is.
  it('clears the owner field after a successful create', async () => {
    renderWithProviders(<CommunitiesTab />)

    fireEvent.change(await screen.findByTestId('community-name-input'), {
      target: { value: 'Midnight Ravens' },
    })
    await pickOwner()

    const ownerInput = screen.getByTestId('community-owner-search') as HTMLInputElement
    expect(ownerInput.value).toBe('corvid')

    const submit = screen.getByTestId('create-community-button')
    await waitFor(() => expect(submit).not.toBeDisabled())
    fireEvent.click(submit)

    await waitFor(() => expect(createCommunity).toHaveBeenCalled())
    await waitFor(() => {
      expect((screen.getByTestId('community-name-input') as HTMLInputElement).value).toBe('')
      expect((screen.getByTestId('community-owner-search') as HTMLInputElement).value).toBe('')
    })
  })

  // The search endpoint returns email addresses, but no picker surface displays
  // another user's address. Username plus join date disambiguates similar names.
  it('never shows a user email in the owner picker', async () => {
    renderWithProviders(<CommunitiesTab />)

    fireEvent.change(await screen.findByTestId('community-owner-search'), {
      target: { value: 'corvid' },
    })
    searchUsers.mockResolvedValue({ data: { users: [OWNER] } })

    const option = await screen.findByRole('button', { name: /corvid/ }, { timeout: 2000 })

    expect(option).toHaveTextContent('corvid')
    expect(option.textContent).not.toContain('@')
    expect(screen.queryByText(/corvid@example\.com/)).not.toBeInTheDocument()
  })

  it('toggles a community between active and inactive', async () => {
    renderWithProviders(<CommunitiesTab />)

    fireEvent.click(await screen.findByRole('button', { name: 'Deactivate' }))

    await waitFor(() =>
      expect(updateCommunity).toHaveBeenCalledWith(1, { is_active: false })
    )
  })

  it('labels an inactive community and offers reactivation', async () => {
    listCommunities.mockResolvedValue({ data: [{ ...community, is_active: false }] })
    renderWithProviders(<CommunitiesTab />)

    expect(await screen.findByText('Inactive')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reactivate' })).toBeInTheDocument()
  })
})
