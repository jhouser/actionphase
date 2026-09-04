import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import { ModeratorsTab } from '../ModeratorsTab'

const listModerators = vi.fn()
const addModerator = vi.fn()
const removeModerator = vi.fn()
const searchUsers = vi.fn()

vi.mock('../../../lib/api', () => ({
  apiClient: {
    communities: {
      listModerators: (slug: string) => listModerators(slug),
      addModerator: (slug: string, data: unknown) => addModerator(slug, data),
      removeModerator: (slug: string, userId: number) => removeModerator(slug, userId),
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
  description: null,
  banner_url: null,
  owner_user_id: 7,
  owner_username: 'corvid',
  is_active: true,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

const moderator = {
  id: 11,
  community_id: 1,
  user_id: 22,
  username: 'rook',
  granted_by_user_id: 7,
  granted_by_username: 'corvid',
  granted_at: '2026-08-02T00:00:00Z',
}

beforeEach(() => {
  vi.clearAllMocks()
  listModerators.mockResolvedValue({ data: [moderator] })
  addModerator.mockResolvedValue({ data: { ...moderator, id: 12, user_id: 33, username: 'jackdaw' } })
  removeModerator.mockResolvedValue({ data: undefined })
  searchUsers.mockResolvedValue({ data: { users: [] } })
})

/** The user picker debounces before it hits the search endpoint. */
async function pickUser(user: { id: number; username: string }) {
  searchUsers.mockResolvedValue({
    data: { users: [{ ...user, email: `${user.username}@example.com`, created_at: '2026-01-01T00:00:00Z' }] },
  })

  fireEvent.change(screen.getByTestId('moderator-user-search'), {
    target: { value: user.username },
  })

  const option = await screen.findByRole(
    'button',
    { name: new RegExp(user.username) },
    { timeout: 2000 }
  )
  fireEvent.mouseDown(option)
}

describe('ModeratorsTab', () => {
  // The server's roster deliberately omits the owner, so a page that rendered
  // only the response would misreport who holds power here.
  it('shows the owner alongside the moderators even though the roster omits them', async () => {
    renderWithProviders(<ModeratorsTab community={community} canAdminister />)

    expect(await screen.findByText('rook')).toBeInTheDocument()
    expect(screen.getByText('corvid')).toBeInTheDocument()
    expect(screen.getByText('Owner')).toBeInTheDocument()
    expect(screen.getByText('Moderator')).toBeInTheDocument()
  })

  it('tells the owner when nobody else moderates yet', async () => {
    listModerators.mockResolvedValue({ data: [] })
    renderWithProviders(<ModeratorsTab community={community} canAdminister />)

    expect(await screen.findByText(/No moderators yet/)).toBeInTheDocument()
  })

  // THE UI HALF OF REQUIREMENT 4. The server refuses a moderator's write either
  // way, but offering controls that always fail is its own defect.
  it('hides every roster control from a moderator', async () => {
    renderWithProviders(<ModeratorsTab community={community} canAdminister={false} />)

    // Wait for the roster so the assertions below cannot pass merely because
    // nothing has rendered yet.
    expect(await screen.findByText('rook')).toBeInTheDocument()

    expect(screen.queryByTestId('moderator-user-search')).not.toBeInTheDocument()
    expect(screen.queryByTestId('add-moderator-submit')).not.toBeInTheDocument()
    expect(screen.queryByTestId(`remove-moderator-${moderator.user_id}`)).not.toBeInTheDocument()
  })

  it('shows roster controls to the owner', async () => {
    renderWithProviders(<ModeratorsTab community={community} canAdminister />)

    // Await the roster, not the picker: the picker renders immediately, so
    // awaiting it would assert against a list still showing its spinner.
    expect(await screen.findByText('rook')).toBeInTheDocument()

    expect(screen.getByTestId('moderator-user-search')).toBeInTheDocument()
    expect(screen.getByTestId(`remove-moderator-${moderator.user_id}`)).toBeInTheDocument()
  })

  it('adds a moderator once a user is picked', async () => {
    renderWithProviders(<ModeratorsTab community={community} canAdminister />)
    await screen.findByTestId('moderator-user-search')

    await pickUser({ id: 33, username: 'jackdaw' })
    fireEvent.click(screen.getByTestId('add-moderator-submit'))

    await waitFor(() => {
      expect(addModerator).toHaveBeenCalledWith('midnight-ravens', { user_id: 33 })
    })
    expect(showSuccess).toHaveBeenCalled()
  })

  it('removes a moderator', async () => {
    renderWithProviders(<ModeratorsTab community={community} canAdminister />)

    fireEvent.click(await screen.findByTestId(`remove-moderator-${moderator.user_id}`))

    await waitFor(() => {
      expect(removeModerator).toHaveBeenCalledWith('midnight-ravens', moderator.user_id)
    })
  })

  // The remove mutation's state is shared by every row, so a spinner bound to a
  // bare isPending would report that all of the moderators are being removed
  // when only one is. Needs two rows and an in-flight request to catch.
  it('spins only the row being removed, not every row', async () => {
    const second = { ...moderator, id: 12, user_id: 33, username: 'jackdaw' }
    listModerators.mockResolvedValue({ data: [moderator, second] })

    // Hold the request open so the pending state is observable.
    let release!: () => void
    removeModerator.mockReturnValue(
      new Promise<{ data: undefined }>((resolve) => {
        release = () => resolve({ data: undefined })
      })
    )

    renderWithProviders(<ModeratorsTab community={community} canAdminister />)

    fireEvent.click(await screen.findByTestId(`remove-moderator-${moderator.user_id}`))

    const spinning = await screen.findByTestId(`remove-moderator-${moderator.user_id}`)
    await waitFor(() => expect(spinning).toBeDisabled())

    // The untouched row must stay actionable.
    expect(screen.getByTestId(`remove-moderator-${second.user_id}`)).not.toBeDisabled()

    release()
  })

  // The server explains precisely why a grant was refused (the owner, a
  // duplicate, an unknown user); a generic failure string would throw that away.
  it('surfaces the server’s explanation when a grant is refused', async () => {
    // `error`, not `detail` -- see the LegacyError shape note in
    // SettingsTab.test.tsx.
    addModerator.mockRejectedValue({
      response: { data: { error: 'that user already moderates this community' } },
    })

    renderWithProviders(<ModeratorsTab community={community} canAdminister />)
    await screen.findByTestId('moderator-user-search')

    await pickUser({ id: 33, username: 'jackdaw' })
    fireEvent.click(screen.getByTestId('add-moderator-submit'))

    await waitFor(() => {
      expect(showError).toHaveBeenCalledWith('that user already moderates this community')
    })
  })
})
