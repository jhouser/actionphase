import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import { SettingsTab } from '../SettingsTab'
import type { Community } from '../../../types/communities'

const updateCommunityProfile = vi.fn()

vi.mock('../../../lib/api', () => ({
  apiClient: {
    communities: {
      updateCommunityProfile: (slug: string, data: unknown) =>
        updateCommunityProfile(slug, data),
    },
  },
}))

const showSuccess = vi.fn()
const showError = vi.fn()
vi.mock('../../../contexts/ToastContext', async () => {
  const actual = await vi.importActual('../../../contexts/ToastContext')
  return { ...actual, useToast: () => ({ showSuccess, showError }) }
})

// CommentEditor is the project's markdown editor; it is exercised by its own
// tests. Standing it in keeps these assertions about the FORM -- which fields
// are sent, and when -- rather than about the editor's preview machinery.
vi.mock('../../../components/CommentEditor', () => ({
  CommentEditor: ({
    value,
    onChange,
    textareaTestId,
  }: {
    value: string
    onChange: (v: string) => void
    textareaTestId?: string
  }) => (
    <textarea
      data-testid={textareaTestId}
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}))

const community: Community = {
  id: 1,
  name: 'Midnight Ravens',
  slug: 'midnight-ravens',
  description: 'We fly at dusk',
  banner_url: null,
  owner_user_id: 7,
  owner_username: 'corvid',
  is_active: true,
  your_role: 'moderator',
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

beforeEach(() => {
  vi.clearAllMocks()
  updateCommunityProfile.mockResolvedValue({ data: community })
})

describe('SettingsTab', () => {
  it('shows the current name and description', () => {
    renderWithProviders(<SettingsTab community={community} canEdit />)

    expect(screen.getByTestId('community-settings-name')).toHaveValue('Midnight Ravens')
    expect(screen.getByTestId('community-settings-description')).toHaveValue('We fly at dusk')
  })

  it('uses a markdown editor for the description, not a plain input', () => {
    renderWithProviders(<SettingsTab community={community} canEdit />)

    // A textarea-based editor, not an <input>: the description is markdown and
    // needs room plus a preview.
    expect(screen.getByTestId('community-settings-description').tagName).toBe('TEXTAREA')
  })

  it('sends only the fields that changed', async () => {
    renderWithProviders(<SettingsTab community={community} canEdit />)

    fireEvent.change(screen.getByTestId('community-settings-name'), {
      target: { value: 'Midnight Ravens Reborn' },
    })
    fireEvent.click(screen.getByTestId('community-settings-save'))

    await waitFor(() => expect(updateCommunityProfile).toHaveBeenCalled())
    // The untouched description is omitted, which the endpoint reads as
    // "leave it alone".
    expect(updateCommunityProfile).toHaveBeenCalledWith('midnight-ravens', {
      name: 'Midnight Ravens Reborn',
    })
  })

  // Omission means "unchanged", so clearing has to send an explicit empty
  // string or a description could never be removed once written.
  it('sends an empty string to clear the description', async () => {
    renderWithProviders(<SettingsTab community={community} canEdit />)

    fireEvent.change(screen.getByTestId('community-settings-description'), {
      target: { value: '' },
    })
    fireEvent.click(screen.getByTestId('community-settings-save'))

    await waitFor(() => expect(updateCommunityProfile).toHaveBeenCalled())
    expect(updateCommunityProfile).toHaveBeenCalledWith('midnight-ravens', {
      description: '',
    })
  })

  it('disables saving until something changes', () => {
    renderWithProviders(<SettingsTab community={community} canEdit />)

    expect(screen.getByTestId('community-settings-save')).toBeDisabled()

    fireEvent.change(screen.getByTestId('community-settings-name'), {
      target: { value: 'Renamed' },
    })
    expect(screen.getByTestId('community-settings-save')).toBeEnabled()
  })

  // The server rejects a blank name with a 400; refusing it here keeps the
  // button honest rather than letting the moderator discover it on submit.
  it('refuses to save a blank name', () => {
    renderWithProviders(<SettingsTab community={community} canEdit />)

    fireEvent.change(screen.getByTestId('community-settings-name'), {
      target: { value: '   ' },
    })

    expect(screen.getByTestId('community-settings-save')).toBeDisabled()
  })

  // The payload shape here is the one the server actually sends: LegacyError
  // serializes its message as `error`. A `detail` key never appears on the
  // wire, so a test using one would pass against a component that reads a
  // field the API does not emit.
  it('reports a failed save', async () => {
    updateCommunityProfile.mockRejectedValue({
      response: { data: { error: 'name cannot be blank' } },
    })
    renderWithProviders(<SettingsTab community={community} canEdit />)

    fireEvent.change(screen.getByTestId('community-settings-name'), {
      target: { value: 'Renamed' },
    })
    fireEvent.click(screen.getByTestId('community-settings-save'))

    await waitFor(() => expect(showError).toHaveBeenCalledWith('name cannot be blank'))
  })

  it('renders read-only for a viewer who cannot moderate', () => {
    renderWithProviders(<SettingsTab community={community} canEdit={false} />)

    expect(screen.queryByTestId('community-settings-form')).not.toBeInTheDocument()
    expect(screen.getByText(/only this community's moderators/i)).toBeInTheDocument()
  })
})
